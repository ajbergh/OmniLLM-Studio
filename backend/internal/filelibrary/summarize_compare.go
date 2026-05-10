package filelibrary

import (
	"context"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

func (s *LibraryService) Summarize(ctx context.Context, req SummarizeRequest) (*SummaryResponse, error) {
	if len(req.LibraryFileIDs) == 0 {
		return nil, fmt.Errorf("library_file_ids is required")
	}
	if req.MaxCharsPerFile <= 0 {
		req.MaxCharsPerFile = 50000
	}
	style := strings.TrimSpace(strings.ToLower(req.SummaryStyle))
	if style == "" {
		style = "detailed"
	}

	ctxBlock, citations, err := s.buildFileSetContext(req.OwnerUserID, req.LibraryFileIDs, req.MaxCharsPerFile)
	if err != nil {
		return nil, err
	}

	if s.llmSvc == nil {
		return &SummaryResponse{Summary: extractiveFallbackSummary(citations), Sources: citations}, nil
	}

	system := "You are summarizing user-provided file excerpts. Use only the supplied excerpts and cite sources inline as [F1], [F2], etc."
	userPrompt := fmt.Sprintf("Summary style: %s\nUser query: %s\n\nSummarize the following file set with citations:\n\n%s", style, strings.TrimSpace(req.Query), ctxBlock)
	resp, err := s.llmSvc.ChatComplete(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return nil, err
	}
	return &SummaryResponse{Summary: strings.TrimSpace(resp.Content), Sources: citations}, nil
}

func (s *LibraryService) Compare(ctx context.Context, req CompareRequest) (*CompareResponse, error) {
	if len(req.LibraryFileIDs) < 2 {
		return nil, fmt.Errorf("at least two library_file_ids are required")
	}
	if req.MaxCharsPerFile <= 0 {
		req.MaxCharsPerFile = 50000
	}
	format := strings.TrimSpace(strings.ToLower(req.OutputFormat))
	if format == "" {
		format = "markdown"
	}

	ctxBlock, citations, err := s.buildFileSetContext(req.OwnerUserID, req.LibraryFileIDs, req.MaxCharsPerFile)
	if err != nil {
		return nil, err
	}

	if s.llmSvc == nil {
		fallback := "Comparison unavailable because no LLM provider is configured.\n\nProvide at least one enabled provider and retry."
		return &CompareResponse{Comparison: fallback, Sources: citations}, nil
	}

	system := "You are comparing user-provided file excerpts. Use only supplied excerpts and cite sources inline as [F1], [F2], etc."
	userPrompt := fmt.Sprintf("Comparison goal: %s\nOutput format: %s\n\nCompare the following file set. Include overlaps, differences, contradictions, and notable risks with citations:\n\n%s", strings.TrimSpace(req.ComparisonGoal), format, ctxBlock)
	resp, err := s.llmSvc.ChatComplete(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return nil, err
	}
	return &CompareResponse{Comparison: strings.TrimSpace(resp.Content), Sources: citations}, nil
}

func (s *LibraryService) buildFileSetContext(ownerUserID string, libraryFileIDs []string, maxCharsPerFile int) (string, []FileCitation, error) {
	var b strings.Builder
	citations := make([]FileCitation, 0, len(libraryFileIDs))
	for i, fileID := range libraryFileIDs {
		file, err := s.libraryRepo.GetByID(fileID)
		if err != nil {
			return "", nil, err
		}
		if file == nil {
			return "", nil, fmt.Errorf("library file not found: %s", fileID)
		}
		if !ownerMatches(file.OwnerUserID, ownerUserID) {
			return "", nil, fmt.Errorf("library file not found: %s", fileID)
		}

		chunks, err := s.chunkRepo.ListByLibraryFileID(fileID)
		if err != nil {
			return "", nil, err
		}
		text := joinChunkContent(chunks, maxCharsPerFile)
		label := fmt.Sprintf("F%d", i+1)
		b.WriteString(fmt.Sprintf("[%s] File: %s\n", label, file.DisplayName))
		b.WriteString("Excerpt:\n")
		b.WriteString(text)
		b.WriteString("\n\n")

		citation := FileCitation{Label: label, FileID: file.ID}
		if len(chunks) > 0 {
			citation.ChunkID = chunks[0].ID
			citation.PageNumber = chunks[0].PageNumber
			citation.SectionTitle = derefString(chunks[0].SectionTitle)
		}
		citations = append(citations, citation)
	}
	return b.String(), citations, nil
}

func extractiveFallbackSummary(citations []FileCitation) string {
	if len(citations) == 0 {
		return "No file excerpts were available to summarize."
	}
	var b strings.Builder
	b.WriteString("Summary (extractive fallback):\n")
	for _, c := range citations {
		b.WriteString(fmt.Sprintf("- Source [%s] included in this summary.\n", c.Label))
	}
	return b.String()
}
