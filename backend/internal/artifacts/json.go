package artifacts

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

var jsonFenceRe = regexp.MustCompile("(?s)```(?:json)?\n?(.*?)```")

// JSONRenderer produces a .json artifact.
// Priority: extract from a ```json fence → parse entire raw as JSON →
// fall back to serialising the Artifact model.
type JSONRenderer struct{}

func (r *JSONRenderer) Format() ArtifactFormat { return FormatJson }

func (r *JSONRenderer) Render(_ context.Context, a Artifact) (*RenderedArtifact, error) {
	out, err := extractOrSerialiseJSON(a)
	if err != nil {
		return nil, err
	}
	return &RenderedArtifact{
		ContentType: ContentTypeForFormat(FormatJson),
		Bytes:       out,
	}, nil
}

func extractOrSerialiseJSON(a Artifact) ([]byte, error) {
	raw := strings.TrimSpace(a.RawContent)

	// 1. Try to find a JSON code fence
	if m := jsonFenceRe.FindStringSubmatch(raw); len(m) >= 2 {
		candidate := strings.TrimSpace(m[1])
		if b, err := prettyJSON(candidate); err == nil {
			return b, nil
		}
	}

	// 2. Try treating the whole raw content as JSON
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		if b, err := prettyJSON(raw); err == nil {
			return b, nil
		}
	}

	// 3. Serialise the artifact model
	type tableOut struct {
		Name    string     `json:"name,omitempty"`
		Headers []string   `json:"headers"`
		Rows    [][]string `json:"rows"`
	}
	type sectionOut struct {
		Heading string   `json:"heading,omitempty"`
		Content string   `json:"content,omitempty"`
		Items   []string `json:"items,omitempty"`
	}
	type artifactOut struct {
		Title    string       `json:"title,omitempty"`
		Sections []sectionOut `json:"sections,omitempty"`
		Tables   []tableOut   `json:"tables,omitempty"`
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

	return json.MarshalIndent(out, "", "  ")
}

func prettyJSON(s string) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}
