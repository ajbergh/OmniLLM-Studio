package api

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

func normalizeMIMEType(mime string) string {
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = mime[:i]
	}
	return strings.TrimSpace(strings.ToLower(mime))
}

func isPDFMIME(mime string) bool {
	return normalizeMIMEType(mime) == "application/pdf"
}

// canExtractAttachmentText reports whether we can derive text content
// suitable for LLM prompt context or RAG chunking.
func canExtractAttachmentText(mime string) bool {
	return isTextMIME(mime) || isPDFMIME(mime)
}

// extractAttachmentText returns extracted text for text-like attachments.
func extractAttachmentText(path, mime string) (string, error) {
	if isTextMIME(mime) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	if isPDFMIME(mime) {
		return extractPDFText(path)
	}

	return "", fmt.Errorf("unsupported attachment mime type: %s", mime)
}

func extractPDFText(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	plainTextReader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract pdf text: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, plainTextReader); err != nil {
		return "", fmt.Errorf("read extracted pdf text: %w", err)
	}

	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", fmt.Errorf("pdf has no extractable text")
	}

	return text, nil
}
