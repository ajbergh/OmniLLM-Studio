package artifacts

import (
	"regexp"
	"strings"
)

var orderedLineRe = regexp.MustCompile(`^\d+\.\s+(.+)$`)

// ParseMarkdown converts raw LLM Markdown output into a structured Artifact.
// It handles headings, paragraphs, bullet lists, ordered lists, GFM tables,
// fenced code blocks, and blockquotes.
func ParseMarkdown(raw string) Artifact {
	a := Artifact{RawContent: raw}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	type parserState int
	const (
		stNormal    parserState = iota
		stCode                  // inside ``` fence
		stBullet                // accumulating bullet items
		stOrdered               // accumulating ordered items
		stTable                 // accumulating table rows
		stBlockquote            // accumulating blockquote lines
	)

	st := stNormal
	var codeLang string
	var codeLines []string
	var bulletItems []string
	var orderedItems []string
	var tableHeaders []string
	var tableRows [][]string
	var bqLines []string

	flush := func() {
		switch st {
		case stBullet:
			if len(bulletItems) > 0 {
				a.Blocks = append(a.Blocks, Block{Type: BlockBullet, Items: append([]string(nil), bulletItems...)})
				bulletItems = bulletItems[:0]
			}
		case stOrdered:
			if len(orderedItems) > 0 {
				a.Blocks = append(a.Blocks, Block{Type: BlockOrdered, Items: append([]string(nil), orderedItems...)})
				orderedItems = orderedItems[:0]
			}
		case stTable:
			if len(tableHeaders) > 0 {
				t := Table{Headers: append([]string(nil), tableHeaders...), Rows: append([][]string(nil), tableRows...)}
				a.Blocks = append(a.Blocks, Block{Type: BlockTable, Table: &t})
				a.Tables = append(a.Tables, t)
				tableHeaders = tableHeaders[:0]
				tableRows = tableRows[:0]
			}
		case stBlockquote:
			if len(bqLines) > 0 {
				a.Blocks = append(a.Blocks, Block{Type: BlockQuote, Text: strings.Join(bqLines, "\n")})
				bqLines = bqLines[:0]
			}
		}
		st = stNormal
	}

	isSeparatorRow := func(s string) bool {
		if !strings.Contains(s, "-") {
			return false
		}
		for _, c := range s {
			if c != '|' && c != '-' && c != ':' && c != ' ' && c != '\t' {
				return false
			}
		}
		return true
	}

	parseTableRow := func(s string) []string {
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "|")
		s = strings.TrimSuffix(s, "|")
		parts := strings.Split(s, "|")
		result := make([]string, len(parts))
		for i, p := range parts {
			result[i] = strings.TrimSpace(p)
		}
		return result
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Fenced code block delimiter
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			if st == stCode {
				a.Blocks = append(a.Blocks, Block{
					Type: BlockCode,
					Code: &CodeBlock{Language: codeLang, Content: strings.Join(codeLines, "\n")},
				})
				codeLines = codeLines[:0]
				codeLang = ""
				st = stNormal
			} else {
				flush()
				st = stCode
				if strings.HasPrefix(trimmed, "```") {
					codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				} else {
					codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "~~~"))
				}
			}
			continue
		}

		if st == stCode {
			codeLines = append(codeLines, line)
			continue
		}

		// Empty line
		if trimmed == "" {
			if st != stNormal {
				flush()
			}
			continue
		}

		// Heading
		if strings.HasPrefix(trimmed, "#") {
			flush()
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			if level > 6 {
				level = 6
			}
			text := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			a.Blocks = append(a.Blocks, Block{Type: BlockHeading, Level: level, Text: text})
			if level == 1 && a.Title == "" {
				a.Title = text
			}
			continue
		}

		// GFM Table: line starts with |
		if strings.HasPrefix(trimmed, "|") {
			if st == stTable {
				if isSeparatorRow(trimmed) {
					continue // skip separator
				}
				tableRows = append(tableRows, parseTableRow(trimmed))
				continue
			}
			// Potential table start: next line must be separator
			if i+1 < len(lines) {
				nextTrimmed := strings.TrimSpace(lines[i+1])
				if strings.HasPrefix(nextTrimmed, "|") && isSeparatorRow(nextTrimmed) {
					flush()
					st = stTable
					tableHeaders = parseTableRow(trimmed)
					i++ // skip separator row
					continue
				}
			}
			// Not a table — fall through to paragraph
		}

		if st == stTable && !strings.HasPrefix(trimmed, "|") {
			flush()
		}

		// Horizontal rule
		if trimmed == "---" || trimmed == "***" || trimmed == "___" ||
			trimmed == "- - -" || trimmed == "* * *" || trimmed == "_ _ _" {
			flush()
			a.Blocks = append(a.Blocks, Block{Type: BlockHR})
			continue
		}

		// Blockquote
		if strings.HasPrefix(trimmed, ">") {
			if st != stBlockquote {
				flush()
				st = stBlockquote
			}
			text := strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " ")
			bqLines = append(bqLines, text)
			continue
		}
		if st == stBlockquote {
			flush()
		}

		// Bullet list
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			if st != stBullet {
				if st != stNormal {
					flush()
				}
				st = stBullet
			}
			bulletItems = append(bulletItems, trimmed[2:])
			continue
		}
		// Continuation indent of bullet
		if st == stBullet && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) {
			if len(bulletItems) > 0 {
				bulletItems[len(bulletItems)-1] += " " + trimmed
			}
			continue
		}
		if st == stBullet {
			flush()
		}

		// Ordered list
		if m := orderedLineRe.FindStringSubmatch(trimmed); m != nil {
			if st != stOrdered {
				if st != stNormal {
					flush()
				}
				st = stOrdered
			}
			orderedItems = append(orderedItems, m[1])
			continue
		}
		if st == stOrdered {
			flush()
		}

		// Paragraph
		a.Blocks = append(a.Blocks, Block{Type: BlockParagraph, Text: trimmed})
	}

	flush()
	return a
}
