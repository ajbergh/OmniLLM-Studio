package artifacts

import (
	"bytes"
	"context"
	"encoding/csv"
)

// CSVRenderer exports the first table in an Artifact as a UTF-8 CSV file.
// If no table is present it falls back to a two-column Section/Content sheet.
type CSVRenderer struct{}

func (r *CSVRenderer) Format() ArtifactFormat { return FormatCsv }

func (r *CSVRenderer) Render(_ context.Context, a Artifact) (*RenderedArtifact, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if len(a.Tables) > 0 {
		t := a.Tables[0]
		if err := w.Write(t.Headers); err != nil {
			return nil, err
		}
		for _, row := range t.Rows {
			// Pad short rows to header length
			padded := make([]string, len(t.Headers))
			copy(padded, row)
			if err := w.Write(padded); err != nil {
				return nil, err
			}
		}
	} else {
		// No table: create a Section / Content fallback
		if err := w.Write([]string{"Section", "Type", "Content"}); err != nil {
			return nil, err
		}
		section := ""
		for _, b := range a.Blocks {
			switch b.Type {
			case BlockHeading:
				section = b.Text
			case BlockParagraph:
				_ = w.Write([]string{section, "paragraph", b.Text})
			case BlockBullet, BlockOrdered:
				for _, item := range b.Items {
					_ = w.Write([]string{section, "list item", item})
				}
			case BlockCode:
				if b.Code != nil {
					_ = w.Write([]string{section, "code", b.Code.Content})
				}
			}
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return &RenderedArtifact{
		ContentType: ContentTypeForFormat(FormatCsv),
		Bytes:       buf.Bytes(),
	}, nil
}
