package rendercontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type jsonSchema struct {
	Schema               string                    `json:"$schema"`
	ID                   string                    `json:"$id"`
	Title                string                    `json:"title"`
	AdditionalProperties any                       `json:"additionalProperties"`
	Required             []string                  `json:"required"`
	Properties           map[string]map[string]any `json:"properties"`
	Defs                 map[string]any            `json:"$defs"`
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve schema test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
}

func loadSchema(t *testing.T, name string) jsonSchema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRootFromTest(t), "video-renderer", "contracts", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var schema jsonSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return schema
}

func TestTimelineV2SchemaIsStrictAndVersioned(t *testing.T) {
	schema := loadSchema(t, "timeline-v2.schema.json")
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected JSON Schema dialect: %q", schema.Schema)
	}
	if schema.ID == "" || schema.Title == "" {
		t.Fatal("timeline schema must have stable id and title")
	}
	if strict, ok := schema.AdditionalProperties.(bool); !ok || strict {
		t.Fatal("timeline v2 root must fail closed on unknown authorable fields")
	}
	version, ok := schema.Properties["version"]["const"].(float64)
	if !ok || version != 2 {
		t.Fatalf("timeline version const = %v, want 2", schema.Properties["version"]["const"])
	}
	for _, field := range []string{"canvas", "duration_ms", "tracks", "markers", "working_color_space", "metadata"} {
		if _, ok := schema.Properties[field]; !ok {
			t.Errorf("timeline v2 missing root field %q", field)
		}
	}
	for _, definition := range []string{"clip", "transform", "text", "transition", "keyframe", "camera"} {
		if _, ok := schema.Defs[definition]; !ok {
			t.Errorf("timeline v2 missing definition %q", definition)
		}
	}
}

func TestRenderManifestV1SchemaBindsImmutableIdentity(t *testing.T) {
	schema := loadSchema(t, "render-manifest-v1.schema.json")
	version, ok := schema.Properties["version"]["const"].(float64)
	if !ok || version != 1 {
		t.Fatalf("render manifest version const = %v, want 1", schema.Properties["version"]["const"])
	}
	for _, field := range []string{
		"contract_version", "snapshot_id", "timeline_id", "timeline_revision",
		"timeline_sha256", "asset_manifest_sha256", "timeline", "assets", "font_resources", "settings",
	} {
		if _, ok := schema.Properties[field]; !ok {
			t.Errorf("render manifest v1 missing immutable identity field %q", field)
		}
	}
	if _, ok := schema.Defs["fontResource"]; !ok {
		t.Error("render manifest v1 missing fontResource definition")
	}
}
