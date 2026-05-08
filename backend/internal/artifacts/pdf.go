package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

// PDFRenderer converts an Artifact to a PDF document using go-pdf/fpdf.
// Uses built-in Helvetica/Courier fonts — no external font files required.
type PDFRenderer struct{}

func (r *PDFRenderer) Format() ArtifactFormat { return FormatPdf }

func (r *PDFRenderer) Render(_ context.Context, a Artifact) (*RenderedArtifact, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	// Unicode translator: converts UTF-8 to Latin-1 (cp1252).
	// Characters outside Latin-1 are replaced with '?' — adequate for most
	// LLM output; full Unicode would require embedded TTF fonts.
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pw, _ := pdf.GetPageSize()
	lm, _, rm, _ := pdf.GetMargins()
	contentW := pw - lm - rm

	// Title
	if a.Title != "" {
		pdf.SetFont("Helvetica", "B", 20)
		pdf.MultiCell(contentW, 10, tr(a.Title), "", "L", false)
		pdf.Ln(2)
	}

	// Subtitle
	if a.Subtitle != "" {
		pdf.SetFont("Helvetica", "I", 12)
		pdf.MultiCell(contentW, 6, tr(a.Subtitle), "", "L", false)
		pdf.Ln(2)
	}

	// Content blocks
	for _, b := range a.Blocks {
		renderPDFBlock(pdf, tr, b, contentW)
	}

	if pdf.Error() != nil {
		return nil, fmt.Errorf("fpdf error: %w", pdf.Error())
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}

	return &RenderedArtifact{
		ContentType: ContentTypeForFormat(FormatPdf),
		Bytes:       buf.Bytes(),
	}, nil
}

func renderPDFBlock(pdf *fpdf.Fpdf, tr func(string) string, b Block, contentW float64) {
	switch b.Type {
	case BlockHeading:
		renderPDFHeading(pdf, tr, b, contentW)
	case BlockParagraph:
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(contentW, 5, tr(b.Text), "", "", false)
		pdf.Ln(2)
	case BlockBullet:
		pdf.SetFont("Helvetica", "", 10)
		for _, item := range b.Items {
			x := pdf.GetX()
			pdf.SetX(x + 5)
			pdf.MultiCell(contentW-5, 5, tr("• "+item), "", "", false)
		}
		pdf.Ln(1)
	case BlockOrdered:
		pdf.SetFont("Helvetica", "", 10)
		for i, item := range b.Items {
			x := pdf.GetX()
			pdf.SetX(x + 5)
			pdf.MultiCell(contentW-5, 5, tr(fmt.Sprintf("%d. %s", i+1, item)), "", "", false)
		}
		pdf.Ln(1)
	case BlockCode:
		if b.Code != nil {
			pdf.SetFont("Courier", "", 8)
			pdf.SetFillColor(245, 245, 245)
			for _, line := range strings.Split(b.Code.Content, "\n") {
				pdf.MultiCell(contentW, 4.5, tr(line), "", "", true)
			}
			pdf.SetFillColor(255, 255, 255)
			pdf.Ln(2)
		}
	case BlockTable:
		if b.Table != nil {
			renderPDFTable(pdf, tr, b.Table, contentW)
			pdf.Ln(4)
		}
	case BlockQuote:
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetX(pdf.GetX() + 8)
		for _, line := range strings.Split(b.Text, "\n") {
			pdf.MultiCell(contentW-8, 5, tr(line), "L", "", false)
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(2)
	case BlockHR:
		y := pdf.GetY()
		pdf.SetDrawColor(180, 180, 180)
		pdf.Line(20, y, 20+contentW, y)
		pdf.SetDrawColor(0, 0, 0)
		pdf.Ln(4)
	}
}

func renderPDFHeading(pdf *fpdf.Fpdf, tr func(string) string, b Block, contentW float64) {
	pdf.Ln(2)
	switch b.Level {
	case 1:
		pdf.SetFont("Helvetica", "B", 18)
		pdf.MultiCell(contentW, 9, tr(b.Text), "", "L", false)
	case 2:
		pdf.SetFont("Helvetica", "B", 14)
		pdf.MultiCell(contentW, 8, tr(b.Text), "", "L", false)
	case 3:
		pdf.SetFont("Helvetica", "B", 12)
		pdf.MultiCell(contentW, 7, tr(b.Text), "", "L", false)
	default:
		pdf.SetFont("Helvetica", "B", 11)
		pdf.MultiCell(contentW, 6, tr(b.Text), "", "L", false)
	}
	pdf.Ln(2)
}

func renderPDFTable(pdf *fpdf.Fpdf, tr func(string) string, t *Table, contentW float64) {
	if len(t.Headers) == 0 {
		return
	}

	numCols := len(t.Headers)
	colW := contentW / float64(numCols)
	// Cap individual column width to prevent very narrow columns on wide tables
	if colW < 15 {
		colW = 15
	}

	// Table name / caption
	if t.Name != "" {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.Cell(contentW, 5, tr(t.Name))
		pdf.Ln(5)
	}

	// Header row
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	for _, h := range t.Headers {
		label := truncate(h, 30)
		pdf.CellFormat(colW, 7, tr(label), "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	// Data rows
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetFillColor(255, 255, 255)
	for _, row := range t.Rows {
		for j := range t.Headers {
			val := ""
			if j < len(row) {
				val = truncate(row[j], 35)
			}
			pdf.CellFormat(colW, 6, tr(val), "1", 0, "", false, 0, "")
		}
		pdf.Ln(-1)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
