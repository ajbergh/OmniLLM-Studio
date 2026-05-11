package filelibrary

import (
	"context"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// Fetch returns a file/chunk and optional expanded full text.
func (s *LibraryService) Fetch(ctx context.Context, req FetchRequest) (*FetchedFile, error) {
	_ = ctx
	if strings.TrimSpace(req.OwnerUserID) == "" {
		// solo mode is allowed with empty owner_user_id
	}
	if strings.TrimSpace(req.LibraryFileID) == "" && strings.TrimSpace(req.ChunkID) == "" {
		return nil, fmt.Errorf("library_file_id or chunk_id is required")
	}
	if req.MaxChars <= 0 {
		req.MaxChars = 20000
	}

	if strings.TrimSpace(req.LibraryFileID) != "" {
		file, err := s.libraryRepo.GetByID(req.LibraryFileID)
		if err != nil {
			return nil, err
		}
		if file == nil {
			return nil, fmt.Errorf("library file not found")
		}
		if !ownerMatches(file.OwnerUserID, req.OwnerUserID) {
			return nil, fmt.Errorf("library file not found")
		}
		resp := &FetchedFile{File: file}
		if req.IncludeFullText {
			chunks, err := s.chunkRepo.ListByLibraryFileID(file.ID)
			if err != nil {
				return nil, err
			}
			resp.FullText = joinChunkContent(chunks, req.MaxChars)
		}
		return resp, nil
	}

	chunks, err := s.chunkRepo.GetByIDs([]string{req.ChunkID})
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("chunk not found")
	}
	chunk := chunks[0]
	resp := &FetchedFile{Chunk: &chunk}
	if chunk.LibraryFileID != nil {
		file, err := s.libraryRepo.GetByID(*chunk.LibraryFileID)
		if err != nil {
			return nil, err
		}
		if file == nil || !ownerMatches(file.OwnerUserID, req.OwnerUserID) {
			return nil, fmt.Errorf("chunk not found")
		}
		resp.File = file
		if req.IncludeFullText {
			fileChunks, err := s.chunkRepo.ListByLibraryFileID(file.ID)
			if err != nil {
				return nil, err
			}
			resp.FullText = joinChunkContent(fileChunks, req.MaxChars)
		}
	} else if req.IncludeFullText {
		resp.FullText = truncate(chunk.Content, req.MaxChars)
	}
	return resp, nil
}

func joinChunkContent(chunks []models.DocumentChunk, maxChars int) string {
	if len(chunks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range chunks {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(c.Content)
		if b.Len() >= maxChars {
			break
		}
	}
	return truncate(b.String(), maxChars)
}

func truncate(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + "..."
}
