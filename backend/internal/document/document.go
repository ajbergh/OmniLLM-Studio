package document

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// NodeType identifies a structural unit extracted from a source document.
type NodeType string

const (
	NodeDocument  NodeType = "document"
	NodePage      NodeType = "page"
	NodeSlide     NodeType = "slide"
	NodeSheet     NodeType = "sheet"
	NodeHeading   NodeType = "heading"
	NodeParagraph NodeType = "paragraph"
	NodeTable     NodeType = "table"
	NodeCode      NodeType = "code"
)

// Node preserves source structure before RAG chunking.
type Node struct {
	Type        NodeType          `json:"type"`
	Text        string            `json:"text"`
	PageNumber  *int              `json:"page_number,omitempty"`
	SlideNumber *int              `json:"slide_number,omitempty"`
	SheetName   string            `json:"sheet_name,omitempty"`
	HeadingPath []string          `json:"heading_path,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ParsedDocument is the canonical pure-Go parser output.
type ParsedDocument struct {
	MIMEType string            `json:"mime_type"`
	Title    string            `json:"title,omitempty"`
	Nodes    []Node            `json:"nodes"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

var slideNumberPattern = regexp.MustCompile(`slide(\d+)\.xml$`)

// ParseFile parses supported text and office formats without CGO.
func ParseFile(path, mime string) (*ParsedDocument, error) {
	mime = NormalizeMIMEType(mime)
	switch {
	case mime == "text/html" || mime == "application/xhtml+xml":
		return parseHTMLFile(path, mime)
	case IsTextMIME(mime):
		return parseTextFile(path, mime)
	case mime == "application/pdf":
		return parsePDF(path, mime)
	case mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return parseDOCX(path, mime)
	case mime == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return parseXLSX(path, mime)
	case mime == "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return parsePPTX(path, mime)
	default:
		return nil, fmt.Errorf("unsupported document mime type: %s", mime)
	}
}

// ExtractFileText renders a parsed document into structure-preserving Markdown
// suitable for prompt context and the existing RAG chunker.
func ExtractFileText(path, mime string) (string, error) {
	parsed, err := ParseFile(path, mime)
	if err != nil {
		return "", err
	}
	text := RenderMarkdown(parsed)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("document has no extractable text")
	}
	return text, nil
}

func NormalizeMIMEType(mime string) string {
	if index := strings.Index(mime, ";"); index >= 0 {
		mime = mime[:index]
	}
	return strings.TrimSpace(strings.ToLower(mime))
}

func IsTextMIME(mime string) bool {
	mime = NormalizeMIMEType(mime)
	return strings.HasPrefix(mime, "text/") ||
		mime == "application/json" ||
		mime == "application/xml" ||
		mime == "application/javascript" ||
		mime == "application/typescript" ||
		mime == "application/x-yaml" ||
		mime == "application/yaml" ||
		mime == "application/toml" ||
		mime == "application/x-sh" ||
		mime == "application/sql" ||
		mime == "application/csv"
}

func parseTextFile(path, mime string) (*ParsedDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, fmt.Errorf("document has no extractable text")
	}
	parsed := &ParsedDocument{MIMEType: mime, Title: filepath.Base(path)}
	if strings.Contains(mime, "markdown") || strings.EqualFold(filepath.Ext(path), ".md") {
		parsed.Nodes = markdownNodes(text)
	} else {
		parsed.Nodes = paragraphNodes(text, nil)
	}
	return parsed, nil
}

func parsePDF(path, mime string) (*ParsedDocument, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()
	plainTextReader, err := reader.GetPlainText()
	if err != nil {
		return nil, fmt.Errorf("extract pdf text: %w", err)
	}
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, plainTextReader); err != nil {
		return nil, fmt.Errorf("read extracted pdf text: %w", err)
	}
	text := strings.TrimSpace(buffer.String())
	if text == "" {
		return nil, fmt.Errorf("pdf has no extractable text")
	}
	return &ParsedDocument{
		MIMEType: mime,
		Title:    filepath.Base(path),
		Nodes:    paragraphNodes(text, map[string]string{"parser": "ledongthuc/pdf"}),
	}, nil
}

func parseDOCX(path, mime string) (*ParsedDocument, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	defer archive.Close()
	entry := findZipEntry(archive.File, "word/document.xml")
	if entry == nil {
		return nil, fmt.Errorf("word/document.xml not found")
	}
	nodes, err := parseWordXML(entry)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("document has no extractable text")
	}
	return &ParsedDocument{MIMEType: mime, Title: filepath.Base(path), Nodes: nodes}, nil
}

func parseWordXML(entry *zip.File) ([]Node, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decoder := xml.NewDecoder(reader)
	var nodes []Node
	var paragraph strings.Builder
	inText := false
	inTable := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "tbl":
				inTable = true
			case "t":
				inText = true
			case "tab":
				paragraph.WriteByte('\t')
			case "br":
				paragraph.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				paragraph.Write(value)
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "t":
				inText = false
			case "p":
				text := strings.TrimSpace(paragraph.String())
				if text != "" {
					typeName := NodeParagraph
					if inTable {
						typeName = NodeTable
					}
					nodes = append(nodes, Node{Type: typeName, Text: text})
				}
				paragraph.Reset()
			case "tbl":
				inTable = false
			}
		}
	}
	return nodes, nil
}

func parseXLSX(path, mime string) (*ParsedDocument, error) {
	workbook, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer workbook.Close()
	parsed := &ParsedDocument{MIMEType: mime, Title: filepath.Base(path)}
	for _, sheet := range workbook.GetSheetList() {
		rows, err := workbook.GetRows(sheet)
		if err != nil {
			continue
		}
		var builder strings.Builder
		for rowIndex, row := range rows {
			if rowIsEmpty(row) {
				continue
			}
			fmt.Fprintf(&builder, "Row %d: ", rowIndex+1)
			for columnIndex, value := range row {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				column, _ := excelize.ColumnNumberToName(columnIndex + 1)
				fmt.Fprintf(&builder, "%s=%s; ", column, value)
			}
			builder.WriteByte('\n')
		}
		text := strings.TrimSpace(builder.String())
		if text != "" {
			parsed.Nodes = append(parsed.Nodes, Node{Type: NodeSheet, Text: text, SheetName: sheet})
		}
	}
	if len(parsed.Nodes) == 0 {
		return nil, fmt.Errorf("spreadsheet has no extractable cells")
	}
	return parsed, nil
}

func parsePPTX(path, mime string) (*ParsedDocument, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open pptx: %w", err)
	}
	defer archive.Close()
	type slideEntry struct {
		number int
		file   *zip.File
	}
	var slides []slideEntry
	for _, entry := range archive.File {
		if !strings.HasPrefix(entry.Name, "ppt/slides/slide") || !strings.HasSuffix(entry.Name, ".xml") {
			continue
		}
		match := slideNumberPattern.FindStringSubmatch(entry.Name)
		if len(match) != 2 {
			continue
		}
		number, _ := strconv.Atoi(match[1])
		slides = append(slides, slideEntry{number: number, file: entry})
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].number < slides[j].number })
	parsed := &ParsedDocument{MIMEType: mime, Title: filepath.Base(path)}
	for _, slide := range slides {
		text, err := extractXMLText(slide.file, "t")
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		number := slide.number
		parsed.Nodes = append(parsed.Nodes, Node{Type: NodeSlide, Text: text, SlideNumber: &number})
	}
	if len(parsed.Nodes) == 0 {
		return nil, fmt.Errorf("presentation has no extractable text")
	}
	return parsed, nil
}

func parseHTMLFile(path, mime string) (*ParsedDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	root, err := html.Parse(file)
	if err != nil {
		return nil, err
	}
	var nodes []Node
	var headings []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			if tag == "script" || tag == "style" || tag == "nav" || tag == "footer" || tag == "noscript" {
				return
			}
			if tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" || tag == "h5" || tag == "h6" {
				text := strings.TrimSpace(nodeText(node))
				if text != "" {
					level, _ := strconv.Atoi(tag[1:])
					if level <= 0 {
						level = 1
					}
					if len(headings) >= level {
						headings = headings[:level-1]
					}
					headings = append(headings, text)
					nodes = append(nodes, Node{Type: NodeHeading, Text: text, HeadingPath: append([]string(nil), headings...)})
				}
				return
			}
			if tag == "p" || tag == "li" || tag == "pre" || tag == "blockquote" {
				text := strings.TrimSpace(nodeText(node))
				if text != "" {
					typeName := NodeParagraph
					if tag == "pre" {
						typeName = NodeCode
					}
					nodes = append(nodes, Node{Type: typeName, Text: text, HeadingPath: append([]string(nil), headings...)})
				}
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("html document has no extractable text")
	}
	return &ParsedDocument{MIMEType: mime, Title: filepath.Base(path), Nodes: nodes}, nil
}

// RenderMarkdown preserves structural boundaries using Markdown headings and
// explicit page/slide/sheet markers understood by the RAG chunker.
func RenderMarkdown(document *ParsedDocument) string {
	if document == nil {
		return ""
	}
	var builder strings.Builder
	if strings.TrimSpace(document.Title) != "" {
		builder.WriteString("# ")
		builder.WriteString(strings.TrimSpace(document.Title))
		builder.WriteString("\n\n")
	}
	for _, node := range document.Nodes {
		text := strings.TrimSpace(node.Text)
		if text == "" {
			continue
		}
		switch node.Type {
		case NodePage:
			if node.PageNumber != nil {
				fmt.Fprintf(&builder, "## Page %d\n\n", *node.PageNumber)
			}
		case NodeSlide:
			if node.SlideNumber != nil {
				fmt.Fprintf(&builder, "## Slide %d\n\n", *node.SlideNumber)
			}
		case NodeSheet:
			fmt.Fprintf(&builder, "## Sheet: %s\n\n", node.SheetName)
		case NodeHeading:
			level := len(node.HeadingPath)
			if level < 1 {
				level = 1
			}
			if level > 6 {
				level = 6
			}
			builder.WriteString(strings.Repeat("#", level))
			builder.WriteByte(' ')
			builder.WriteString(text)
			builder.WriteString("\n\n")
		case NodeCode:
			builder.WriteString("```\n")
			builder.WriteString(text)
			builder.WriteString("\n```\n\n")
		case NodeTable:
			builder.WriteString("Table:\n")
			builder.WriteString(text)
			builder.WriteString("\n\n")
		}
		if node.Type != NodeHeading && node.Type != NodePage && node.Type != NodeCode && node.Type != NodeTable {
			builder.WriteString(text)
			builder.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func markdownNodes(text string) []Node {
	var nodes []Node
	var headings []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	var paragraph strings.Builder
	flush := func() {
		value := strings.TrimSpace(paragraph.String())
		if value != "" {
			nodes = append(nodes, Node{Type: NodeParagraph, Text: value, HeadingPath: append([]string(nil), headings...)})
		}
		paragraph.Reset()
	}
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 0 && level < len(trimmed) && trimmed[level] == ' ' {
				flush()
				heading := strings.TrimSpace(trimmed[level:])
				if len(headings) >= level {
					headings = headings[:level-1]
				}
				headings = append(headings, heading)
				nodes = append(nodes, Node{Type: NodeHeading, Text: heading, HeadingPath: append([]string(nil), headings...)})
				continue
			}
		}
		if trimmed == "" {
			flush()
			continue
		}
		paragraph.WriteString(line)
		paragraph.WriteByte('\n')
	}
	flush()
	return nodes
}

func paragraphNodes(text string, metadata map[string]string) []Node {
	paragraphs := regexpSplitBlankLines(text)
	nodes := make([]Node, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			nodes = append(nodes, Node{Type: NodeParagraph, Text: paragraph, Metadata: metadata})
		}
	}
	return nodes
}

func regexpSplitBlankLines(text string) []string {
	var paragraphs []string
	var current strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if value := strings.TrimSpace(current.String()); value != "" {
				paragraphs = append(paragraphs, value)
				current.Reset()
			}
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if value := strings.TrimSpace(current.String()); value != "" {
		paragraphs = append(paragraphs, value)
	}
	return paragraphs
}

func findZipEntry(files []*zip.File, name string) *zip.File {
	for _, entry := range files {
		if entry.Name == name {
			return entry
		}
	}
	return nil
}

func extractXMLText(entry *zip.File, element string) (string, error) {
	reader, err := entry.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	decoder := xml.NewDecoder(reader)
	var builder strings.Builder
	inText := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == element {
				inText = true
			}
		case xml.EndElement:
			if value.Name.Local == element {
				inText = false
				builder.WriteByte(' ')
			}
		case xml.CharData:
			if inText {
				builder.Write(value)
			}
		}
	}
	return strings.Join(strings.Fields(builder.String()), " "), nil
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func rowIsEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
