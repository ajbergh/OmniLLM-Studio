package video

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	root string
}

func NewStorage(attachmentsDir string) *Storage {
	return &Storage{root: filepath.Join(attachmentsDir, "video")}
}

func (s *Storage) Root() string {
	return s.root
}

func (s *Storage) Write(projectID, generationID, fileName, mimeType string, data []byte) (relativePath, storedName string, err error) {
	if projectID == "" || generationID == "" {
		return "", "", fmt.Errorf("project_id and generation_id are required")
	}
	storedName = sanitizeFileName(fileName)
	if storedName == "" {
		storedName = "output" + extensionForMimeType(mimeType)
	}
	relativePath = filepath.Join("video", sanitizePathSegment(projectID), sanitizePathSegment(generationID), storedName)
	fullPath, err := safeJoin(filepath.Dir(s.root), relativePath)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", "", err
	}
	return relativePath, storedName, nil
}

func extensionForMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/wave", "audio/x-wav":
		return ".wav"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

func sanitizePathSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "item"
	}
	return value
}

func sanitizeFileName(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		default:
			return '-'
		}
	}, value)
	return strings.Trim(value, "-.")
}

func safeJoin(baseDir, untrustedPath string) (string, error) {
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
	if absJoined != absBase && !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory")
	}
	return joined, nil
}
