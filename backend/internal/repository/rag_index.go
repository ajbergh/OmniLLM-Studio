package repository

// File overview: creates and persists RAG v2 embedding-space, generation, job, and telemetry state.

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

const ragV2Schema = `
CREATE TABLE IF NOT EXISTS rag_embedding_spaces (
    id TEXT PRIMARY KEY,
    provider_profile_id TEXT NOT NULL DEFAULT '',
    provider_type TEXT NOT NULL,
    model TEXT NOT NULL,
    dimensions INTEGER NOT NULL DEFAULT 0,
    distance_metric TEXT NOT NULL DEFAULT 'cosine',
    normalize INTEGER NOT NULL DEFAULT 1,
    document_task_type TEXT NOT NULL DEFAULT '',
    query_task_type TEXT NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS rag_indexes (
    id TEXT PRIMARY KEY,
    logical_name TEXT NOT NULL UNIQUE,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL,
    active_generation_id TEXT,
    backend TEXT NOT NULL DEFAULT 'chromem',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS rag_index_generations (
    id TEXT PRIMARY KEY,
    index_id TEXT NOT NULL,
    embedding_space_id TEXT NOT NULL,
    parser_version INTEGER NOT NULL DEFAULT 1,
    chunker_version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL CHECK(status IN (
        'queued','extracting','parsing','chunking','embedding','indexing',
        'validating','ready','active','superseded','failed'
    )),
    expected_chunks INTEGER NOT NULL DEFAULT 0,
    indexed_chunks INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    activated_at DATETIME,
    FOREIGN KEY (index_id) REFERENCES rag_indexes(id) ON DELETE CASCADE,
    FOREIGN KEY (embedding_space_id) REFERENCES rag_embedding_spaces(id)
);

CREATE TABLE IF NOT EXISTS rag_ingest_jobs (
    id TEXT PRIMARY KEY,
    generation_id TEXT,
    library_file_id TEXT,
    attachment_id TEXT,
    status TEXT NOT NULL CHECK(status IN (
        'queued','extracting','parsing','chunking','embedding','indexing',
        'validating','completed','failed','cancelled'
    )),
    stage TEXT NOT NULL DEFAULT '',
    progress REAL NOT NULL DEFAULT 0,
    attempts INTEGER NOT NULL DEFAULT 0,
    expected_chunks INTEGER NOT NULL DEFAULT 0,
    completed_chunks INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    started_at DATETIME,
    completed_at DATETIME,
    FOREIGN KEY (generation_id) REFERENCES rag_index_generations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rag_retrieval_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL DEFAULT '',
    workspace_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    query_hash TEXT NOT NULL DEFAULT '',
    embedding_space_id TEXT NOT NULL DEFAULT '',
    vector_candidates INTEGER NOT NULL DEFAULT 0,
    keyword_candidates INTEGER NOT NULL DEFAULT 0,
    fused_candidates INTEGER NOT NULL DEFAULT 0,
    selected_candidates INTEGER NOT NULL DEFAULT 0,
    embedding_ms INTEGER NOT NULL DEFAULT 0,
    vector_ms INTEGER NOT NULL DEFAULT 0,
    keyword_ms INTEGER NOT NULL DEFAULT 0,
    fusion_ms INTEGER NOT NULL DEFAULT 0,
    rerank_ms INTEGER NOT NULL DEFAULT 0,
    context_ms INTEGER NOT NULL DEFAULT 0,
    context_tokens INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_rag_indexes_scope ON rag_indexes(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_rag_generations_index_status ON rag_index_generations(index_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rag_jobs_status ON rag_ingest_jobs(status, updated_at);
CREATE INDEX IF NOT EXISTS idx_rag_jobs_generation ON rag_ingest_jobs(generation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_rag_retrieval_created ON rag_retrieval_events(created_at DESC);
`

const ragFTSSchema = `
CREATE VIRTUAL TABLE IF NOT EXISTS document_chunks_fts USING fts5(
    content,
    section_title,
    content='document_chunks',
    content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS document_chunks_fts_ai AFTER INSERT ON document_chunks BEGIN
    INSERT INTO document_chunks_fts(rowid, content, section_title)
    VALUES (new.rowid, new.content, COALESCE(new.section_title, ''));
END;

CREATE TRIGGER IF NOT EXISTS document_chunks_fts_ad AFTER DELETE ON document_chunks BEGIN
    INSERT INTO document_chunks_fts(document_chunks_fts, rowid, content, section_title)
    VALUES ('delete', old.rowid, old.content, COALESCE(old.section_title, ''));
END;

CREATE TRIGGER IF NOT EXISTS document_chunks_fts_au AFTER UPDATE OF content, section_title ON document_chunks BEGIN
    INSERT INTO document_chunks_fts(document_chunks_fts, rowid, content, section_title)
    VALUES ('delete', old.rowid, old.content, COALESCE(old.section_title, ''));
    INSERT INTO document_chunks_fts(rowid, content, section_title)
    VALUES (new.rowid, new.content, COALESCE(new.section_title, ''));
END;
`

func ensureRAGV2Schema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if _, err := db.Exec(ragV2Schema); err != nil {
		return fmt.Errorf("create rag v2 schema: %w", err)
	}
	if _, err := db.Exec(ragFTSSchema); err != nil {
		// The application remains operational on an SQLite build without FTS5;
		// lexical search transparently falls back to bounded LIKE queries.
		log.Printf("[rag] FTS5 unavailable, using lexical fallback: %v", err)
		return nil
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM document_chunks_fts`).Scan(&count); err == nil && count == 0 {
		if _, err := db.Exec(`INSERT INTO document_chunks_fts(document_chunks_fts) VALUES ('rebuild')`); err != nil {
			log.Printf("[rag] FTS5 initial rebuild failed: %v", err)
		}
	}
	return nil
}

// RAGIndexRepo persists embedding spaces, generations, jobs, and atomic active
// generation pointers.
type RAGIndexRepo struct {
	db *sql.DB
}

// NewRAGIndexRepo creates a generation metadata repository after ensuring its schema.
func NewRAGIndexRepo(db *sql.DB) (*RAGIndexRepo, error) {
	if err := ensureRAGV2Schema(db); err != nil {
		return nil, err
	}
	return &RAGIndexRepo{db: db}, nil
}

// RAGEmbeddingSpaceRecord is the persisted representation of one immutable embedding space.
type RAGEmbeddingSpaceRecord struct {
	ID                string
	ProviderProfileID string
	ProviderType      string
	Model             string
	Dimensions        int
	DistanceMetric    string
	Normalize         bool
	DocumentTaskType  string
	QueryTaskType     string
	SchemaVersion     int
	Fingerprint       string
}

// UpsertEmbeddingSpace creates or refreshes an embedding-space record keyed by fingerprint.
func (r *RAGIndexRepo) UpsertEmbeddingSpace(space RAGEmbeddingSpaceRecord) (string, error) {
	if strings.TrimSpace(space.Fingerprint) == "" {
		return "", fmt.Errorf("embedding-space fingerprint is required")
	}
	if space.ID == "" {
		space.ID = "emb_" + uuid.NewString()
	}
	_, err := r.db.Exec(`
        INSERT INTO rag_embedding_spaces (
            id, provider_profile_id, provider_type, model, dimensions,
            distance_metric, normalize, document_task_type, query_task_type,
            schema_version, fingerprint
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(fingerprint) DO UPDATE SET
            provider_profile_id=excluded.provider_profile_id,
            dimensions=CASE WHEN excluded.dimensions > 0 THEN excluded.dimensions ELSE rag_embedding_spaces.dimensions END`,
		space.ID,
		space.ProviderProfileID,
		space.ProviderType,
		space.Model,
		space.Dimensions,
		space.DistanceMetric,
		boolInt(space.Normalize),
		space.DocumentTaskType,
		space.QueryTaskType,
		space.SchemaVersion,
		space.Fingerprint,
	)
	if err != nil {
		return "", fmt.Errorf("upsert embedding space: %w", err)
	}
	var id string
	if err := r.db.QueryRow(`SELECT id FROM rag_embedding_spaces WHERE fingerprint = ?`, space.Fingerprint).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// EnsureIndex creates or updates the logical index for a scope and returns its stable ID.
func (r *RAGIndexRepo) EnsureIndex(logicalName, scopeType, scopeID, backend string) (string, error) {
	if logicalName == "" || scopeType == "" || scopeID == "" {
		return "", fmt.Errorf("logical name, scope type, and scope id are required")
	}
	if backend == "" {
		backend = "chromem"
	}
	id := "idx_" + uuid.NewString()
	_, err := r.db.Exec(`
        INSERT INTO rag_indexes (id, logical_name, scope_type, scope_id, backend)
        VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(logical_name) DO UPDATE SET
            scope_type=excluded.scope_type,
            scope_id=excluded.scope_id,
            backend=excluded.backend,
            updated_at=datetime('now')`,
		id, logicalName, scopeType, scopeID, backend,
	)
	if err != nil {
		return "", fmt.Errorf("ensure rag index: %w", err)
	}
	if err := r.db.QueryRow(`SELECT id FROM rag_indexes WHERE logical_name = ?`, logicalName).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// BeginGeneration creates a queued immutable generation with its expected chunk count.
func (r *RAGIndexRepo) BeginGeneration(indexID, embeddingSpaceID string, parserVersion, chunkerVersion, expectedChunks int) (string, error) {
	if indexID == "" || embeddingSpaceID == "" {
		return "", fmt.Errorf("index id and embedding space id are required")
	}
	id := "gen_" + uuid.NewString()
	_, err := r.db.Exec(`
        INSERT INTO rag_index_generations (
            id, index_id, embedding_space_id, parser_version, chunker_version,
            status, expected_chunks
        ) VALUES (?, ?, ?, ?, ?, 'queued', ?)`,
		id, indexID, embeddingSpaceID, parserVersion, chunkerVersion, expectedChunks,
	)
	if err != nil {
		return "", fmt.Errorf("begin rag generation: %w", err)
	}
	return id, nil
}

// UpdateGeneration records a valid lifecycle state, indexed count, and optional error.
func (r *RAGIndexRepo) UpdateGeneration(id, status string, indexedChunks int, errorMessage *string) error {
	if !validGenerationStatus(status) {
		return fmt.Errorf("invalid generation status %q", status)
	}
	_, err := r.db.Exec(`
        UPDATE rag_index_generations
        SET status=?, indexed_chunks=?, error_message=?, updated_at=datetime('now')
        WHERE id=?`, status, indexedChunks, errorMessage, id)
	return err
}

// ActivateGeneration atomically switches the active pointer and supersedes the
// previous generation only after the replacement has reached ready state.
func (r *RAGIndexRepo) ActivateGeneration(indexID, generationID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	var expected, indexed int
	if err := tx.QueryRow(`
        SELECT status, expected_chunks, indexed_chunks
        FROM rag_index_generations WHERE id=? AND index_id=?`, generationID, indexID).Scan(&status, &expected, &indexed); err != nil {
		return err
	}
	if status != "ready" {
		return fmt.Errorf("generation %s is %s, not ready", generationID, status)
	}
	if expected > 0 && indexed != expected {
		return fmt.Errorf("generation %s indexed %d of %d chunks", generationID, indexed, expected)
	}

	var previous sql.NullString
	if err := tx.QueryRow(`SELECT active_generation_id FROM rag_indexes WHERE id=?`, indexID).Scan(&previous); err != nil {
		return err
	}
	if previous.Valid && previous.String != "" && previous.String != generationID {
		if _, err := tx.Exec(`
            UPDATE rag_index_generations
            SET status='superseded', updated_at=datetime('now')
            WHERE id=?`, previous.String); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
        UPDATE rag_index_generations
        SET status='active', activated_at=datetime('now'), updated_at=datetime('now')
        WHERE id=?`, generationID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
        UPDATE rag_indexes
        SET active_generation_id=?, updated_at=datetime('now')
        WHERE id=?`, generationID, indexID); err != nil {
		return err
	}
	return tx.Commit()
}

// CreateIngestJob creates a queued ingestion job linked to optional generation and source IDs.
func (r *RAGIndexRepo) CreateIngestJob(generationID, libraryFileID, attachmentID string) (string, error) {
	id := "job_" + uuid.NewString()
	_, err := r.db.Exec(`
        INSERT INTO rag_ingest_jobs (
            id, generation_id, library_file_id, attachment_id, status, stage
        ) VALUES (?, ?, ?, ?, 'queued', 'queued')`,
		id, ragNullableString(generationID), ragNullableString(libraryFileID), ragNullableString(attachmentID),
	)
	return id, err
}

// UpdateIngestJob clamps progress and records stage, completion count, timestamps, and errors.
func (r *RAGIndexRepo) UpdateIngestJob(id, status, stage string, progress float64, completedChunks int, errorMessage *string) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	now := time.Now().UTC()
	_, err := r.db.Exec(`
        UPDATE rag_ingest_jobs
        SET status=?, stage=?, progress=?, completed_chunks=?, error_message=?,
            started_at=COALESCE(started_at, ?),
            completed_at=CASE WHEN ? IN ('completed','failed','cancelled') THEN ? ELSE completed_at END,
            updated_at=?
        WHERE id=?`,
		status, stage, progress, completedChunks, errorMessage,
		now, status, now, now, id,
	)
	return err
}

func validGenerationStatus(status string) bool {
	switch status {
	case "queued", "extracting", "parsing", "chunking", "embedding", "indexing", "validating", "ready", "active", "superseded", "failed":
		return true
	default:
		return false
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ragNullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
