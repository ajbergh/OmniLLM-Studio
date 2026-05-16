package music

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
	return &Storage{root: filepath.Join(attachmentsDir, "music")}
}

func (s *Storage) Root() string {
	return s.root
}

func (s *Storage) Write(sessionID, generationID, mimeType string, data []byte) (relativePath, fileName string, err error) {
	if sessionID == "" || generationID == "" {
		return "", "", fmt.Errorf("session_id and generation_id are required")
	}
	ext := extensionForMimeType(mimeType)
	fileName = "output" + ext
	relativePath = filepath.Join("music", sanitizePathSegment(sessionID), sanitizePathSegment(generationID), fileName)
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
	return relativePath, fileName, nil
}

func extensionForMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/wave", "audio/x-wav":
		return ".wav"
	case "audio/ogg":
		return ".ogg"
	case "audio/flac":
		return ".flac"
	case "audio/aac":
		return ".aac"
	case "audio/mp4", "audio/m4a":
		return ".m4a"
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
