package artifacts

import (
	"context"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var yamlFenceRe = regexp.MustCompile("(?s)```(?:yaml|yml)?\n?(.*?)```")

// YAMLRenderer produces a .yaml artifact.
// Priority: extract from a ```yaml fence → parse entire raw as YAML →
// fall back to serialising the Artifact model.
type YAMLRenderer struct{}

func (r *YAMLRenderer) Format() ArtifactFormat { return FormatYaml }

func (r *YAMLRenderer) Render(_ context.Context, a Artifact) (*RenderedArtifact, error) {
	out, err := extractOrSerialiseYAML(a)
	if err != nil {
		return nil, err
	}
	return &RenderedArtifact{
		ContentType: ContentTypeForFormat(FormatYaml),
		Bytes:       out,
	}, nil
}

func extractOrSerialiseYAML(a Artifact) ([]byte, error) {
	raw := strings.TrimSpace(a.RawContent)

	// 1. Try to find a YAML code fence
	if m := yamlFenceRe.FindStringSubmatch(raw); len(m) >= 2 {
		candidate := strings.TrimSpace(m[1])
		if b, err := roundTripYAML(candidate); err == nil {
			return b, nil
		}
	}

	// 2. Try the whole raw content as YAML (if it looks like YAML/config)
	if looksLikeYAML(raw) {
		if b, err := roundTripYAML(raw); err == nil {
			return b, nil
		}
	}

	// 3. Serialise the artifact model
	type tableOut struct {
		Name    string     `yaml:"name,omitempty"`
		Headers []string   `yaml:"headers"`
		Rows    [][]string `yaml:"rows"`
	}
	type sectionOut struct {
		Heading string   `yaml:"heading,omitempty"`
		Content string   `yaml:"content,omitempty"`
		Items   []string `yaml:"items,omitempty"`
	}
	type artifactOut struct {
		Title    string       `yaml:"title,omitempty"`
		Sections []sectionOut `yaml:"sections,omitempty"`
		Tables   []tableOut   `yaml:"tables,omitempty"`
	}

	out := artifactOut{Title: a.Title}
	var curSection *sectionOut
	for _, b := range a.Blocks {
		switch b.Type {
		case BlockHeading:
			if curSection != nil {
				out.Sections = append(out.Sections, *curSection)
			}
			curSection = &sectionOut{Heading: b.Text}
		case BlockParagraph:
			if curSection == nil {
				curSection = &sectionOut{}
			}
			if curSection.Content != "" {
				curSection.Content += "\n"
			}
			curSection.Content += b.Text
		case BlockBullet, BlockOrdered:
			if curSection == nil {
				curSection = &sectionOut{}
			}
			curSection.Items = append(curSection.Items, b.Items...)
		case BlockTable:
			if b.Table != nil {
				out.Tables = append(out.Tables, tableOut{
					Name:    b.Table.Name,
					Headers: b.Table.Headers,
					Rows:    b.Table.Rows,
				})
			}
		}
	}
	if curSection != nil {
		out.Sections = append(out.Sections, *curSection)
	}

	return yaml.Marshal(out)
}

func roundTripYAML(s string) ([]byte, error) {
	var v interface{}
	if err := yaml.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return yaml.Marshal(v)
}

func looksLikeYAML(s string) bool {
	lines := strings.SplitN(s, "\n", 5)
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		// YAML key: value pattern
		if strings.Contains(l, ": ") || strings.HasSuffix(l, ":") {
			return true
		}
		// YAML list item
		if strings.HasPrefix(l, "- ") {
			return true
		}
	}
	return false
}
