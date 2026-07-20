package api

// File overview: centralizes attachment MIME eligibility and structure-preserving text extraction for conversation RAG indexing.

import (
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/document"
)

func normalizeMIMEType(mime string) string {
	return document.NormalizeMIMEType(mime)
}

func isPDFMIME(mime string) bool {
	return normalizeMIMEType(mime) == "application/pdf"
}

func isDocxMIME(mime string) bool {
	return normalizeMIMEType(mime) == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}

func isXlsxMIME(mime string) bool {
	return normalizeMIMEType(mime) == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

func isPptxMIME(mime string) bool {
	return normalizeMIMEType(mime) == "application/vnd.openxmlformats-officedocument.presentationml.presentation"
}

// canExtractAttachmentText reports whether a pure-Go parser can derive text
// suitable for prompt context and RAG chunking.
func canExtractAttachmentText(mime string) bool {
	m := normalizeMIMEType(mime)
	return document.IsTextMIME(m) || isPDFMIME(m) || isDocxMIME(m) || isXlsxMIME(m) || isPptxMIME(m) || m == "application/xhtml+xml"
}

func extractAttachmentText(path, mime string) (string, error) {
	if !canExtractAttachmentText(mime) {
		return "", fmt.Errorf("unsupported attachment mime type: %s", mime)
	}
	return document.ExtractFileText(path, mime)
}

func extractPDFText(path string) (string, error) {
	return document.ExtractFileText(path, "application/pdf")
}

func extractDocxText(path string) (string, error) {
	return document.ExtractFileText(path, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
}

func extractXlsxText(path string) (string, error) {
	return document.ExtractFileText(path, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func extractPptxText(path string) (string, error) {
	return document.ExtractFileText(path, "application/vnd.openxmlformats-officedocument.presentationml.presentation")
}

// normalizeExtractedText is retained for callers/tests that need a stable
// whitespace-normalized comparison.
func normalizeExtractedText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
