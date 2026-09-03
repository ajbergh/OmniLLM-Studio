package video

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// renderFontResources is the renderer-local view of immutable project font
// assets. Snapshot-backed renders populate Assets with the staged font entries
// from the render font manifest; AttachmentsDir resolves their frozen relative
// storage paths.
type renderFontResources struct {
	attachmentsDir string
	assets         map[string]models.VideoAsset
}

func (resources renderFontResources) index() (map[string]models.VideoAsset, error) {
	indexed := make(map[string]models.VideoAsset)
	for _, asset := range resources.assets {
		if !strings.EqualFold(strings.TrimSpace(asset.Kind), "font") {
			continue
		}
		resourceID := strings.TrimSpace(fontResourceIDFromMetadata(asset))
		if resourceID == "" {
			continue
		}
		if existing, ok := indexed[resourceID]; ok {
			return nil, fmt.Errorf("renderer font resource %q is ambiguous between %q and %q", resourceID, existing.FileName, asset.FileName)
		}
		indexed[resourceID] = asset
	}
	return indexed, nil
}

func (resources renderFontResources) pathFor(asset models.VideoAsset) (string, error) {
	storedPath := strings.TrimSpace(asset.FilePath)
	if storedPath == "" {
		return "", fmt.Errorf("renderer font asset %q has no source file", asset.ID)
	}
	if filepath.IsAbs(storedPath) {
		return filepath.Clean(storedPath), nil
	}
	if strings.TrimSpace(resources.attachmentsDir) == "" {
		return "", fmt.Errorf("renderer font asset %q requires an attachments root", asset.ID)
	}
	resolved, err := safeJoin(resources.attachmentsDir, filepath.FromSlash(storedPath))
	if err != nil {
		return "", fmt.Errorf("renderer font asset %q path is invalid: %w", asset.ID, err)
	}
	return resolved, nil
}

// validateRenderFontResources fails closed before FFmpeg starts when an authored
// font_resource_id cannot resolve to exactly one readable immutable font file.
// Snapshot-backed callers have already hash-verified these bytes; this renderer
// preflight makes accidental direct RenderRequest construction equally explicit.
func validateRenderFontResources(req RenderRequest) error {
	needed := make(map[string][]string)
	for _, track := range req.Timeline.Tracks {
		if !track.Visible {
			continue
		}
		for _, clip := range track.Clips {
			if clip.Text == nil || strings.TrimSpace(clip.Text.Text) == "" {
				continue
			}
			resourceID := strings.TrimSpace(clip.Text.FontResourceID)
			if resourceID != "" {
				needed[resourceID] = append(needed[resourceID], clip.ID)
			}
		}
	}
	if len(needed) == 0 {
		return nil
	}
	resources := renderFontResources{attachmentsDir: req.AttachmentsDir, assets: req.Assets}
	indexed, err := resources.index()
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(needed))
	for resourceID := range needed {
		ids = append(ids, resourceID)
	}
	sort.Strings(ids)
	for _, resourceID := range ids {
		asset, ok := indexed[resourceID]
		if !ok {
			return fmt.Errorf("timeline clips %s require renderer font resource %q, but the immutable render inputs do not provide it", strings.Join(needed[resourceID], ", "), resourceID)
		}
		fontPath, err := resources.pathFor(asset)
		if err != nil {
			return err
		}
		info, err := os.Stat(fontPath)
		if err != nil {
			return fmt.Errorf("renderer font resource %q is unavailable: %w", resourceID, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("renderer font resource %q is not a regular file", resourceID)
		}
	}
	return nil
}

func (resources renderFontResources) fileForText(text TimelineText) string {
	resourceID := strings.TrimSpace(text.FontResourceID)
	if resourceID == "" {
		return ""
	}
	indexed, err := resources.index()
	if err != nil {
		return ""
	}
	asset, ok := indexed[resourceID]
	if !ok {
		return ""
	}
	fontPath, err := resources.pathFor(asset)
	if err != nil {
		return ""
	}
	return fontPath
}

// drawTextFilterWithFontResources keeps the legacy family-name path for text
// without a resource id. A resource-backed clip instead selects the exact
// staged font file and removes fontconfig family selection so export cannot
// silently substitute a different installed face.
func drawTextFilterWithFontResources(clip TimelineClip, text TimelineText, width, height int, resources ...renderFontResources) string {
	filter := drawTextFilter(clip, text, width, height)
	if filter == "" || strings.TrimSpace(text.FontResourceID) == "" || len(resources) == 0 {
		return filter
	}
	fontPath := resources[0].fileForText(text)
	if fontPath == "" {
		return filter
	}
	if family := strings.TrimSpace(text.FontFamily); family != "" {
		filter = strings.Replace(filter, ":font='"+escapeDrawText(family)+"'", "", 1)
	}
	marker := ":fontcolor="
	at := strings.Index(filter, marker)
	if at < 0 {
		return filter
	}
	return filter[:at] + ":fontfile='" + escapeDrawText(fontPath) + "'" + filter[at:]
}
