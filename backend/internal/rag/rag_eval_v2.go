package rag

// File overview: provides retrieval-quality metrics and stage-level telemetry helpers.

import (
	"math"
	"sort"
	"time"
)

// EvaluationCase is one retrieval-quality benchmark query.
type EvaluationCase struct {
	ID                string   `json:"id"`
	Query             string   `json:"query"`
	RelevantChunkIDs  []string `json:"relevant_chunk_ids"`
	RelevantSourceIDs []string `json:"relevant_source_ids,omitempty"`
}

// EvaluationMetrics summarizes retrieval quality over a corpus.
type EvaluationMetrics struct {
	Cases      int     `json:"cases"`
	RecallAtK  float64 `json:"recall_at_k"`
	MRR        float64 `json:"mrr"`
	NDCGAtK    float64 `json:"ndcg_at_k"`
	HitRateAtK float64 `json:"hit_rate_at_k"`
}

// EvaluateRetrieval computes recall@k, reciprocal rank, nDCG@k, and hit rate.
func EvaluateRetrieval(cases []EvaluationCase, results map[string][]string, k int) EvaluationMetrics {
	if k <= 0 {
		k = 10
	}
	metrics := EvaluationMetrics{Cases: len(cases)}
	if len(cases) == 0 {
		return metrics
	}
	for _, testCase := range cases {
		relevant := make(map[string]struct{}, len(testCase.RelevantChunkIDs))
		for _, id := range testCase.RelevantChunkIDs {
			relevant[id] = struct{}{}
		}
		ranked := results[testCase.ID]
		if len(ranked) > k {
			ranked = ranked[:k]
		}
		hits := 0
		firstRank := 0
		dcg := 0.0
		for index, id := range ranked {
			if _, ok := relevant[id]; !ok {
				continue
			}
			hits++
			if firstRank == 0 {
				firstRank = index + 1
			}
			dcg += 1 / math.Log2(float64(index+2))
		}
		if len(relevant) > 0 {
			metrics.RecallAtK += float64(hits) / float64(len(relevant))
		}
		if firstRank > 0 {
			metrics.MRR += 1 / float64(firstRank)
			metrics.HitRateAtK++
		}
		idealHits := len(relevant)
		if idealHits > k {
			idealHits = k
		}
		idcg := 0.0
		for index := 0; index < idealHits; index++ {
			idcg += 1 / math.Log2(float64(index+2))
		}
		if idcg > 0 {
			metrics.NDCGAtK += dcg / idcg
		}
	}
	count := float64(len(cases))
	metrics.RecallAtK /= count
	metrics.MRR /= count
	metrics.NDCGAtK /= count
	metrics.HitRateAtK /= count
	return metrics
}

// CompareEvaluationMetrics returns deterministic metric deltas for release
// reports and CI regression checks.
func CompareEvaluationMetrics(before, after EvaluationMetrics) map[string]float64 {
	return map[string]float64{
		"recall_at_k":   after.RecallAtK - before.RecallAtK,
		"mrr":           after.MRR - before.MRR,
		"ndcg_at_k":     after.NDCGAtK - before.NDCGAtK,
		"hit_rate_at_k": after.HitRateAtK - before.HitRateAtK,
	}
}

// StableRankIDs normalizes arbitrary scored candidates into deterministic IDs
// for evaluation snapshots.
func StableRankIDs(candidates []RankedCandidate, k int) []string {
	items := append([]RankedCandidate(nil), candidates...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].ID < items[j].ID
		}
		return items[i].Score > items[j].Score
	})
	if k > 0 && len(items) > k {
		items = items[:k]
	}
	ids := make([]string, len(items))
	for index := range items {
		ids[index] = items[index].ID
	}
	return ids
}

// RetrievalTelemetry captures stage-level RAG latency and candidate counts.
type RetrievalTelemetry struct {
	StartedAt time.Time `json:"started_at"`

	EmbeddingDuration time.Duration `json:"embedding_duration"`
	VectorDuration    time.Duration `json:"vector_duration"`
	KeywordDuration   time.Duration `json:"keyword_duration"`
	FusionDuration    time.Duration `json:"fusion_duration"`
	RerankDuration    time.Duration `json:"rerank_duration"`
	ContextDuration   time.Duration `json:"context_duration"`

	VectorCandidates   int `json:"vector_candidates"`
	KeywordCandidates  int `json:"keyword_candidates"`
	FusedCandidates    int `json:"fused_candidates"`
	SelectedCandidates int `json:"selected_candidates"`
	ContextTokens      int `json:"context_tokens"`
}

// NewRetrievalTelemetry starts a UTC-timestamped telemetry measurement.
func NewRetrievalTelemetry() *RetrievalTelemetry {
	return &RetrievalTelemetry{StartedAt: time.Now().UTC()}
}

// TotalDuration returns elapsed wall-clock time since telemetry creation, or zero for an uninitialized value.
func (t *RetrievalTelemetry) TotalDuration() time.Duration {
	if t == nil || t.StartedAt.IsZero() {
		return 0
	}
	return time.Since(t.StartedAt)
}
