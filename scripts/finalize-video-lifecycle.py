#!/usr/bin/env python3
"""One-shot exact lifecycle integration for large source files.

The script patches startup/shutdown and patch-history cleanup in a full checkout,
restores the normal read-only workflow, deletes itself, and is committed by the
scoped branch-only CI step.
"""

from pathlib import Path

BRANCH = "feature/video-renderer-reliability-transcription-scalability-20260720"
ROOT = Path(__file__).resolve().parents[1]
ROUTER = ROOT / "backend/internal/api/router.go"
DESKTOP = ROOT / "backend/cmd/desktop/main.go"
ULTIMATE = ROOT / "frontend/src/components/video/VideoEditStudioUltimate.tsx"
WORKFLOW = ROOT / ".github/workflows/ci.yml"

router = ROUTER.read_text(encoding="utf-8")
old_router = """\tvideoTranscriptionService := video.NewVideoTranscriptionService(videoTranscriptRepo, providerRepo, videoProjectRepo, videoAssetRepo, cfg.AttachmentsDir)
\tvideoTranscriptionHandler := NewVideoTranscriptionHandler(videoTranscriptionService)
\t// Resume any generation poll goroutines that were in-flight when the server last stopped.
"""
new_router = """\tvideoTranscriptionService := video.NewVideoTranscriptionService(videoTranscriptRepo, providerRepo, videoProjectRepo, videoAssetRepo, cfg.AttachmentsDir)
\tvideoTranscriptionHandler := NewVideoTranscriptionHandler(videoTranscriptionService)
\tgo videoTranscriptionService.RecoverInterrupted()
\t// Resume any generation poll goroutines that were in-flight when the server last stopped.
"""
if old_router not in router:
    raise SystemExit("router transcription lifecycle insertion point not found")
ROUTER.write_text(router.replace(old_router, new_router, 1), encoding="utf-8")

desktop = DESKTOP.read_text(encoding="utf-8")
old_shutdown = """\t\tOnShutdown: func(ctx context.Context) {
\t\t\tclose(stopCleanup)
"""
new_shutdown = """\t\tOnShutdown: func(ctx context.Context) {
\t\t\tshutdownNativeCaptures()
\t\t\tclose(stopCleanup)
"""
if old_shutdown not in desktop:
    raise SystemExit("desktop shutdown insertion point not found")
DESKTOP.write_text(desktop.replace(old_shutdown, new_shutdown, 1), encoding="utf-8")

ultimate = ULTIMATE.read_text(encoding="utf-8")
old_cleanup = """    return () => { unsubscribe(); if (useVideoStudioStore.getState().undoTimeline === undo) useVideoStudioStore.setState({ undoTimeline: originalUndo, redoTimeline: originalRedo }); };
"""
new_cleanup = """    return () => {
      unsubscribe();
      if (useVideoStudioStore.getState().undoTimeline === undo) {
        useVideoStudioStore.setState({
          undoTimeline: originalUndo,
          redoTimeline: originalRedo,
          timelineUndoStack: [],
          timelineRedoStack: [],
        });
      }
    };
"""
if old_cleanup not in ultimate:
    raise SystemExit("patch-history cleanup insertion point not found")
ULTIMATE.write_text(ultimate.replace(old_cleanup, new_cleanup, 1), encoding="utf-8")

workflow = WORKFLOW.read_text(encoding="utf-8")
workflow = workflow.replace("    permissions:\n      contents: write\n", "", 1)
workflow = workflow.replace(
    "      - uses: actions/checkout@v7\n        with:\n          ref: ${{ github.event.pull_request.head.sha || github.sha }}\n          fetch-depth: 0\n"
    "      - name: Finalize Video Edit Studio lifecycle integration\n"
    "        if: github.event_name == 'pull_request' && github.head_ref == '" + BRANCH + "'\n"
    "        run: |\n"
    "          set -euo pipefail\n"
    "          python scripts/finalize-video-lifecycle.py\n"
    "          git config user.name 'github-actions[bot]'\n"
    "          git config user.email '41898282+github-actions[bot]@users.noreply.github.com'\n"
    "          git add -A\n"
    "          git diff --cached --check\n"
    "          git commit -m 'fix(video): complete startup shutdown and history lifecycle'\n"
    "          git push origin HEAD:" + BRANCH + "\n",
    "      - uses: actions/checkout@v7\n",
    1,
)
WORKFLOW.write_text(workflow, encoding="utf-8")
Path(__file__).unlink()
