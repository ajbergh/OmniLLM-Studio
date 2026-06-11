package video

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GenerateAssetArtifacts renders a poster thumbnail (video/image assets) and a
// waveform image (audio assets) next to the source file using FFmpeg.
// Best-effort: returns relative paths ("" when skipped or failed) and never
// blocks ingest on FFmpeg being unavailable.
func GenerateAssetArtifacts(ctx context.Context, rootDir, relPath, mimeType string) (thumbnailRel, waveformRel string) {
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", ""
	}
	srcAbs := filepath.Join(rootDir, filepath.FromSlash(relPath))
	if _, err := os.Stat(srcAbs); err != nil {
		return "", ""
	}
	mime := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))

	switch {
	case strings.HasPrefix(mime, "video/"):
		thumbnailRel = runArtifactFFmpeg(ctx, binary, relPath, srcAbs, ".thumb.jpg",
			[]string{"-ss", "0.5", "-i", srcAbs, "-frames:v", "1", "-vf", "scale=320:-2"})
	case strings.HasPrefix(mime, "image/"):
		thumbnailRel = runArtifactFFmpeg(ctx, binary, relPath, srcAbs, ".thumb.jpg",
			[]string{"-i", srcAbs, "-frames:v", "1", "-vf", "scale=320:-2"})
	case strings.HasPrefix(mime, "audio/"):
		waveformRel = runArtifactFFmpeg(ctx, binary, relPath, srcAbs, ".wave.png",
			[]string{"-i", srcAbs, "-filter_complex", "showwavespic=s=640x120:colors=0x4ade80", "-frames:v", "1"})
	}
	return thumbnailRel, waveformRel
}

// runArtifactFFmpeg runs ffmpeg with the given input args and writes
// <src><suffix>; returns the relative output path or "" on any failure.
func runArtifactFFmpeg(ctx context.Context, binary, relPath, srcAbs, suffix string, inputArgs []string) string {
	outAbs := srcAbs + suffix
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := append([]string{"-hide_banner", "-loglevel", "error", "-y"}, inputArgs...)
	args = append(args, outAbs)
	if err := exec.CommandContext(runCtx, binary, args...).Run(); err != nil {
		_ = os.Remove(outAbs)
		return ""
	}
	if info, err := os.Stat(outAbs); err != nil || info.Size() == 0 {
		_ = os.Remove(outAbs)
		return ""
	}
	return relPath + suffix
}
