package filelibrary

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
)

// Search performs hybrid retrieval by combining vector and keyword results.
func (s *LibraryService) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 8
	}
	if topK > 30 {
		topK = 30
	}

	scopes := expandScopes(req.Scope)
	files, err := s.libraryRepo.SearchMetadata(req.OwnerUserID, query, scopes)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		files, err = s.ListFiles(ctx, req.OwnerUserID, req.Scope, "")
		if err != nil {
			return nil, err
		}
	}

	filteredFiles := make([]models.LibraryFile, 0, len(files))
	for _, f := range files {
		if fileMatchesFilters(f, req) {
			filteredFiles = append(filteredFiles, f)
		}
	}
	files = filteredFiles

	fileByID := make(map[string]mapFile)
	fileIDs := make([]string, 0, len(files))
	for _, f := range files {
		fileByID[f.ID] = mapFileFromModel(f)
		fileIDs = append(fileIDs, f.ID)
	}

	keywordLimit := topK * 3
	keywordChunks, err := s.chunkRepo.SearchByLibraryFileIDs(fileIDs, query, keywordLimit)
	if err != nil {
		return nil, err
	}
	keywordRank := make(map[string]int)
	chunkByID := make(map[string]models.DocumentChunk)
	for i, c := range keywordChunks {
		keywordRank[c.ID] = i
		chunkByID[c.ID] = c
	}

	vectorRank := make(map[string]int)
	vectorSimilarity := make(map[string]float64)
	vectorCount := 0
	if s.vectorStore != nil && s.llmSvc != nil && s.settingsRepo != nil {
		settings, settingsErr := s.settingsRepo.GetTyped()
		if settingsErr == nil && settings.RAGEnabled {
			embedProvider, embedModel, embedErr := s.resolveEmbeddingProvider(req.ConversationID, settings)
			if embedErr == nil {
				embedFunc := rag.NewLLMEmbeddingFunc(s.llmSvc, embedProvider, embedModel)
				vectorLimit := topK * 3
				allVectorIDs := make([]string, 0)
				for _, sc := range scopes {
					collection := CollectionName(sc, req.OwnerUserID, req.WorkspaceID, req.ConversationID)
					hits, hitErr := s.vectorStore.QuerySimilar(ctx, collection, query, vectorLimit, embedFunc)
					if hitErr != nil {
						continue
					}
					for i, h := range hits {
						if existing, ok := vectorSimilarity[h.ID]; !ok || h.Similarity > existing {
							vectorSimilarity[h.ID] = h.Similarity
							vectorRank[h.ID] = i
						}
						allVectorIDs = append(allVectorIDs, h.ID)
					}
				}
				if len(allVectorIDs) > 0 {
					vectorChunks, loadErr := s.chunkRepo.GetByIDs(uniqueStrings(allVectorIDs))
					if loadErr == nil {
						for _, c := range vectorChunks {
							if c.LibraryFileID == nil {
								continue
							}
							if _, ok := fileByID[*c.LibraryFileID]; !ok {
								continue
							}
							chunkByID[c.ID] = c
						}
						vectorCount = len(vectorChunks)
					}
				}
			}
		}
	}

	type scored struct {
		chunk models.DocumentChunk
		score float64
	}
	scoredChunks := make([]scored, 0, len(chunkByID))
	for chunkID, c := range chunkByID {
		if c.LibraryFileID == nil {
			continue
		}
		mf, ok := fileByID[*c.LibraryFileID]
		if !ok {
			continue
		}

		vectorScore := normalizeVectorScore(vectorSimilarity[chunkID], vectorRank, chunkID)
		keywordScore := normalizeRankScore(keywordRank, chunkID, len(keywordChunks))
		scopeBoost := scopeBoostValue(mf.Scope)
		recencyBoost := recencyBoostValue(mf.UpdatedAt)
		finalScore := (0.70 * vectorScore) + (0.25 * keywordScore) + (0.03 * scopeBoost) + (0.02 * recencyBoost)
		scoredChunks = append(scoredChunks, scored{chunk: c, score: finalScore})
	}

	sort.SliceStable(scoredChunks, func(i, j int) bool {
		return scoredChunks[i].score > scoredChunks[j].score
	})
	if len(scoredChunks) > topK {
		scoredChunks = scoredChunks[:topK]
	}

	results := make([]FileSearchResult, 0, len(scoredChunks))
	for _, item := range scoredChunks {
		c := item.chunk
		mf := fileByID[*c.LibraryFileID]
		snippet := buildSnippet(c.Content, query, 500)
		citation := FileCitation{
			Label:        buildCitationLabel(mf.DisplayName, c.PageNumber, c.SectionTitle),
			FileID:       mf.ID,
			ChunkID:      c.ID,
			PageNumber:   c.PageNumber,
			SectionTitle: derefString(c.SectionTitle),
		}
		results = append(results, FileSearchResult{
			ChunkID:       c.ID,
			LibraryFileID: mf.ID,
			FileName:      mf.FileName,
			DisplayName:   mf.DisplayName,
			MimeType:      mf.MimeType,
			Scope:         mf.Scope,
			SourceType:    mf.SourceType,
			SourceURL:     mf.SourceURL,
			PageNumber:    c.PageNumber,
			SectionTitle:  derefString(c.SectionTitle),
			Snippet:       snippet,
			Score:         item.score,
			Citation:      citation,
		})
	}

	resp := &SearchResponse{
		Query:   query,
		Scope:   strings.TrimSpace(req.Scope),
		Results: results,
		Metadata: SearchMetadata{
			SearchedCollections: collectionsForScopes(scopes, req.OwnerUserID, req.WorkspaceID, req.ConversationID),
			VectorResults:       vectorCount,
			KeywordResults:      len(keywordChunks),
			MergedResults:       len(results),
		},
	}
	return resp, nil
}

func fileMatchesFilters(f models.LibraryFile, req SearchRequest) bool {
	if len(req.FileTypeFilter) > 0 {
		fileType := strings.ToLower(strings.TrimSpace(derefOr(f.FileExt, "")))
		mimeType := strings.ToLower(strings.TrimSpace(derefOr(f.MimeType, "")))
		matched := false
		for _, ft := range req.FileTypeFilter {
			needle := strings.ToLower(strings.TrimSpace(ft))
			if needle == "" {
				continue
			}
			if fileType == needle || strings.Contains(mimeType, needle) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(req.SourceFilter) > 0 {
		matched := false
		for _, sf := range req.SourceFilter {
			if strings.EqualFold(strings.TrimSpace(sf), strings.TrimSpace(f.SourceType)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if req.TimeFilter != nil {
		if !timeInRange(f.UpdatedAt, req.TimeFilter.StartDate, req.TimeFilter.EndDate) {
			return false
		}
	}

	return true
}

func normalizeVectorScore(sim float64, ranks map[string]int, chunkID string) float64 {
	if sim > 0 {
		if sim > 1 {
			sim = 1
		}
		if sim < 0 {
			sim = 0
		}
		return sim
	}
	return normalizeRankScore(ranks, chunkID, len(ranks))
}

func normalizeRankScore(ranks map[string]int, chunkID string, total int) float64 {
	if total <= 0 {
		return 0
	}
	rank, ok := ranks[chunkID]
	if !ok {
		return 0
	}
	score := 1.0 - (float64(rank) / float64(total))
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func scopeBoostValue(scope string) float64 {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "conversation":
		return 0.05
	case "workspace":
		return 0.03
	case "global":
		return 0.01
	default:
		return 0
	}
}

func recencyBoostValue(updatedAt time.Time) float64 {
	if updatedAt.IsZero() {
		return 0
	}
	days := time.Since(updatedAt).Hours() / 24.0
	if days < 0 {
		days = 0
	}
	if days >= 365 {
		return 0
	}
	boost := 1.0 - (days / 365.0)
	if boost < 0 {
		boost = 0
	}
	return boost
}

func timeInRange(t time.Time, startDate, endDate string) bool {
	if t.IsZero() {
		return false
	}
	start := parseDateOrZero(startDate)
	end := parseDateOrZero(endDate)
	if !start.IsZero() && t.Before(start) {
		return false
	}
	if !end.IsZero() && t.After(end.Add(24*time.Hour)) {
		return false
	}
	return true
}

func parseDateOrZero(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func collectionsForScopes(scopes []string, ownerUserID, workspaceID, conversationID string) []string {
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		out = append(out, CollectionName(sc, ownerUserID, workspaceID, conversationID))
	}
	return out
}

func buildSnippet(content, query string, maxLen int) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 400
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || len(text) <= maxLen {
		if len(text) > maxLen {
			return text[:maxLen] + "..."
		}
		return text
	}

	lower := strings.ToLower(text)
	idx := strings.Index(lower, q)
	if idx < 0 {
		if len(text) > maxLen {
			return text[:maxLen] + "..."
		}
		return text
	}
	start := idx - (maxLen / 3)
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(text) {
		end = len(text)
	}
	snippet := text[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet += "..."
	}
	return snippet
}

func buildCitationLabel(displayName string, pageNumber *int, sectionTitle *string) string {
	label := displayName
	if pageNumber != nil {
		return fmt.Sprintf("%s, p. %d", displayName, *pageNumber)
	}
	if sectionTitle != nil && strings.TrimSpace(*sectionTitle) != "" {
		label = fmt.Sprintf("%s, section %s", displayName, strings.TrimSpace(*sectionTitle))
	}
	return label
}

type mapFile struct {
	ID          string
	FileName    string
	DisplayName string
	MimeType    string
	Scope       string
	SourceType  string
	SourceURL   string
	UpdatedAt   time.Time
}

func mapFileFromModel(f models.LibraryFile) mapFile {
	return mapFile{
		ID:          f.ID,
		FileName:    derefOr(f.OriginalFilename, f.DisplayName),
		DisplayName: f.DisplayName,
		MimeType:    derefOr(f.MimeType, "application/octet-stream"),
		Scope:       f.Scope,
		SourceType:  f.SourceType,
		SourceURL:   derefOr(f.SourceURL, ""),
		UpdatedAt:   f.UpdatedAt,
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefOr(v *string, fallback string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return fallback
	}
	return *v
}
