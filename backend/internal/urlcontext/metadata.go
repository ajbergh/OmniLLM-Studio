package urlcontext

// BuildMetadata returns a map suitable for storing in message MetadataJSON.
func BuildMetadata(result *ResolveResult) map[string]any {
	if result == nil || !result.Handled {
		return nil
	}

	m := map[string]any{
		"url_context":          true,
		"url_context_used_rag": result.UsedRAG,
	}

	if len(result.Sources) > 0 {
		m["url_context_sources"] = result.Sources
	}

	if len(result.Warnings) > 0 {
		m["url_context_warnings"] = result.Warnings
	}

	return m
}

// MergeMetadata merges URL context metadata into an existing metadata map.
// It does not overwrite existing keys.
func MergeMetadata(existing map[string]any, urlCtx map[string]any) map[string]any {
	if urlCtx == nil {
		return existing
	}
	if existing == nil {
		existing = make(map[string]any)
	}
	for k, v := range urlCtx {
		if _, ok := existing[k]; !ok {
			existing[k] = v
		}
	}
	return existing
}
