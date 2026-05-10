package filelibrary

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

func safeJoin(baseDir, untrustedPath string) (string, error) {
	if untrustedPath == "" {
		return "", fmt.Errorf("empty path")
	}
	cleaned := filepath.Clean(untrustedPath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("path traversal not allowed")
	}
	joined := filepath.Join(baseDir, cleaned)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) && absJoined != absBase {
		return "", fmt.Errorf("path escapes base directory")
	}
	return joined, nil
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func normalizeMIMEType(mime string) string {
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = mime[:i]
	}
	return strings.TrimSpace(strings.ToLower(mime))
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

func isTextMIME(mime string) bool {
	m := normalizeMIMEType(mime)
	return strings.HasPrefix(m, "text/") || m == "application/json" || m == "application/xml" || m == "application/x-yaml"
}

func extractFileText(path, mime string) (string, error) {
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
	if isDocxMIME(mime) {
		return extractDocxText(path)
	}
	if isXlsxMIME(mime) {
		return extractXlsxText(path)
	}
	if isPptxMIME(mime) {
		return extractPptxText(path)
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

func extractOOXMLText(zipPath string, entries []string, runElement string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var sb strings.Builder
	for _, f := range r.File {
		matched := false
		for _, e := range entries {
			if f.Name == e {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		inRun := false
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == runElement {
					inRun = true
				}
			case xml.EndElement:
				if t.Name.Local == runElement {
					inRun = false
				}
			case xml.CharData:
				if inRun {
					sb.Write(t)
					sb.WriteByte(' ')
				}
			}
		}
		sb.WriteByte('\n')
	}

	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", fmt.Errorf("document has no extractable text")
	}
	return text, nil
}

func extractDocxText(path string) (string, error) {
	return extractOOXMLText(path, []string{"word/document.xml"}, "t")
}

func extractXlsxText(path string) (string, error) {
	return extractOOXMLText(path, []string{"xl/sharedStrings.xml"}, "t")
}

func extractPptxText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	var slideEntries []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideEntries = append(slideEntries, f.Name)
		}
	}
	r.Close()
	if len(slideEntries) == 0 {
		return "", fmt.Errorf("no slides found in presentation")
	}
	return extractOOXMLText(path, slideEntries, "t")
}
