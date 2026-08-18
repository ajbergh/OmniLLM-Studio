package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
)

func TestParsePDFExtractsTextWithPureGoParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.pdf")
	doc := fpdf.New("P", "mm", "A4", "")
	doc.AddPage()
	doc.SetFont("Arial", "", 12)
	doc.MultiCell(0, 8, "OmniLLM secure PDF parser migration", "", "L", false)
	if err := doc.OutputFileAndClose(path); err != nil {
		t.Fatalf("write test PDF: %v", err)
	}

	parsed, err := ParseFile(path, "application/pdf")
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	markdown := strings.Join(strings.Fields(RenderMarkdown(parsed)), " ")
	if !strings.Contains(markdown, "OmniLLM secure PDF parser migration") {
		t.Fatalf("expected extracted PDF text, got %q", markdown)
	}
	if len(parsed.Nodes) == 0 || parsed.Nodes[0].Metadata["parser"] != "tsawler/tabula" {
		t.Fatalf("expected tsawler/tabula parser metadata, got %#v", parsed.Nodes)
	}
}

func TestParseHTMLPreservesHeadingsAndIgnoresNavigation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.html")
	content := `<html><body><nav>Ignore me</nav><h1>Architecture</h1><p>Keep this paragraph.</p><script>ignore()</script><h2>Storage</h2><p>SQLite remains authoritative.</p></body></html>`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseFile(path, "text/html")
	if err != nil {
		t.Fatal(err)
	}
	rendered := RenderMarkdown(parsed)
	if strings.Contains(rendered, "Ignore me") || strings.Contains(rendered, "ignore()") {
		t.Fatalf("navigation/script content leaked: %s", rendered)
	}
	if !strings.Contains(rendered, "# Architecture") || !strings.Contains(rendered, "## Storage") {
		t.Fatalf("heading hierarchy was not preserved: %s", rendered)
	}
}

func TestExtractPlainText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("alpha\n\nbeta"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := ExtractFileText(path, "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "alpha") || !strings.Contains(text, "beta") {
		t.Fatalf("missing text: %s", text)
	}
}
