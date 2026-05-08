package artifacts

import (
	"context"
	"fmt"
	"strings"
)

// MarkdownRenderer returns the artifact as normalised Markdown text.
// If the raw content is available it is returned with light normalisation;
// otherwise it is reconstructed from the parsed block model.
type MarkdownRenderer struct{}

func (r *MarkdownRenderer) Format() ArtifactFormat { return FormatMarkdown }

func (r *MarkdownRenderer) Render(_ context.Context, a Artifact) (*RenderedArtifact, error) {
	var sb strings.Builder

	if a.RawContent != "" {
		sb.WriteString(strings.TrimSpace(a.RawContent))
		sb.WriteString("\n")
	} else {
		renderBlocksAsMarkdown(&sb, a)
	}

	return &RenderedArtifact{
		ContentType: ContentTypeForFormat(FormatMarkdown),
		Bytes:       []byte(sb.String()),
	}, nil
}

// renderBlocksAsMarkdown reconstructs Markdown from the parsed block model.
// Used as fallback when RawContent is empty.
func renderBlocksAsMarkdown(sb *strings.Builder, a Artifact) {
	for i, b := range a.Blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		switch b.Type {
		case BlockHeading:
			sb.WriteString(strings.Repeat("#", b.Level))
			sb.WriteString(" ")
			sb.WriteString(b.Text)
			sb.WriteString("\n")
		case BlockParagraph:
			sb.WriteString(b.Text)
			sb.WriteString("\n")
		case BlockBullet:
			for _, item := range b.Items {
				sb.WriteString("- ")
				sb.WriteString(item)
				sb.WriteString("\n")
			}
		case BlockOrdered:
			for j, item := range b.Items {
				fmt.Fprintf(sb, "%d. %s\n", j+1, item)
			}
		case BlockCode:
			if b.Code != nil {
				sb.WriteString("```")
				sb.WriteString(b.Code.Language)
				sb.WriteString("\n")
				sb.WriteString(b.Code.Content)
				sb.WriteString("\n```\n")
			}
		case BlockTable:
			if b.Table != nil {
				writeMarkdownTable(sb, b.Table)
			}
		case BlockQuote:
			for _, line := range strings.Split(b.Text, "\n") {
				sb.WriteString("> ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		case BlockHR:
			sb.WriteString("---\n")
		}
	}
}

func writeMarkdownTable(sb *strings.Builder, t *Table) {
	if len(t.Headers) == 0 {
		return
	}
	sb.WriteString("| ")
	sb.WriteString(strings.Join(t.Headers, " | "))
	sb.WriteString(" |\n")

	sb.WriteString("|")
	for range t.Headers {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")

	for _, row := range t.Rows {
		sb.WriteString("| ")
		cells := make([]string, len(t.Headers))
		for i := range cells {
			if i < len(row) {
				cells[i] = row[i]
			}
		}
		sb.WriteString(strings.Join(cells, " | "))
		sb.WriteString(" |\n")
	}
}
