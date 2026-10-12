package rag

// Read-only diagnostic of persisted relational chunk IDs vs. physical chromem
// collections. chromem-go v0.7.0 exposes Count and GetByID but does not expose
// document ID enumeration: we can prove expected-ID presence, not orphan absence.

import (
	"context"
	"fmt"
)

const maxIntegrityScanRecords = 100_000

// ChunkIDIntegrity measures the presence of expected relational chunk IDs
// in at least one physical embedding collection, including legacy generations.
// AllExpectedPresent does NOT prove that the current embedding space is ready,
// embeddings are current, or that unreferenced vector IDs are absent.
type ChunkIDIntegrity struct {
	PhysicalCollections    int  `json:"physical_collections"`
	ExpectedChunks         int  `json:"expected_chunks"`
	VectorRecords          int  `json:"vector_records"`
	ChunksMissingAnyVector int  `json:"chunks_missing_any_vector"`
	AllExpectedPresent     bool `json:"all_expected_present"`
	OrphanVectorIDsChecked bool `json:"orphan_vector_ids_checked"`
}

// InspectChunkIDs checks actual IDs using the installed chromem v0.7.0
// GetByID API; counts alone can miss equal-size but inconsistent datasets.
// It does not create a collection, call embeddings, or modify records.
func (s *VectorStore) InspectChunkIDs(ctx context.Context, logicalName string, expectedIDs []string) (ChunkIDIntegrity, error) {
	report := ChunkIDIntegrity{OrphanVectorIDsChecked: false}
	if s == nil || s.db == nil || logicalName == "" {
		return report, fmt.Errorf("index unavailable or logical scope empty")
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if len(expectedIDs) > maxIntegrityScanRecords {
		return report, fmt.Errorf("chunk scope exceeds integrity scan limit of %d", maxIntegrityScanRecords)
	}
	expected := make(map[string]struct{}, len(expectedIDs))
	for _, id := range expectedIDs {
		if id == "" {
			return report, fmt.Errorf("empty relational chunk ID")
		}
		expected[id] = struct{}{}
	}
	report.ExpectedChunks = len(expected)
	collections := s.PhysicalCollections(logicalName)
	report.PhysicalCollections = len(collections)
	for _, name := range collections {
		collection := s.db.GetCollection(name, nil)
		if collection == nil {
			return ChunkIDIntegrity{}, fmt.Errorf("physical collection disappeared during integrity scan")
		}
		count := collection.Count()
		if count > maxIntegrityScanRecords || report.VectorRecords+count > maxIntegrityScanRecords {
			return ChunkIDIntegrity{}, fmt.Errorf("vector scope exceeds integrity scan limit of %d", maxIntegrityScanRecords)
		}
		report.VectorRecords += count
	}
	for id := range expected {
		if err := ctx.Err(); err != nil {
			return ChunkIDIntegrity{}, err
		}
		found := false
		for _, name := range collections {
			collection := s.db.GetCollection(name, nil)
			if collection == nil {
				return ChunkIDIntegrity{}, fmt.Errorf("physical collection disappeared during integrity scan")
			}
			if _, err := collection.GetByID(ctx, id); err == nil {
				found = true
				break
			}
		}
		if !found {
			report.ChunksMissingAnyVector++
		}
	}
	report.AllExpectedPresent = report.ChunksMissingAnyVector == 0
	return report, nil
}
