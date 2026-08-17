package video

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
func (s *Service) buildRenderAssetManifest(projectID, snapshotID string, doc TimelineDocument) (entries []RenderAssetManifestEntry, manifestJSON string, manifestSHA256 string, err error) {
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
		entries = append(entries, RenderAssetManifestEntry{
			Asset: stagedAsset, ClipIDs: append([]string(nil), references[assetID]...),
			FileSHA256: fileHash, SizeBytes: sizeBytes,
		})
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, "", "", fmt.Errorf("marshal render asset manifest: %w", err)
	}
	return entries, string(raw), contentSHA256(raw), nil
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
	return assets, nil
}
