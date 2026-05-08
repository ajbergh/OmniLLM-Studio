package artifacts

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Generator orchestrates artifact creation: it parses the raw LLM output,
// selects the appropriate renderer, writes the file to disk, and returns
// metadata for saving an attachment record.
type Generator struct {
	storageDir string
	registry   *Registry
}

// NewGenerator returns a Generator that stores files in storageDir.
func NewGenerator(storageDir string) *Generator {
	return &Generator{
		storageDir: storageDir,
		registry:   NewRegistry(),
	}
}

// Generate renders rawContent to format and writes the result to storageDir.
// It returns the storage-relative filename, byte size, and MIME type.
// suggestedFilename is used as the base name; if empty a timestamped default
// is generated.
func (g *Generator) Generate(ctx context.Context, rawContent string, format ArtifactFormat, suggestedFilename string) (storagePath string, size int64, contentType string, err error) {
	log.Printf("INFO: artifact: generating format=%s filename=%q", format, suggestedFilename)

	artifact := ParseMarkdown(rawContent)
	rendered, err := g.registry.Render(ctx, format, artifact)
	if err != nil {
		return "", 0, "", fmt.Errorf("artifact render %s: %w", format, err)
	}

	filename := SafeFilename(suggestedFilename, format)

	if err := os.MkdirAll(g.storageDir, 0o755); err != nil {
		return "", 0, "", fmt.Errorf("artifact mkdir: %w", err)
	}

	destPath := filepath.Join(g.storageDir, filename)
	if err := os.WriteFile(destPath, rendered.Bytes, 0o644); err != nil {
		return "", 0, "", fmt.Errorf("artifact write: %w", err)
	}

	log.Printf("INFO: artifact: wrote %s (%d bytes)", filename, len(rendered.Bytes))
	return filename, int64(len(rendered.Bytes)), ContentTypeForFormat(format), nil
}
