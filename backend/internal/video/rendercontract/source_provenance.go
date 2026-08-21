package rendercontract

import (
	"fmt"
	"sort"
	"strings"
)

const SourceProvenanceContractV1 = "source-provenance-v1"

// EvaluatedSourceProvenance is the immutable identity and decoded source box
// supplied by a Render Manifest asset. It is intentionally derived only from
// serialized manifest data: FrameState evaluation must not probe files or
// infer source dimensions from the canvas.
type EvaluatedSourceProvenance struct {
	ContractVersion string                  `json:"contract_version"`
	AssetID         string                  `json:"asset_id"`
	ClipIDs         []string                `json:"clip_ids"`
	FileSHA256      string                  `json:"file_sha256"`
	SourceBounds    TimelineV2ContentBounds `json:"source_bounds"`
}

// EvaluateSourceProvenance projects valid visual dimensions from immutable
// Render Manifest v1 media probes. Assets without visual dimensions simply do
// not produce source provenance; a partial or invalid visual probe fails
// closed rather than fabricating a source box.
func EvaluateSourceProvenance(manifest RenderManifestV1) ([]EvaluatedSourceProvenance, error) {
	seenAssetIDs := make(map[string]struct{}, len(manifest.Assets))
	result := make([]EvaluatedSourceProvenance, 0, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		assetID := asset.AssetID
		if strings.TrimSpace(assetID) == "" {
			return nil, fmt.Errorf("source provenance asset id is required")
		}
		if assetID != strings.TrimSpace(assetID) {
			return nil, fmt.Errorf("source provenance asset id %q must not have surrounding whitespace", assetID)
		}
		if _, exists := seenAssetIDs[assetID]; exists {
			return nil, fmt.Errorf("source provenance has duplicate asset %q", assetID)
		}
		seenAssetIDs[assetID] = struct{}{}
		if !isLowerSHA256(asset.FileSHA256) {
			return nil, fmt.Errorf("source provenance asset %q has an invalid file_sha256", assetID)
		}
		clipIDs, err := canonicalClipIDs(assetID, asset.ClipIDs)
		if err != nil {
			return nil, err
		}
		if asset.Media == nil || (asset.Media.Width == nil && asset.Media.Height == nil) {
			continue
		}
		if asset.Media.Width == nil || asset.Media.Height == nil {
			return nil, fmt.Errorf("source provenance asset %q must provide both media width and height", assetID)
		}
		if *asset.Media.Width < 1 || *asset.Media.Height < 1 {
			return nil, fmt.Errorf("source provenance asset %q media width and height must be positive", assetID)
		}
		result = append(result, EvaluatedSourceProvenance{
			ContractVersion: SourceProvenanceContractV1,
			AssetID:         assetID,
			ClipIDs:         clipIDs,
			FileSHA256:      asset.FileSHA256,
			SourceBounds: TimelineV2ContentBounds{
				X: 0, Y: 0, Width: float64(*asset.Media.Width), Height: float64(*asset.Media.Height),
			},
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AssetID < result[j].AssetID })
	return result, nil
}

func sourceProvenanceByAsset(manifest RenderManifestV1) (map[string]EvaluatedSourceProvenance, error) {
	provenance, err := EvaluateSourceProvenance(manifest)
	if err != nil {
		return nil, err
	}
	byAsset := make(map[string]EvaluatedSourceProvenance, len(provenance))
	for _, source := range provenance {
		byAsset[source.AssetID] = source
	}
	return byAsset, nil
}

func canonicalClipIDs(assetID string, clipIDs []string) ([]string, error) {
	seen := make(map[string]struct{}, len(clipIDs))
	result := make([]string, 0, len(clipIDs))
	for _, clipID := range clipIDs {
		if strings.TrimSpace(clipID) == "" {
			return nil, fmt.Errorf("source provenance asset %q has an empty clip id", assetID)
		}
		if clipID != strings.TrimSpace(clipID) {
			return nil, fmt.Errorf("source provenance asset %q clip id %q must not have surrounding whitespace", assetID, clipID)
		}
		if _, exists := seen[clipID]; exists {
			return nil, fmt.Errorf("source provenance asset %q has duplicate clip id %q", assetID, clipID)
		}
		seen[clipID] = struct{}{}
		result = append(result, clipID)
	}
	sort.Strings(result)
	return result, nil
}

func isLowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func sourceProvenanceForClip(clip TimelineV2Clip, byAsset map[string]EvaluatedSourceProvenance) (*EvaluatedSourceProvenance, error) {
	if clip.AssetID == "" || byAsset == nil {
		return nil, nil
	}
	source, ok := byAsset[clip.AssetID]
	if !ok {
		return nil, nil
	}
	for _, clipID := range source.ClipIDs {
		if clipID == clip.ID {
			copy := source
			copy.ClipIDs = append([]string(nil), source.ClipIDs...)
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("source provenance asset %q does not bind clip %q", clip.AssetID, clip.ID)
}
