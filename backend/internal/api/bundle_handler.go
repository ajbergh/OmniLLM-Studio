package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/bundle"
)

const maxImportSize = 500 << 20 // 500 MB

// BundleHandler handles export and import endpoints.
type BundleHandler struct {
	exporter *bundle.Exporter
	importer *bundle.Importer
}

// NewBundleHandler creates a BundleHandler.
func NewBundleHandler(exporter *bundle.Exporter, importer *bundle.Importer) *BundleHandler {
	return &BundleHandler{exporter: exporter, importer: importer}
}

// Export generates and streams a workspace bundle ZIP.
// POST /v1/export
// Body: { "include_attachments": true, "conversation_ids": ["id1", "id2"] }
func (h *BundleHandler) Export(w http.ResponseWriter, r *http.Request) {
	var opts bundle.ExportOptions
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &opts); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	filename := fmt.Sprintf("omnillm-studio-export-%s.zip", time.Now().UTC().Format("20060102-150405"))

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := h.exporter.Export(w, opts); err != nil {
		// Headers already sent if streaming started; log but can't change status
		log.Printf("ERROR: export: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

// Import uploads and imports a workspace bundle.
// POST /v1/import
// Multipart form: file + strategy (skip|overwrite)
func (h *BundleHandler) Import(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	strategy := bundle.ImportStrategy(r.FormValue("strategy"))
	if strategy == "" {
		strategy = bundle.ImportSkip
	}
	if strategy != bundle.ImportSkip && strategy != bundle.ImportOverwrite {
		respondError(w, http.StatusBadRequest, "strategy must be 'skip' or 'overwrite'")
		return
	}

	// Stream upload to temp file instead of holding in memory
	tmpFile, err := os.CreateTemp("", "omnillm-import-*.zip")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	written, err := io.Copy(tmpFile, io.LimitReader(file, maxImportSize))
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}

	// Seek back to start for zip reader
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		respondInternalError(w, err)
		return
	}

	result, err := h.importer.Import(tmpFile, written, strategy)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ValidateImport checks a bundle without importing.
// POST /v1/import/validate
// Multipart form: file
func (h *BundleHandler) ValidateImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	// Stream upload to temp file instead of holding in memory
	tmpFile, err := os.CreateTemp("", "omnillm-validate-*.zip")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	written, err := io.Copy(tmpFile, io.LimitReader(file, maxImportSize))
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		respondInternalError(w, err)
		return
	}

	report, err := h.importer.Validate(tmpFile, written)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, report)
}
