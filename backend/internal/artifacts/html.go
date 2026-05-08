package artifacts

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// HTMLRenderer converts an Artifact to a self-contained HTML document.
type HTMLRenderer struct{}

func (r *HTMLRenderer) Format() ArtifactFormat { return FormatHtml }

func (r *HTMLRenderer) Render(_ context.Context, a Artifact) (*RenderedArtifact, error) {
	var body strings.Builder
	renderBlocksAsHTML(&body, a)

	title := "Document"
	if a.Title != "" {
		title = a.Title
	}

	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
         max-width: 860px; margin: 40px auto; padding: 0 24px;
         color: #1a1a1a; line-height: 1.6; }
  h1 { font-size: 2em; border-bottom: 2px solid #e0e0e0; padding-bottom: 8px; margin-top: 0; }
  h2 { font-size: 1.5em; border-bottom: 1px solid #e0e0e0; padding-bottom: 4px; }
  h3 { font-size: 1.25em; }
  h4, h5, h6 { font-size: 1em; }
  table { border-collapse: collapse; width: 100%%; margin: 16px 0; }
  th { background: #f0f0f0; font-weight: 600; }
  th, td { border: 1px solid #d0d0d0; padding: 8px 12px; text-align: left; }
  tr:nth-child(even) td { background: #fafafa; }
  pre { background: #f5f5f5; border: 1px solid #e0e0e0; border-radius: 6px;
        padding: 16px; overflow-x: auto; font-size: 0.9em; }
  code { font-family: 'SFMono-Regular', Consolas, monospace; font-size: 0.9em;
         background: #f0f0f0; padding: 2px 4px; border-radius: 3px; }
  pre code { background: none; padding: 0; }
  blockquote { border-left: 4px solid #d0d0d0; margin: 0; padding: 8px 16px; color: #555; }
  ul, ol { padding-left: 24px; }
  li { margin: 4px 0; }
  hr { border: none; border-top: 1px solid #e0e0e0; margin: 24px 0; }
  p { margin: 12px 0; }
</style>
</head>
<body>
%s
</body>
</html>
`, html.EscapeString(title), body.String())

	return &RenderedArtifact{
		ContentType: ContentTypeForFormat(FormatHtml),
		Bytes:       []byte(doc),
	}, nil
}

func renderBlocksAsHTML(sb *strings.Builder, a Artifact) {
	for _, b := range a.Blocks {
		switch b.Type {
		case BlockHeading:
			tag := fmt.Sprintf("h%d", clampHeadingLevel(b.Level))
			fmt.Fprintf(sb, "<%s>%s</%s>\n", tag, html.EscapeString(b.Text), tag)
		case BlockParagraph:
			fmt.Fprintf(sb, "<p>%s</p>\n", html.EscapeString(b.Text))
		case BlockBullet:
			sb.WriteString("<ul>\n")
			for _, item := range b.Items {
				fmt.Fprintf(sb, "  <li>%s</li>\n", html.EscapeString(item))
			}
			sb.WriteString("</ul>\n")
		case BlockOrdered:
			sb.WriteString("<ol>\n")
			for _, item := range b.Items {
				fmt.Fprintf(sb, "  <li>%s</li>\n", html.EscapeString(item))
			}
			sb.WriteString("</ol>\n")
		case BlockCode:
			if b.Code != nil {
				lang := ""
				if b.Code.Language != "" {
					lang = fmt.Sprintf(` class="language-%s"`, html.EscapeString(b.Code.Language))
				}
				fmt.Fprintf(sb, "<pre><code%s>%s</code></pre>\n",
					lang, html.EscapeString(b.Code.Content))
			}
		case BlockTable:
			if b.Table != nil {
				renderHTMLTable(sb, b.Table)
			}
		case BlockQuote:
			sb.WriteString("<blockquote>\n")
			for _, line := range strings.Split(b.Text, "\n") {
				fmt.Fprintf(sb, "  <p>%s</p>\n", html.EscapeString(line))
			}
			sb.WriteString("</blockquote>\n")
		case BlockHR:
			sb.WriteString("<hr>\n")
		}
	}
}

func renderHTMLTable(sb *strings.Builder, t *Table) {
	sb.WriteString("<table>\n<thead><tr>\n")
	for _, h := range t.Headers {
		fmt.Fprintf(sb, "  <th>%s</th>\n", html.EscapeString(h))
	}
	sb.WriteString("</tr></thead>\n<tbody>\n")
	for _, row := range t.Rows {
		sb.WriteString("<tr>\n")
		for j := range t.Headers {
			val := ""
			if j < len(row) {
				val = row[j]
			}
			fmt.Fprintf(sb, "  <td>%s</td>\n", html.EscapeString(val))
		}
		sb.WriteString("</tr>\n")
	}
	sb.WriteString("</tbody></table>\n")
}

func clampHeadingLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 6 {
		return 6
	}
	return level
}
