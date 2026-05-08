package artifacts

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// XLSXRenderer exports an Artifact to an Excel workbook (.xlsx).
// One worksheet per table; if no tables are present a Content sheet is created.
type XLSXRenderer struct{}

func (r *XLSXRenderer) Format() ArtifactFormat { return FormatXlsx }

func (r *XLSXRenderer) Render(_ context.Context, a Artifact) (*RenderedArtifact, error) {
	f := excelize.NewFile()
	defer f.Close()

	if len(a.Tables) == 0 {
		sheetName := "Content"
		f.SetSheetName("Sheet1", sheetName)
		if err := writeContentSheet(f, sheetName, a); err != nil {
			return nil, err
		}
	} else {
		for idx, t := range a.Tables {
			sheetName := sheetNameFor(t, a, idx)
			if idx == 0 {
				f.SetSheetName("Sheet1", sheetName)
			} else {
				if _, err := f.NewSheet(sheetName); err != nil {
					return nil, err
				}
			}
			if err := writeTableToSheet(f, sheetName, t); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}

	return &RenderedArtifact{
		ContentType: ContentTypeForFormat(FormatXlsx),
		Bytes:       buf.Bytes(),
	}, nil
}

func writeTableToSheet(f *excelize.File, sheetName string, t Table) error {
	if len(t.Headers) == 0 {
		return nil
	}

	// Bold header style
	boldStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E8E8E8"}, Pattern: 1},
	})
	if err != nil {
		return err
	}

	// Write headers
	for j, h := range t.Headers {
		cell, _ := excelize.CoordinatesToCellName(j+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}
	// Apply bold style to header row
	startCell := "A1"
	endCell, _ := excelize.CoordinatesToCellName(len(t.Headers), 1)
	f.SetCellStyle(sheetName, startCell, endCell, boldStyle)

	// Write data rows
	for i, row := range t.Rows {
		for j, val := range row {
			if j >= len(t.Headers) {
				break
			}
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				f.SetCellValue(sheetName, cell, n)
			} else {
				f.SetCellValue(sheetName, cell, val)
			}
		}
	}

	// Freeze first row
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
		Selection: []excelize.Selection{
			{SQRef: "A2", ActiveCell: "A2", Pane: "bottomLeft"},
		},
	})

	// AutoFilter over the whole table range
	if len(t.Rows) > 0 {
		lastCell, _ := excelize.CoordinatesToCellName(len(t.Headers), len(t.Rows)+1)
		f.AutoFilter(sheetName, "A1:"+lastCell, nil)
	}

	// Column widths based on max content length
	for j, h := range t.Headers {
		colName, _ := excelize.ColumnNumberToName(j + 1)
		maxLen := len(h)
		for _, row := range t.Rows {
			if j < len(row) && len(row[j]) > maxLen {
				maxLen = len(row[j])
			}
		}
		width := float64(maxLen) + 4
		if width < 10 {
			width = 10
		}
		if width > 40 {
			width = 40
		}
		f.SetColWidth(sheetName, colName, colName, width)
	}
	return nil
}

func writeContentSheet(f *excelize.File, sheetName string, a Artifact) error {
	headers := []string{"Section", "Type", "Content"}
	t := Table{Headers: headers}

	section := ""
	for _, b := range a.Blocks {
		switch b.Type {
		case BlockHeading:
			section = b.Text
		case BlockParagraph:
			t.Rows = append(t.Rows, []string{section, "paragraph", b.Text})
		case BlockBullet, BlockOrdered:
			for _, item := range b.Items {
				t.Rows = append(t.Rows, []string{section, "list item", item})
			}
		case BlockCode:
			if b.Code != nil {
				t.Rows = append(t.Rows, []string{section, "code", b.Code.Content})
			}
		}
	}
	return writeTableToSheet(f, sheetName, t)
}

func sheetNameFor(t Table, a Artifact, idx int) string {
	name := t.Name
	if name == "" && idx == 0 {
		name = a.Title
	}
	if name == "" {
		name = fmt.Sprintf("Sheet%d", idx+1)
	}
	return sanitizeSheetName(name)
}

func sanitizeSheetName(name string) string {
	// Excel sheet name constraints: max 31 chars, no: \ / * ? : [ ]
	replacer := strings.NewReplacer(
		`\`, "-", `/`, "-", `*`, "-", `?`, "-",
		`:`, "-", `[`, "(", `]`, ")",
	)
	name = replacer.Replace(name)
	name = strings.TrimSpace(name)
	if len(name) > 31 {
		name = name[:31]
	}
	if name == "" {
		name = "Sheet"
	}
	return name
}
