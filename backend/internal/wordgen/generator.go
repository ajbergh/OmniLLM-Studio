package wordgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/drumkitai/go-word/pkg/markdown"
)

// Generator converts Markdown text to .docx files and saves them to a storage directory.
type Generator struct {
	storageDir string
}

// NewGenerator creates a Generator that writes .docx files into storageDir.
func NewGenerator(storageDir string) *Generator {
	return &Generator{storageDir: storageDir}
}

// Generate converts markdownContent to a .docx file.
// filename is the desired base name (e.g. "report.docx"); if empty or missing the
// .docx extension, a timestamped default is used.
// Returns the storage-relative filename and the file's byte size.
func (g *Generator) Generate(markdownContent, filename string) (string, int64, error) {
	filename = sanitizeFilename(filename)

	converter := markdown.NewConverter(markdown.DefaultOptions())
	doc, err := converter.ConvertString(markdownContent, nil)
	if err != nil {
		return "", 0, fmt.Errorf("convert markdown to docx: %w", err)
	}

	destPath := filepath.Join(g.storageDir, filename)
	if err := os.MkdirAll(g.storageDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create storage dir: %w", err)
	}
	if err := doc.Save(destPath); err != nil {
		return "", 0, fmt.Errorf("save docx: %w", err)
	}

	fi, err := os.Stat(destPath)
	if err != nil {
		return "", 0, fmt.Errorf("stat docx: %w", err)
	}
	return filename, fi.Size(), nil
}

// sanitizeFilename normalises filename to a safe .docx base name.
// If blank, returns a timestamped default.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "document-" + time.Now().UTC().Format("20060102-150405") + ".docx"
	}

	// Strip the directory component if the caller accidentally passed a path.
	name = filepath.Base(name)

	// Replace characters that are unsafe in filenames on any OS.
	unsafe := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	name = unsafe.ReplaceAllString(name, "-")

	// Collapse repeated dashes / spaces and trim.
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		name = "document"
	}

	// Ensure the extension is .docx (case-insensitive).
	if !strings.EqualFold(filepath.Ext(name), ".docx") {
		name += ".docx"
	}
	return name
}
