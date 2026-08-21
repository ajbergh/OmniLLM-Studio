package rendercontract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type projectionNode struct {
	Type                 string                    `json:"type"`
	Ref                  string                    `json:"$ref"`
	Properties           map[string]projectionNode `json:"properties"`
	Required             []string                  `json:"required"`
	Items                *projectionNode           `json:"items"`
	Enum                 []any                     `json:"enum"`
	Const                any                       `json:"const"`
	OneOf                []projectionNode          `json:"oneOf"`
	AdditionalProperties any                       `json:"additionalProperties"`
}
type projectionSchema struct {
	projectionNode
	Defs map[string]projectionNode `json:"$defs"`
}

func loadProjectionSchema(t *testing.T, name string) projectionSchema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRootFromTest(t), "video-renderer", "contracts", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var schema projectionSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return schema
}

func TestTimelineV2GoProjectionMatchesSchema(t *testing.T) {
	schema := loadProjectionSchema(t, "timeline-v2.schema.json")
	assertStructProjection(t, "timeline", reflect.TypeOf(TimelineV2Document{}), schema.projectionNode, schema.Defs, true)
	projections := map[string]reflect.Type{
		"canvas": reflect.TypeOf(TimelineV2Canvas{}), "track": reflect.TypeOf(TimelineV2Track{}), "clip": reflect.TypeOf(TimelineV2Clip{}),
		"transform": reflect.TypeOf(TimelineV2Transform{}), "crop": reflect.TypeOf(TimelineV2Crop{}), "contentBounds": reflect.TypeOf(TimelineV2ContentBounds{}),
		"text": reflect.TypeOf(TimelineV2Text{}), "shape": reflect.TypeOf(TimelineV2Shape{}), "cursor": reflect.TypeOf(TimelineV2Cursor{}),
		"cursorEvent": reflect.TypeOf(TimelineV2CursorEvent{}), "effect": reflect.TypeOf(TimelineV2Effect{}), "transition": reflect.TypeOf(TimelineV2Transition{}),
		"motionCurve": reflect.TypeOf(MotionCurve{}), "keyframe": reflect.TypeOf(TimelineV2Keyframe{}), "animationBlock": reflect.TypeOf(TimelineV2AnimationBlock{}),
		"camera": reflect.TypeOf(TimelineV2Camera{}), "scene": reflect.TypeOf(TimelineV2Scene{}), "marker": reflect.TypeOf(TimelineV2Marker{}),
	}
	for name, goType := range projections {
		node, ok := schema.Defs[name]
		if !ok {
			t.Fatalf("timeline schema missing $defs.%s", name)
		}
		if name == "motionCurve" {
			node = unionObjectNode(node)
		}
		assertStructProjection(t, "$defs."+name, goType, node, schema.Defs, name != "motionCurve")
	}
}

func TestRenderManifestV1GoProjectionMatchesSchema(t *testing.T) {
	schema := loadProjectionSchema(t, "render-manifest-v1.schema.json")
	assertStructProjection(t, "manifest", reflect.TypeOf(RenderManifestV1{}), schema.projectionNode, schema.Defs, true)
	projections := map[string]reflect.Type{"asset": reflect.TypeOf(RenderManifestAsset{}), "fontResource": reflect.TypeOf(RenderManifestFontResource{}), "mediaProbe": reflect.TypeOf(RenderManifestMediaProbe{}), "settings": reflect.TypeOf(RenderManifestSettings{})}
	for name, goType := range projections {
		node, ok := schema.Defs[name]
		if !ok {
			t.Fatalf("render manifest schema missing $defs.%s", name)
		}
		assertStructProjection(t, "$defs."+name, goType, node, schema.Defs, true)
	}
}

func assertStructProjection(t *testing.T, path string, goType reflect.Type, node projectionNode, defs map[string]projectionNode, checkRequired bool) {
	t.Helper()
	goType = dereference(goType)
	if goType.Kind() != reflect.Struct {
		t.Fatalf("%s projection is %s, want struct", path, goType)
	}
	required := map[string]bool{}
	for _, name := range node.Required {
		required[name] = true
	}
	goFields := map[string]reflect.StructField{}
	for i := 0; i < goType.NumField(); i++ {
		field := goType.Field(i)
		jsonName, options := parseJSONTag(field.Tag.Get("json"))
		if jsonName == "" || jsonName == "-" {
			continue
		}
		goFields[jsonName] = field
		if checkRequired {
			hasOmitEmpty := options["omitempty"]
			if required[jsonName] && hasOmitEmpty {
				t.Errorf("%s.%s required by schema but tagged omitempty", path, jsonName)
			}
			if !required[jsonName] && !hasOmitEmpty {
				t.Errorf("%s.%s optional by schema but missing omitempty", path, jsonName)
			}
		}
	}
	goNames := sortedFieldKeys(goFields)
	schemaNames := sortedNodeKeys(node.Properties)
	if !reflect.DeepEqual(goNames, schemaNames) {
		t.Fatalf("%s field drift:\nGo: %v\nSchema: %v", path, goNames, schemaNames)
	}
	for name, schemaField := range node.Properties {
		if err := compatibleGoType(goFields[name].Type, schemaField, defs); err != nil {
			t.Errorf("%s.%s: %v", path, name, err)
		}
	}
}

func compatibleGoType(goType reflect.Type, node projectionNode, defs map[string]projectionNode) error {
	goType = dereference(goType)
	if node.Ref != "" {
		if node.Ref == "timeline-v2.schema.json" {
			if goType != reflect.TypeOf(TimelineV2Document{}) {
				return fmt.Errorf("Go type %s does not project Timeline v2", goType)
			}
			return nil
		}
		const prefix = "#/$defs/"
		if strings.HasPrefix(node.Ref, prefix) {
			name := strings.TrimPrefix(node.Ref, prefix)
			def, ok := defs[name]
			if !ok {
				return fmt.Errorf("missing schema ref %q", node.Ref)
			}
			return compatibleGoType(goType, def, defs)
		}
		return fmt.Errorf("unsupported schema ref %q", node.Ref)
	}
	if len(node.OneOf) > 0 {
		if goType.Kind() != reflect.Struct {
			return fmt.Errorf("Go type %s must be struct for oneOf", goType)
		}
		return nil
	}
	if len(node.Enum) > 0 || node.Const != nil {
		value := node.Const
		if value == nil && len(node.Enum) > 0 {
			value = node.Enum[0]
		}
		switch value.(type) {
		case string:
			if goType.Kind() != reflect.String {
				return fmt.Errorf("Go kind %s, want string", goType.Kind())
			}
		case float64:
			if goType.Kind() != reflect.Int && goType.Kind() != reflect.Int64 && goType.Kind() != reflect.Float64 {
				return fmt.Errorf("Go kind %s, want numeric", goType.Kind())
			}
		}
		return nil
	}
	switch node.Type {
	case "string":
		if goType.Kind() != reflect.String {
			return fmt.Errorf("Go kind %s, want string", goType.Kind())
		}
	case "integer":
		if goType.Kind() != reflect.Int && goType.Kind() != reflect.Int64 {
			return fmt.Errorf("Go kind %s, want integer", goType.Kind())
		}
	case "number":
		if goType.Kind() != reflect.Float64 {
			return fmt.Errorf("Go kind %s, want float64", goType.Kind())
		}
	case "boolean":
		if goType.Kind() != reflect.Bool {
			return fmt.Errorf("Go kind %s, want bool", goType.Kind())
		}
	case "array":
		if goType.Kind() != reflect.Slice {
			return fmt.Errorf("Go kind %s, want slice", goType.Kind())
		}
		if node.Items != nil {
			return compatibleGoType(goType.Elem(), *node.Items, defs)
		}
	case "object":
		if allowsAdditionalProperties(node.AdditionalProperties) {
			if goType.Kind() != reflect.Map {
				return fmt.Errorf("Go kind %s, want metadata map", goType.Kind())
			}
		} else if goType.Kind() != reflect.Struct {
			return fmt.Errorf("Go kind %s, want struct", goType.Kind())
		}
	}
	return nil
}

func unionObjectNode(node projectionNode) projectionNode {
	properties := map[string]projectionNode{}
	for _, variant := range node.OneOf {
		for name, property := range variant.Properties {
			properties[name] = property
		}
	}
	return projectionNode{Type: "object", Properties: properties}
}
func parseJSONTag(tag string) (string, map[string]bool) {
	parts := strings.Split(tag, ",")
	options := map[string]bool{}
	for _, option := range parts[1:] {
		options[option] = true
	}
	return parts[0], options
}
func sortedNodeKeys(nodes map[string]projectionNode) []string {
	keys := make([]string, 0, len(nodes))
	for key := range nodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func sortedFieldKeys(fields map[string]reflect.StructField) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func dereference(value reflect.Type) reflect.Type {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	return value
}
func allowsAdditionalProperties(value any) bool { allowed, ok := value.(bool); return ok && allowed }
