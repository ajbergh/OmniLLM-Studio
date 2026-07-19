package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
