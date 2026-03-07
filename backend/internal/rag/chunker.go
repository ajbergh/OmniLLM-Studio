package rag

import (
	"strings"
	"unicode/utf8"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// ChunkOptions configures chunking behavior.
type ChunkOptions struct {
	ChunkSize int // target chunk size in characters
	Overlap   int // overlap between consecutive chunks in characters
}

// DefaultChunkOptions returns sensible defaults.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		ChunkSize: 1000,
		Overlap:   200,
	}
}

// ChunkText splits text into overlapping chunks. Returns DocumentChunk records
// with IDs, offsets, and content populated. AttachmentID and ConversationID
// must be set by the caller.
func ChunkText(text string, attachmentID, conversationID string, opts ChunkOptions) []models.DocumentChunk {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1000
	}
	if opts.Overlap < 0 {
		opts.Overlap = 0
	}
	if opts.Overlap >= opts.ChunkSize {
		opts.Overlap = opts.ChunkSize / 5
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	textLen := utf8.RuneCountInString(text)
	runes := []rune(text)

	var chunks []models.DocumentChunk
	step := opts.ChunkSize - opts.Overlap
	if step <= 0 {
		step = opts.ChunkSize
	}

	index := 0
	for offset := 0; offset < textLen; offset += step {
		end := offset + opts.ChunkSize
		if end > textLen {
			end = textLen
		}

		content := string(runes[offset:end])
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}

		// Estimate token count (~4 chars per token)
		tokenEst := len(content) / 4
		if tokenEst == 0 {
			tokenEst = 1
		}

		chunk := models.DocumentChunk{
			ID:             uuid.New().String(),
			AttachmentID:   attachmentID,
			ConversationID: conversationID,
			ChunkIndex:     index,
			Content:        content,
			CharOffset:     offset,
			CharLength:     end - offset,
			TokenCount:     &tokenEst,
			MetadataJSON:   "{}",
		}
		chunks = append(chunks, chunk)
		index++

		if end >= textLen {
			break
		}
	}

	return chunks
}

// ChunkMarkdown splits markdown text at heading boundaries where possible,
// falling back to character-based chunking for sections longer than chunkSize.
func ChunkMarkdown(text string, attachmentID, conversationID string, opts ChunkOptions) []models.DocumentChunk {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1000
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Split by markdown headings (lines starting with #)
	lines := strings.Split(text, "\n")
	type section struct {
		heading string
		content strings.Builder
		offset  int
	}

	var sections []section
	currentOffset := 0
	var cur *section

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Start new section
			sections = append(sections, section{heading: trimmed, offset: currentOffset})
			cur = &sections[len(sections)-1]
			cur.content.WriteString(line)
			cur.content.WriteString("\n")
		} else {
			if cur == nil {
				// Text before first heading
				sections = append(sections, section{heading: "", offset: currentOffset})
				cur = &sections[len(sections)-1]
			}
			cur.content.WriteString(line)
			cur.content.WriteString("\n")
		}
		currentOffset += len(line) + 1 // +1 for \n
	}

	// Convert sections into chunks
	var chunks []models.DocumentChunk
	index := 0
	for _, sec := range sections {
		content := strings.TrimSpace(sec.content.String())
		if content == "" {
			continue
		}

		// If section fits in one chunk, add it directly
		if utf8.RuneCountInString(content) <= opts.ChunkSize {
			tokenEst := len(content) / 4
			if tokenEst == 0 {
				tokenEst = 1
			}
			chunk := models.DocumentChunk{
				ID:             uuid.New().String(),
				AttachmentID:   attachmentID,
				ConversationID: conversationID,
				ChunkIndex:     index,
				Content:        content,
				CharOffset:     sec.offset,
				CharLength:     utf8.RuneCountInString(content),
				TokenCount:     &tokenEst,
				MetadataJSON:   "{}",
			}
			chunks = append(chunks, chunk)
			index++
		} else {
			// Section too big — sub-chunk with character splitting
			subChunks := ChunkText(content, attachmentID, conversationID, opts)
			for _, sc := range subChunks {
				sc.ChunkIndex = index
				sc.CharOffset += sec.offset
				chunks = append(chunks, sc)
				index++
			}
		}
	}

	return chunks
}

// DetectAndChunk automatically detects content type and chunks accordingly.
func DetectAndChunk(text, mimeType, attachmentID, conversationID string, opts ChunkOptions) []models.DocumentChunk {
	if strings.Contains(mimeType, "markdown") || strings.HasSuffix(mimeType, ".md") || strings.Contains(text[:min(len(text), 200)], "\n#") {
		return ChunkMarkdown(text, attachmentID, conversationID, opts)
	}
	return ChunkText(text, attachmentID, conversationID, opts)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
