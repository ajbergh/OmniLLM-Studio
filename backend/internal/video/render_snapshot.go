package video

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

const (
	legacySnapshotRenderer        = "ffmpeg"
	legacySnapshotRendererVersion = "fidelity-v1"
)

// RenderAssetManifestEntry freezes the asset record and the content identity
// used by a render. ClipIDs make missing/corrupt-source failures actionable.
type RenderAssetManifestEntry struct {
	Asset      models.VideoAsset `json:"asset"`
	ClipIDs    []string          `json:"clip_ids"`
	FileSHA256 string            `json:"file_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Media      *MediaProbe       `json:"media,omitempty"`
}

func contentSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileContentSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func timelineAssetClipIDs(doc TimelineDocument) map[string][]string {
	result := map[string][]string{}
	for _, track := range doc.Tracks {
		for _, clip := range track.Clips {
			assetID := strings.TrimSpace(clip.AssetID)
			if assetID == "" {
				continue
			}
			result[assetID] = append(result[assetID], clip.ID)
		}
	}
	return result
}

// resolveRenderAssetPath resolves symlinks for both the storage root and the
// source file, then re-checks containment. Lexical safeJoin alone cannot stop
// an in-root symlink from pointing outside managed storage.
func resolveRenderAssetPath(attachmentsDir, storedPath string) (string, error) {
	candidate, err := safeJoin(attachmentsDir, filepath.FromSlash(storedPath))
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(attachmentsDir)
	if err != nil {
		// Windows application sandboxes can allow direct access to a workspace
		// while denying metadata traversal on one of its user-profile ancestors.
		// In that specific case EvalSymlinks fails even though root/candidate are
		// accessible. Fall back to rejecting every symlink/reparse entry beneath
		// the lexical root, which preserves the escape boundary without needing
		// to resolve the inaccessible ancestor chain.
		accessDenied := errors.Is(err, os.ErrPermission) || os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "access is denied")
		if runtime.GOOS == "windows" && accessDenied {
			return resolveRenderAssetPathWithoutSymlinks(attachmentsDir, candidate)
		}
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes storage root")
	}
	return resolvedPath, nil
}

func resolveRenderAssetPathWithoutSymlinks(attachmentsDir, candidate string) (string, error) {
	absRoot, err := filepath.Abs(attachmentsDir)
	if err != nil {
		return "", err
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(absRoot, absCandidate)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes storage root")
	}
	current := absRoot
	parts := []string{"."}
	if relative != "." {
		parts = append(parts, strings.Split(relative, string(filepath.Separator))...)
	}
	for _, part := range parts {
		if part != "." {
			current = filepath.Join(current, part)
		}
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path contains a symlink or reparse point inside storage root")
		}
	}
	return absCandidate, nil
}

func renderSnapshotStagingDir(attachmentsDir, snapshotID string) (string, error) {
	segment := sanitizePathSegment(snapshotID)
	if segment == "" || segment != snapshotID {
		return "", fmt.Errorf("invalid render snapshot id")
	}
	return safeJoin(attachmentsDir, filepath.Join("video", "render-snapshots", segment))
}

func removeRenderSnapshotStaging(attachmentsDir, snapshotID string) error {
	directory, err := renderSnapshotStagingDir(attachmentsDir, snapshotID)
	if err != nil {
		return err
	}
	return os.RemoveAll(directory)
}

func stageRenderAsset(sourcePath, destinationPath string) (hash string, size int64, err error) {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return "", 0, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destinationPath), ".snapshot-input-*")
	if err != nil {
		return "", 0, err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	digest := sha256.New()
	size, err = io.Copy(io.MultiWriter(temporary, digest), source)
	if err != nil {
		return "", 0, err
	}
	if err := temporary.Sync(); err != nil {
		return "", 0, err
	}
	if err := temporary.Close(); err != nil {
		return "", 0, err
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return "", 0, err
	}
	committed = true
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}

// buildRenderAssetManifest validates every referenced source and returns a
// stable, sorted manifest. Paths must remain inside the attachments root.
func (s *Service) buildRenderAssetManifest(ctx context.Context, projectID, snapshotID string, doc TimelineDocument) (entries []RenderAssetManifestEntry, manifestJSON string, manifestSHA256 string, err error) {
	if _, err := renderSnapshotStagingDir(s.attachmentsDir, snapshotID); err != nil {
		return nil, "", "", err
	}
	defer func() {
		if err != nil {
			_ = removeRenderSnapshotStaging(s.attachmentsDir, snapshotID)
		}
	}()
	references := timelineAssetClipIDs(doc)
	assetList, err := s.assets.ListByProject(projectID)
	if err != nil {
		return nil, "", "", err
	}
	assetByID := make(map[string]models.VideoAsset, len(assetList))
	for _, asset := range assetList {
		assetByID[asset.ID] = asset
	}
	ids := make([]string, 0, len(references))
	for assetID := range references {
		ids = append(ids, assetID)
	}
	sort.Strings(ids)
	entries = make([]RenderAssetManifestEntry, 0, len(ids))
	for _, assetID := range ids {
		asset, ok := assetByID[assetID]
		if !ok || asset.ProjectID == nil || *asset.ProjectID != projectID {
			return nil, "", "", fmt.Errorf("timeline clips %s reference missing asset %q", strings.Join(references[assetID], ", "), assetID)
		}
		if strings.TrimSpace(asset.FilePath) == "" {
			return nil, "", "", fmt.Errorf("timeline clips %s reference asset %q with no source file", strings.Join(references[assetID], ", "), assetID)
		}
		fullPath, err := resolveRenderAssetPath(s.attachmentsDir, asset.FilePath)
		if err != nil {
			return nil, "", "", fmt.Errorf("asset %q source path is invalid: %w", assetID, err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("asset %q source file is unavailable: %w", assetID, err)
		}
		if !info.Mode().IsRegular() {
			return nil, "", "", fmt.Errorf("asset %q source is not a regular file", assetID)
		}
		assetSegment := sanitizePathSegment(asset.ID)
		if assetSegment == "" {
			return nil, "", "", fmt.Errorf("asset %q has an invalid id", assetID)
		}
		stagedName := sanitizeFileName(asset.FileName)
		if stagedName == "" {
			stagedName = "source" + filepath.Ext(asset.FilePath)
		}
		stagedRelativePath := filepath.Join("video", "render-snapshots", snapshotID, "inputs", assetSegment, stagedName)
		stagedPath, err := safeJoin(s.attachmentsDir, stagedRelativePath)
		if err != nil {
			return nil, "", "", fmt.Errorf("stage asset %q source path: %w", assetID, err)
		}
		fileHash, sizeBytes, err := stageRenderAsset(fullPath, stagedPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("stage asset %q source: %w", assetID, err)
		}
		stagedAsset := asset
		stagedAsset.FilePath = filepath.ToSlash(stagedRelativePath)
		stagedAsset.SizeBytes = sizeBytes
		var media *MediaProbe
		if renderAssetRequiresMediaProbe(stagedAsset) {
			media, err = ProbeMediaRequired(ctx, stagedPath)
			if err != nil {
				return nil, "", "", fmt.Errorf("timeline clips %s reference corrupt or undecodable asset %q: %w", strings.Join(references[assetID], ", "), assetID, err)
			}
			if renderAssetRequiresAudio(stagedAsset) && !media.HasAudio {
				return nil, "", "", fmt.Errorf("timeline clips %s reference asset %q without a decodable audio stream", strings.Join(references[assetID], ", "), assetID)
			}
			if renderAssetRequiresVideo(stagedAsset) && media.VideoCodec == "" && media.Width == 0 && media.Height == 0 {
				return nil, "", "", fmt.Errorf("timeline clips %s reference asset %q without a decodable visual stream", strings.Join(references[assetID], ", "), assetID)
			}
			stagedAsset.MetadataJSON = mergeProbeMetadataJSON(stagedAsset.MetadataJSON, media)
		}
		entries = append(entries, RenderAssetManifestEntry{
			Asset: stagedAsset, ClipIDs: append([]string(nil), references[assetID]...),
			FileSHA256: fileHash, SizeBytes: sizeBytes, Media: media,
		})
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, "", "", fmt.Errorf("marshal render asset manifest: %w", err)
	}
	return entries, string(raw), contentSHA256(raw), nil
}

// RenderFontManifestEntry freezes one packaged font face and the content
// identity used by a render. ClipIDs make missing-face failures actionable.
type RenderFontManifestEntry struct {
	Asset      models.VideoAsset `json:"asset"`
	ClipIDs    []string          `json:"clip_ids"`
	FileSHA256 string            `json:"file_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
}

// timelineFontResourceClipIDs collects every authored font_resource_id on text
// clips mapped to the referencing clip IDs. The map key is the resource id.
func timelineFontResourceClipIDs(doc TimelineDocument) map[string][]string {
	result := map[string][]string{}
	for _, track := range doc.Tracks {
		for _, clip := range track.Clips {
			if clip.Text == nil {
				continue
			}
			resourceID := strings.TrimSpace(clip.Text.FontResourceID)
			if resourceID == "" {
				continue
			}
			result[resourceID] = append(result[resourceID], clip.ID)
		}
	}
	return result
}

// buildRenderFontManifest stages every font asset referenced by an authored
// text font_resource_id into the snapshot's fonts/ directory and returns a
// stable, sorted manifest. A referenced resource id with no project font
// asset fails closed; unreferenced font assets are not packaged.
func (s *Service) buildRenderFontManifest(ctx context.Context, projectID, snapshotID string, doc TimelineDocument) (entries []RenderFontManifestEntry, manifestJSON string, manifestSHA256 string, err error) {
	references := timelineFontResourceClipIDs(doc)
	if len(references) == 0 {
		return []RenderFontManifestEntry{}, "[]", contentSHA256([]byte("[]")), nil
	}
	fontAssets, err := s.fontAssetsByResourceID(projectID)
	if err != nil {
		return nil, "", "", err
	}
	ids := make([]string, 0, len(references))
	for resourceID := range references {
		ids = append(ids, resourceID)
	}
	sort.Strings(ids)
	entries = make([]RenderFontManifestEntry, 0, len(ids))
	for _, resourceID := range ids {
		asset, ok := fontAssets[resourceID]
		if !ok {
			return nil, "", "", fmt.Errorf("timeline clips %s reference font resource %q that the project does not provide", strings.Join(references[resourceID], ", "), resourceID)
		}
		if strings.TrimSpace(asset.FilePath) == "" {
			return nil, "", "", fmt.Errorf("font resource %q has no source file", resourceID)
		}
		fullPath, err := resolveRenderAssetPath(s.attachmentsDir, asset.FilePath)
		if err != nil {
			return nil, "", "", fmt.Errorf("font resource %q source path is invalid: %w", resourceID, err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("font resource %q source file is unavailable: %w", resourceID, err)
		}
		if !info.Mode().IsRegular() {
			return nil, "", "", fmt.Errorf("font resource %q source is not a regular file", resourceID)
		}
		stagedName := sanitizeFileName(asset.FileName)
		if stagedName == "" {
			stagedName = "face" + filepath.Ext(asset.FilePath)
		}
		stagedRelativePath := filepath.Join("video", "render-snapshots", snapshotID, "fonts", sanitizePathSegment(resourceID), stagedName)
		stagedPath, err := safeJoin(s.attachmentsDir, stagedRelativePath)
		if err != nil {
			return nil, "", "", fmt.Errorf("stage font resource %q path: %w", resourceID, err)
		}
		fileHash, sizeBytes, err := stageRenderAsset(fullPath, stagedPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("stage font resource %q bytes: %w", resourceID, err)
		}
		stagedAsset := asset
		stagedAsset.FilePath = filepath.ToSlash(stagedRelativePath)
		stagedAsset.SizeBytes = sizeBytes
		entries = append(entries, RenderFontManifestEntry{
			Asset: stagedAsset, ClipIDs: append([]string(nil), references[resourceID]...),
			FileSHA256: fileHash, SizeBytes: sizeBytes,
		})
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, "", "", fmt.Errorf("marshal render font manifest: %w", err)
	}
	return entries, string(raw), contentSHA256(raw), nil
}

// fontAssetsByResourceID resolves the project's font-kind assets by their
// declared font_resource_id metadata. Duplicate or missing declarations fail
// closed so one resource id always names exactly one immutable face.
func (s *Service) fontAssetsByResourceID(projectID string) (map[string]models.VideoAsset, error) {
	assets, err := s.assets.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]models.VideoAsset)
	for _, asset := range assets {
		if !strings.EqualFold(strings.TrimSpace(asset.Kind), "font") || asset.ProjectID == nil || *asset.ProjectID != projectID {
			continue
		}
		resourceID := strings.TrimSpace(fontResourceIDFromMetadata(asset))
		if resourceID == "" {
			continue
		}
		if existing, exists := result[resourceID]; exists {
			return nil, fmt.Errorf("project declares font resource %q on both %q and %q", resourceID, existing.FileName, asset.FileName)
		}
		result[resourceID] = asset
	}
	return result, nil
}

// fontResourceIDFromMetadata reads the canonical font_resource_id declaration
// from an uploaded font asset's metadata JSON.
func fontResourceIDFromMetadata(asset models.VideoAsset) string {
	if strings.TrimSpace(asset.MetadataJSON) == "" {
		return ""
	}
	var metadata struct {
		FontResourceID string `json:"font_resource_id"`
	}
	if err := json.Unmarshal([]byte(asset.MetadataJSON), &metadata); err != nil {
		return ""
	}
	return metadata.FontResourceID
}

// snapshotHasFontResources reports whether an immutable snapshot packaged any
// declared font faces.
func snapshotHasFontResources(snapshot *models.VideoRenderSnapshot) bool {
	if snapshot == nil || strings.TrimSpace(snapshot.FontManifestJSON) == "" {
		return false
	}
	var entries []RenderFontManifestEntry
	if err := json.Unmarshal([]byte(snapshot.FontManifestJSON), &entries); err != nil {
		return false
	}
	return len(entries) > 0
}

func renderAssetRequiresMediaProbe(asset models.VideoAsset) bool {
	kind := strings.ToLower(strings.TrimSpace(asset.Kind))
	mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
	return kind == "video" || kind == "audio" || kind == "music" || kind == "image" ||
		strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "image/")
}

func renderAssetRequiresAudio(asset models.VideoAsset) bool {
	kind := strings.ToLower(strings.TrimSpace(asset.Kind))
	mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
	return kind == "audio" || kind == "music" || strings.HasPrefix(mimeType, "audio/")
}

func renderAssetRequiresVideo(asset models.VideoAsset) bool {
	kind := strings.ToLower(strings.TrimSpace(asset.Kind))
	mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
	return kind == "video" || kind == "image" || strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "image/")
}

// assetsFromRenderSnapshot verifies that the immutable manifest and source
// bytes still match enqueue-time hashes, then reconstructs the renderer map
// without reading mutable asset rows.
func (s *Service) assetsFromRenderSnapshot(snapshot *models.VideoRenderSnapshot) (map[string]models.VideoAsset, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("render snapshot is required")
	}
	if contentSHA256([]byte(snapshot.AssetManifestJSON)) != snapshot.AssetManifestSHA256 {
		return nil, fmt.Errorf("render snapshot asset manifest hash mismatch")
	}
	var entries []RenderAssetManifestEntry
	if err := json.Unmarshal([]byte(snapshot.AssetManifestJSON), &entries); err != nil {
		return nil, fmt.Errorf("parse render asset manifest: %w", err)
	}
	assets := make(map[string]models.VideoAsset, len(entries))
	for _, entry := range entries {
		asset := entry.Asset
		fullPath, err := resolveRenderAssetPath(s.attachmentsDir, asset.FilePath)
		if err != nil {
			return nil, fmt.Errorf("snapshot asset %q source path is invalid: %w", asset.ID, err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, fmt.Errorf("snapshot asset %q source file is unavailable: %w", asset.ID, err)
		}
		if !info.Mode().IsRegular() || info.Size() != entry.SizeBytes {
			return nil, fmt.Errorf("snapshot asset %q source file changed after submission", asset.ID)
		}
		fileHash, err := fileContentSHA256(fullPath)
		if err != nil {
			return nil, fmt.Errorf("verify snapshot asset %q source: %w", asset.ID, err)
		}
		if fileHash != entry.FileSHA256 {
			return nil, fmt.Errorf("snapshot asset %q source content changed after submission", asset.ID)
		}
		assets[asset.ID] = asset
	}
	if err := s.verifyFontsFromRenderSnapshot(snapshot); err != nil {
		return nil, err
	}
	return assets, nil
}

// verifyFontsFromRenderSnapshot re-verifies the immutable font manifest hash
// and every staged font face's bytes before a render consumes the snapshot.
func (s *Service) verifyFontsFromRenderSnapshot(snapshot *models.VideoRenderSnapshot) error {
	if strings.TrimSpace(snapshot.FontManifestJSON) == "" {
		if snapshot.FontManifestSHA256 != "" {
			return fmt.Errorf("render snapshot font manifest hash mismatch")
		}
		return nil
	}
	if contentSHA256([]byte(snapshot.FontManifestJSON)) != snapshot.FontManifestSHA256 {
		return fmt.Errorf("render snapshot font manifest hash mismatch")
	}
	var entries []RenderFontManifestEntry
	if err := json.Unmarshal([]byte(snapshot.FontManifestJSON), &entries); err != nil {
		return fmt.Errorf("parse render font manifest: %w", err)
	}
	for _, entry := range entries {
		fullPath, err := resolveRenderAssetPath(s.attachmentsDir, entry.Asset.FilePath)
		if err != nil {
			return fmt.Errorf("snapshot font %q source path is invalid: %w", entry.Asset.ID, err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return fmt.Errorf("snapshot font %q source file is unavailable: %w", entry.Asset.ID, err)
		}
		if !info.Mode().IsRegular() || info.Size() != entry.SizeBytes {
			return fmt.Errorf("snapshot font %q source file changed after submission", entry.Asset.ID)
		}
		fileHash, err := fileContentSHA256(fullPath)
		if err != nil {
			return fmt.Errorf("verify snapshot font %q source: %w", entry.Asset.ID, err)
		}
		if fileHash != entry.FileSHA256 {
			return fmt.Errorf("snapshot font %q source content changed after submission", entry.Asset.ID)
		}
	}
	return nil
}
