package artifacts

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---- format detection ----

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		msg    string
		want   ArtifactFormat
		wantOK bool
	}{
		{"put this in Excel", FormatXlsx, true},
		{"export as xlsx", FormatXlsx, true},
		{"create a spreadsheet", FormatXlsx, true},
		{"export as CSV", FormatCsv, true},
		{"give me a csv", FormatCsv, true},
		{"make this a PDF", FormatPdf, true},
		{"export as PDF", FormatPdf, true},
		{"export as markdown", FormatMarkdown, true},
		{"save as md", FormatMarkdown, true},
		{"make this a README", FormatMarkdown, true},
		{"export as HTML", FormatHtml, true},
		{"make this a web page", FormatHtml, true},
		{"return as JSON", FormatJson, true},
		{"give me json", FormatJson, true},
		{"return as YAML", FormatYaml, true},
		{"kubernetes yaml", FormatYaml, true},
		{"just tell me about Go", "", false},
		{"what is an excel formula?", "", false},
	}

	for _, c := range cases {
		got, ok := DetectFormat(c.msg)
		if ok != c.wantOK {
			t.Errorf("DetectFormat(%q) ok=%v want %v", c.msg, ok, c.wantOK)
			continue
		}
		if ok && got != c.want {
			t.Errorf("DetectFormat(%q) = %q want %q", c.msg, got, c.want)
		}
	}
}

// ---- safe filename ----

func TestSafeFilename(t *testing.T) {
	cases := []struct {
		name   string
		format ArtifactFormat
		suffix string
	}{
		{"project plan", FormatXlsx, ".xlsx"},
		{"interview review", FormatPdf, ".pdf"},
		{"comparison matrix", FormatCsv, ".csv"},
		{"", FormatMarkdown, ".md"},
		{"report.docx", FormatJson, ".json"},
	}
	for _, c := range cases {
		got := SafeFilename(c.name, c.format)
		if !strings.HasSuffix(got, c.suffix) {
			t.Errorf("SafeFilename(%q, %q) = %q; want suffix %q", c.name, c.format, got, c.suffix)
		}
		if strings.ContainsAny(got, `<>:"/\|?*`) {
			t.Errorf("SafeFilename returned unsafe filename: %q", got)
		}
	}
}

// ---- markdown parser ----

func TestParseMarkdownHeadings(t *testing.T) {
	md := "# Title\n\n## Section One\n\nSome paragraph.\n\n### Subsection\n\nMore text."
	a := ParseMarkdown(md)
	if a.Title != "Title" {
		t.Errorf("expected title 'Title', got %q", a.Title)
	}
	headings := 0
	for _, b := range a.Blocks {
		if b.Type == BlockHeading {
			headings++
		}
	}
	if headings != 3 {
		t.Errorf("expected 3 headings, got %d", headings)
	}
}

func TestParseMarkdownTable(t *testing.T) {
	md := "| Name | Score |\n|------|-------|\n| Alice | 95 |\n| Bob | 87 |"
	a := ParseMarkdown(md)
	if len(a.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(a.Tables))
	}
	if len(a.Tables[0].Headers) != 2 {
		t.Errorf("expected 2 headers, got %d", len(a.Tables[0].Headers))
	}
	if len(a.Tables[0].Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(a.Tables[0].Rows))
	}
}

func TestParseMarkdownBullets(t *testing.T) {
	md := "- apple\n- banana\n- cherry"
	a := ParseMarkdown(md)
	var bullets *Block
	for i := range a.Blocks {
		if a.Blocks[i].Type == BlockBullet {
			bullets = &a.Blocks[i]
		}
	}
	if bullets == nil {
		t.Fatal("expected bullet block")
	}
	if len(bullets.Items) != 3 {
		t.Errorf("expected 3 bullet items, got %d", len(bullets.Items))
	}
}

// ---- CSV renderer ----

func TestCSVRenderer(t *testing.T) {
	a := Artifact{
		Tables: []Table{{
			Headers: []string{"Name", "Score"},
			Rows:    [][]string{{"Alice", "95"}, {"Bob", "87"}},
		}},
	}
	r := &CSVRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(ra.Bytes) == 0 {
		t.Error("CSV output is empty")
	}
	out := string(ra.Bytes)
	if !strings.Contains(out, "Name") || !strings.Contains(out, "Alice") {
		t.Errorf("CSV missing expected content: %s", out)
	}
}

// ---- XLSX renderer ----

func TestXLSXRenderer(t *testing.T) {
	a := Artifact{
		Tables: []Table{{
			Headers: []string{"Product", "Qty", "Price"},
			Rows: [][]string{
				{"Widget A", "10", "9.99"},
				{"Widget B", "5", "19.99"},
			},
		}},
	}
	r := &XLSXRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	// XLSX files start with PK (ZIP magic bytes)
	if len(ra.Bytes) < 4 || string(ra.Bytes[:2]) != "PK" {
		t.Error("output does not look like a valid XLSX (ZIP) file")
	}
}

// ---- JSON renderer ----

func TestJSONRendererFromFence(t *testing.T) {
	raw := "Here is your data:\n\n```json\n{\"key\": \"value\", \"count\": 42}\n```"
	a := ParseMarkdown(raw)
	a.RawContent = raw
	r := &JSONRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]interface{}
	if err := json.Unmarshal(ra.Bytes, &v); err != nil {
		t.Errorf("output is not valid JSON: %v\n%s", err, ra.Bytes)
	}
	if v["key"] != "value" {
		t.Errorf("expected key=value, got %v", v["key"])
	}
}

func TestJSONRendererFallback(t *testing.T) {
	a := Artifact{
		Title: "Test",
		Tables: []Table{{
			Headers: []string{"A", "B"},
			Rows:    [][]string{{"1", "2"}},
		}},
	}
	r := &JSONRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	var v interface{}
	if err := json.Unmarshal(ra.Bytes, &v); err != nil {
		t.Errorf("fallback JSON is invalid: %v", err)
	}
}

// ---- YAML renderer ----

func TestYAMLRendererFromFence(t *testing.T) {
	raw := "Config:\n\n```yaml\napp: myapp\nreplicas: 3\n```"
	a := ParseMarkdown(raw)
	a.RawContent = raw
	r := &YAMLRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]interface{}
	if err := yaml.Unmarshal(ra.Bytes, &v); err != nil {
		t.Errorf("output is not valid YAML: %v\n%s", err, ra.Bytes)
	}
	if v["app"] != "myapp" {
		t.Errorf("expected app=myapp, got %v", v["app"])
	}
}

func TestYAMLRendererFallback(t *testing.T) {
	a := Artifact{Title: "Config", Blocks: []Block{
		{Type: BlockHeading, Level: 2, Text: "Settings"},
		{Type: BlockBullet, Items: []string{"enable logging", "set timeout: 30s"}},
	}}
	r := &YAMLRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	var v interface{}
	if err := yaml.Unmarshal(ra.Bytes, &v); err != nil {
		t.Errorf("fallback YAML is invalid: %v\n%s", err, ra.Bytes)
	}
}

// ---- Markdown renderer ----

func TestMarkdownRendererPreservesContent(t *testing.T) {
	raw := "# My Doc\n\n## Section\n\nSome text.\n\n| Col1 | Col2 |\n|------|------|\n| A | B |"
	a := ParseMarkdown(raw)
	a.RawContent = raw
	r := &MarkdownRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	out := string(ra.Bytes)
	if !strings.Contains(out, "# My Doc") {
		t.Error("heading missing from markdown output")
	}
	if !strings.Contains(out, "| Col1 |") {
		t.Error("table missing from markdown output")
	}
}

// ---- HTML renderer ----

func TestHTMLRendererOutput(t *testing.T) {
	a := Artifact{
		Title: "Test Report",
		Blocks: []Block{
			{Type: BlockHeading, Level: 2, Text: "Overview"},
			{Type: BlockParagraph, Text: "Some <script>alert(1)</script> text."},
		},
	}
	r := &HTMLRenderer{}
	ra, err := r.Render(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	out := string(ra.Bytes)
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE")
	}
	if !strings.Contains(out, "<h2>Overview</h2>") {
		t.Error("heading not rendered")
	}
	// XSS: script tag must be escaped
	if strings.Contains(out, "<script>") {
		t.Error("unescaped <script> tag found — XSS risk")
	}
}
