package wordgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""}, // timestamped default – just check suffix
		{"report", "report.docx"},
		{"report.docx", "report.docx"},
		{"report.DOCX", "report.DOCX"},
		{"my report", "my report.docx"},
		{"a/b/c.docx", "c.docx"},
		{"bad<>name", "bad--name.docx"},
	}
	for _, tc := range cases {
		got := sanitizeFilename(tc.input)
		if tc.input == "" {
			if !strings.HasSuffix(got, ".docx") {
				t.Errorf("sanitizeFilename(%q) = %q; want .docx suffix", tc.input, got)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("sanitizeFilename(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	g := NewGenerator(dir)

	md := `# Hello World

This is a **test** document with:

- Item one
- Item two

## Section 2

Some _italic_ text and ` + "`code`" + ` inline.
`

	filename, size, err := g.Generate(md, "test-output.docx")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if filename != "test-output.docx" {
		t.Errorf("filename = %q; want test-output.docx", filename)
	}
	if size == 0 {
		t.Error("size = 0; expected non-zero")
	}

	// Verify the file exists on disk.
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("output file not found: %v", err)
	}

	// Verify it starts with the OOXML ZIP magic bytes (PK\x03\x04).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 4 || string(data[:4]) != "PK\x03\x04" {
		t.Errorf("output does not look like a ZIP/OOXML file (magic %x)", data[:min(4, len(data))])
	}
}

func TestGenerateDefaultFilename(t *testing.T) {
	dir := t.TempDir()
	g := NewGenerator(dir)

	filename, size, err := g.Generate("# Hi", "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasSuffix(filename, ".docx") {
		t.Errorf("filename %q does not end in .docx", filename)
	}
	if size == 0 {
		t.Error("size = 0")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
