package filelibrary

// File overview: contains File Library extraction compatibility helpers and hybrid retrieval.

import (
	"context"
	"fmt"
	"github.com/ajbergh/omnillm-studio/internal/document"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
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

func isTextMIME(mime string) bool {
	return document.IsTextMIME(mime)
}

func extractFileText(path, mime string) (string, error) {
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

// Search performs hybrid retrieval using SQLite FTS5/BM25, vector search,
// reciprocal-rank fusion, metadata boosts, and source-diverse selection.
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
	candidateLimit := topK * 6
	if candidateLimit < 30 {
		candidateLimit = 30
	}

	scopes := expandScopes(req.Scope)
	// Search all accessible files in the requested scopes. Metadata matches are
	// a ranking signal, never a prefilter that can hide semantically relevant
	// files with unrelated filenames.
	files, err := s.ListFiles(ctx, req.OwnerUserID, req.Scope, "")
	if err != nil {
		return nil, err
	}
	filteredFiles := make([]models.LibraryFile, 0, len(files))
	for _, file := range files {
		if file.Status != "indexed" && file.Status != "embedding" {
			continue
		}
		if fileMatchesFilters(file, req) {
			filteredFiles = append(filteredFiles, file)
		}
	}
	files = filteredFiles

	fileByID := make(map[string]mapFile, len(files))
	fileIDs := make([]string, 0, len(files))
	for _, file := range files {
		fileByID[file.ID] = mapFileFromModel(file)
		fileIDs = append(fileIDs, file.ID)
	}
	if len(fileIDs) == 0 {
		return &SearchResponse{
			Query: query,
			Scope: strings.TrimSpace(req.Scope),
			Metadata: SearchMetadata{
				SearchedCollections: collectionsForScopes(scopes, req.OwnerUserID, req.WorkspaceID, req.ConversationID),
			},
		}, nil
	}

	chunkByID := make(map[string]models.DocumentChunk)
	keywordHits, err := s.chunkRepo.SearchFTSByLibraryFileIDs(fileIDs, query, candidateLimit)
	if err != nil {
		return nil, err
	}
	keywordItems := make([]rag.RankedCandidate, 0, len(keywordHits))
	for _, hit := range keywordHits {
		chunk := hit.Chunk
		if chunk.LibraryFileID == nil {
			continue
		}
		if _, allowed := fileByID[*chunk.LibraryFileID]; !allowed {
			continue
		}
		chunkByID[chunk.ID] = chunk
		keywordItems = append(keywordItems, rag.RankedCandidate{
			ID:       chunk.ID,
			Score:    hit.Score,
			SourceID: *chunk.LibraryFileID,
		})
	}

	vectorSimilarity := make(map[string]float64)
	vectorCount := 0
	if s.vectorStore != nil && s.llmSvc != nil && s.settingsRepo != nil {
		settings, settingsErr := s.settingsRepo.GetTyped()
		if settingsErr == nil && settings.RAGEnabled {
			embedProvider, embedModel, embedErr := s.resolveEmbeddingProvider(req.ConversationID, settings)
			if embedErr == nil {
				embedFunc := rag.NewLLMEmbeddingFunc(s.llmSvc, embedProvider, embedModel)
				allVectorIDs := make([]string, 0)
				for _, scope := range scopes {
					collection := CollectionName(scope, req.OwnerUserID, req.WorkspaceID, req.ConversationID)
					hits, hitErr := s.vectorStore.QuerySimilar(ctx, collection, query, candidateLimit, embedFunc)
					if hitErr != nil {
						continue
					}
					for _, hit := range hits {
						if existing, ok := vectorSimilarity[hit.ID]; !ok || hit.Similarity > existing {
							vectorSimilarity[hit.ID] = hit.Similarity
						}
						allVectorIDs = append(allVectorIDs, hit.ID)
					}
				}
				if len(allVectorIDs) > 0 {
					vectorChunks, loadErr := s.chunkRepo.GetByIDs(uniqueStrings(allVectorIDs))
					if loadErr == nil {
						for _, chunk := range vectorChunks {
							if chunk.LibraryFileID == nil {
								continue
							}
							if _, allowed := fileByID[*chunk.LibraryFileID]; !allowed {
								continue
							}
							chunkByID[chunk.ID] = chunk
							vectorCount++
						}
					}
				}
			}
		}
	}

	vectorItems := make([]rag.RankedCandidate, 0, len(vectorSimilarity))
	for chunkID, similarity := range vectorSimilarity {
		chunk, ok := chunkByID[chunkID]
		if !ok || chunk.LibraryFileID == nil {
			continue
		}
		vectorItems = append(vectorItems, rag.RankedCandidate{
			ID:       chunkID,
			Score:    similarity,
			SourceID: *chunk.LibraryFileID,
		})
	}
	sort.SliceStable(vectorItems, func(i, j int) bool {
		if vectorItems[i].Score == vectorItems[j].Score {
			return vectorItems[i].ID < vectorItems[j].ID
		}
		return vectorItems[i].Score > vectorItems[j].Score
	})

	fused := rag.ReciprocalRankFusion([]rag.RankedList{
		{Name: "vector", Weight: 1.0, Items: vectorItems},
		{Name: "bm25", Weight: 1.0, Items: keywordItems},
	}, 60)

	// Apply modest, bounded metadata signals after RRF. These improve ties without
	// overwhelming semantic or lexical relevance.
	queryTerms := normalizedQueryTerms(query)
	for index := range fused {
		chunk, ok := chunkByID[fused[index].ID]
		if !ok || chunk.LibraryFileID == nil {
			continue
		}
		file := fileByID[*chunk.LibraryFileID]
		fused[index].SourceID = file.ID
		fused[index].Score += 0.002 * metadataMatchScore(file, queryTerms)
		fused[index].Score += 0.0005 * scopePreference(file.Scope)
		fused[index].Score += 0.0002 * recencyBoostValue(file.UpdatedAt)
	}
	sort.SliceStable(fused, func(i, j int) bool {
		if fused[i].Score == fused[j].Score {
			return fused[i].ID < fused[j].ID
		}
		return fused[i].Score > fused[j].Score
	})
	fused = rag.MMRSelect(fused, topK, 0.82)

	results := make([]FileSearchResult, 0, len(fused))
	for _, candidate := range fused {
		chunk, ok := chunkByID[candidate.ID]
		if !ok || chunk.LibraryFileID == nil {
			continue
		}
		file, ok := fileByID[*chunk.LibraryFileID]
		if !ok {
			continue
		}
		snippet := buildSnippet(chunk.Content, query, 650)
		citation := FileCitation{
			Label:        buildCitationLabel(file.DisplayName, chunk.PageNumber, chunk.SectionTitle),
			FileID:       file.ID,
			ChunkID:      chunk.ID,
			PageNumber:   chunk.PageNumber,
			SectionTitle: derefString(chunk.SectionTitle),
		}
		results = append(results, FileSearchResult{
			ChunkID:       chunk.ID,
			LibraryFileID: file.ID,
			FileName:      file.FileName,
			DisplayName:   file.DisplayName,
			MimeType:      file.MimeType,
			Scope:         file.Scope,
			SourceType:    file.SourceType,
			SourceURL:     file.SourceURL,
			PageNumber:    chunk.PageNumber,
			SectionTitle:  derefString(chunk.SectionTitle),
			Snippet:       snippet,
			Score:         candidate.Score,
			Citation:      citation,
		})
	}

	evidence := make([]rag.Evidence, 0, len(results))
	for index, result := range results {
		evidence = append(evidence, rag.Evidence{
			ID: result.ChunkID, SourceType: result.Scope + "_file", SourceID: result.LibraryFileID,
			DisplayName: result.DisplayName, Text: result.Snippet, Score: result.Score,
			Citation: fmt.Sprintf("F%d", index+1),
		})
	}
	rag.RegisterRequestEvidence(ctx, evidence...)

	return &SearchResponse{
		Query:   query,
		Scope:   strings.TrimSpace(req.Scope),
		Results: results,
		Metadata: SearchMetadata{
			SearchedCollections: collectionsForScopes(scopes, req.OwnerUserID, req.WorkspaceID, req.ConversationID),
			VectorResults:       vectorCount,
			KeywordResults:      len(keywordHits),
			MergedResults:       len(results),
		},
	}, nil
}

func fileMatchesFilters(file models.LibraryFile, req SearchRequest) bool {
	if len(req.FileTypeFilter) > 0 {
		fileType := strings.ToLower(strings.TrimSpace(derefOr(file.FileExt, "")))
		mimeType := strings.ToLower(strings.TrimSpace(derefOr(file.MimeType, "")))
		matched := false
		for _, filter := range req.FileTypeFilter {
			needle := strings.ToLower(strings.TrimSpace(filter))
			if needle != "" && (fileType == needle || strings.Contains(mimeType, needle)) {
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
		for _, filter := range req.SourceFilter {
			if strings.EqualFold(strings.TrimSpace(filter), strings.TrimSpace(file.SourceType)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if req.TimeFilter != nil && !timeInRange(file.UpdatedAt, req.TimeFilter.StartDate, req.TimeFilter.EndDate) {
		return false
	}
	return true
}

func scopePreference(scope string) float64 {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "conversation":
		return 1
	case "workspace":
		return 0.65
	case "global":
		return 0.35
	default:
		return 0
	}
}

func recencyBoostValue(updatedAt time.Time) float64 {
	if updatedAt.IsZero() {
		return 0
	}
	days := time.Since(updatedAt).Hours() / 24
	if days < 0 {
		days = 0
	}
	if days >= 365 {
		return 0
	}
	return 1 - (days / 365)
}

func timeInRange(value time.Time, startDate, endDate string) bool {
	if value.IsZero() {
		return false
	}
	start := parseDateOrZero(startDate)
	end := parseDateOrZero(endDate)
	if !start.IsZero() && value.Before(start) {
		return false
	}
	if !end.IsZero() && value.After(end.Add(24*time.Hour)) {
		return false
	}
	return true
}

func parseDateOrZero(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, format := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func collectionsForScopes(scopes []string, ownerUserID, workspaceID, conversationID string) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, CollectionName(scope, ownerUserID, workspaceID, conversationID))
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
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	lower := strings.ToLower(text)
	queryTerms := normalizedQueryTerms(query)
	byteIndex := -1
	for _, term := range queryTerms {
		if index := strings.Index(lower, term); index >= 0 && (byteIndex < 0 || index < byteIndex) {
			byteIndex = index
		}
	}
	startRune := 0
	if byteIndex > 0 {
		startRune = utf8.RuneCountInString(text[:byteIndex]) - maxLen/3
		if startRune < 0 {
			startRune = 0
		}
	}
	endRune := startRune + maxLen
	if endRune > len(runes) {
		endRune = len(runes)
		startRune = endRune - maxLen
		if startRune < 0 {
			startRune = 0
		}
	}
	snippet := string(runes[startRune:endRune])
	if startRune > 0 {
		snippet = "..." + snippet
	}
	if endRune < len(runes) {
		snippet += "..."
	}
	return snippet
}

func buildCitationLabel(displayName string, pageNumber *int, sectionTitle *string) string {
	if pageNumber != nil {
		return fmt.Sprintf("%s, p. %d", displayName, *pageNumber)
	}
	if sectionTitle != nil && strings.TrimSpace(*sectionTitle) != "" {
		return fmt.Sprintf("%s, section %s", displayName, strings.TrimSpace(*sectionTitle))
	}
	return displayName
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

func mapFileFromModel(file models.LibraryFile) mapFile {
	return mapFile{
		ID:          file.ID,
		FileName:    derefOr(file.OriginalFilename, file.DisplayName),
		DisplayName: file.DisplayName,
		MimeType:    derefOr(file.MimeType, "application/octet-stream"),
		Scope:       file.Scope,
		SourceType:  file.SourceType,
		SourceURL:   derefOr(file.SourceURL, ""),
		UpdatedAt:   file.UpdatedAt,
	}
}

func normalizedQueryTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	seen := make(map[string]struct{}, len(fields))
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, " \t\r\n.,;:!?()[]{}\"'")
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		terms = append(terms, field)
	}
	return terms
}

func metadataMatchScore(file mapFile, terms []string) float64 {
	if len(terms) == 0 {
		return 0
	}
	haystack := strings.ToLower(file.DisplayName + " " + file.FileName + " " + file.SourceType)
	matches := 0
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			matches++
		}
	}
	return float64(matches) / float64(len(terms))
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefOr(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}
