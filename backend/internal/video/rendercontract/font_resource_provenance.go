package rendercontract

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

const FontResourceProvenanceContractV1 = "font-resource-provenance-v1"

// EvaluatedFontResourceProvenance is one immutable static font face packaged
// with a Render Manifest snapshot. It is an identity/package contract only:
// text face selection and glyph metrics remain deliberately unassigned until
// an explicit consumer contract can bind them without fallback guessing.
type EvaluatedFontResourceProvenance struct {
	ContractVersion string `json:"contract_version"`
	FontResourceID  string `json:"font_resource_id"`
	FontFamily      string `json:"font_family"`
	FontWeight      int    `json:"font_weight"`
	FontStyle       string `json:"font_style"`
	Format          string `json:"format"`
	StagedPath      string `json:"staged_path"`
	FileSHA256      string `json:"file_sha256"`
	SizeBytes       int64  `json:"size_bytes"`
}

// EvaluateFontResourceProvenance validates and projects immutable packaged
// font faces from Render Manifest v1. It does not inspect a filesystem, load
// a system font, or infer a text-face match from an authored family name.
func EvaluateFontResourceProvenance(manifest RenderManifestV1) ([]EvaluatedFontResourceProvenance, error) {
	seenIDs := make(map[string]struct{}, len(manifest.FontResources))
	result := make([]EvaluatedFontResourceProvenance, 0, len(manifest.FontResources))
	for _, resource := range manifest.FontResources {
		resourceID, err := canonicalFontResourceID(resource.FontResourceID)
		if err != nil {
			return nil, err
		}
		if _, exists := seenIDs[resourceID]; exists {
			return nil, fmt.Errorf("font resource provenance has duplicate font resource %q", resourceID)
		}
		seenIDs[resourceID] = struct{}{}
		fontFamily, err := canonicalFontResourceToken(fmt.Sprintf("font resource %q font family", resourceID), resource.FontFamily)
		if err != nil {
			return nil, err
		}
		if resource.FontWeight < 1 || resource.FontWeight > 1000 {
			return nil, fmt.Errorf("font resource provenance font resource %q font weight must be between 1 and 1000", resourceID)
		}
		if resource.FontStyle != "normal" && resource.FontStyle != "italic" {
			return nil, fmt.Errorf("font resource provenance font resource %q has unsupported font style %q", resourceID, resource.FontStyle)
		}
		if resource.Format != "woff2" && resource.Format != "woff" && resource.Format != "ttf" && resource.Format != "otf" {
			return nil, fmt.Errorf("font resource provenance font resource %q has unsupported format %q", resourceID, resource.Format)
		}
		if err := validateFontResourceStagedPath(resourceID, resource.StagedPath); err != nil {
			return nil, err
		}
		if !isLowerSHA256(resource.FileSHA256) {
			return nil, fmt.Errorf("font resource provenance font resource %q has an invalid file_sha256", resourceID)
		}
		if resource.SizeBytes < 1 {
			return nil, fmt.Errorf("font resource provenance font resource %q size_bytes must be positive", resourceID)
		}
		result = append(result, EvaluatedFontResourceProvenance{
			ContractVersion: FontResourceProvenanceContractV1,
			FontResourceID:  resourceID,
			FontFamily:      fontFamily,
			FontWeight:      resource.FontWeight,
			FontStyle:       resource.FontStyle,
			Format:          resource.Format,
			StagedPath:      resource.StagedPath,
			FileSHA256:      resource.FileSHA256,
			SizeBytes:       resource.SizeBytes,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FontResourceID < result[j].FontResourceID })
	return result, nil
}

func canonicalFontResourceID(value string) (string, error) {
	resourceID, err := canonicalFontResourceToken("font resource id", value)
	if err != nil {
		return "", err
	}
	for index, character := range resourceID {
		isLowerAlpha := character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		if (index == 0 && !isLowerAlpha && !isDigit) || (index > 0 && !isLowerAlpha && !isDigit && character != '.' && character != '_' && character != '-') {
			return "", fmt.Errorf("font resource provenance font resource id %q must use lowercase ASCII letters, digits, dots, underscores, or hyphens", resourceID)
		}
	}
	return resourceID, nil
}

func canonicalFontResourceToken(field, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("font resource provenance %s is required", field)
	}
	if value != strings.TrimSpace(value) {
		return "", fmt.Errorf("font resource provenance %s %q must not have surrounding whitespace", field, value)
	}
	return value, nil
}

func validateFontResourceStagedPath(resourceID, stagedPath string) error {
	if _, err := canonicalFontResourceToken(fmt.Sprintf("font resource %q staged path", resourceID), stagedPath); err != nil {
		return err
	}
	if strings.Contains(stagedPath, `\`) || path.IsAbs(stagedPath) || path.Clean(stagedPath) != stagedPath || stagedPath == "." || stagedPath == ".." || strings.HasPrefix(stagedPath, "../") {
		return fmt.Errorf("font resource provenance font resource %q staged path must be a clean relative POSIX path", resourceID)
	}
	return nil
}
