package rag

// File overview: provides deterministic administrative snapshots of chromem physical collections.

import "sort"

// CollectionCounts returns a stable snapshot of every physical collection.
func (s *VectorStore) CollectionCounts() map[string]int {
	out := map[string]int{}
	if s == nil || s.db == nil {
		return out
	}
	for name, collection := range s.db.ListCollections() {
		if collection != nil {
			out[name] = collection.Count()
		}
	}
	return out
}

// CollectionNames returns physical collection names in deterministic order.
func (s *VectorStore) CollectionNames() []string {
	counts := s.CollectionCounts()
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
