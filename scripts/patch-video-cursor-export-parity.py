from pathlib import Path

branch_file = Path('backend/internal/video/renderer_fidelity.go')
text = branch_file.read_text()

old_tail = '''\tif req.Settings.DiagnosticFrameIndex != nil {
\t\t// Fidelity expansion can create hundreds of short sampled segments per
\t\t// clip. Keep authored activity in the canonical frame domain, but select
\t\t// exactly one synthetic fidelity sample per authored clip at the exact
\t\t// rational frame presentation time so adjacent samples never double-stack.
\t\ttimelineOffsetMS := int64(0)
\t\tif req.Settings.RangeEndMS > req.Settings.RangeStartMS {
\t\t\ttimelineOffsetMS = req.Settings.RangeStartMS
\t\t}
\t\treq.Timeline = FilterTimelineAtDiagnosticFrame(req.Timeline, *req.Settings.DiagnosticFrameIndex, fps, timelineOffsetMS)
\t}
\treturn r.delegate.Render(ctx, req, progress)
}'''
new_tail = '''\tif req.Settings.DiagnosticFrameIndex != nil {
\t\t// Fidelity expansion can create hundreds of short sampled segments per
\t\t// clip. Keep authored activity in the canonical frame domain, but select
\t\t// exactly one synthetic fidelity sample per authored clip at the exact
\t\t// rational frame presentation time so adjacent samples never double-stack.
\t\ttimelineOffsetMS := int64(0)
\t\tif req.Settings.RangeEndMS > req.Settings.RangeStartMS {
\t\t\ttimelineOffsetMS = req.Settings.RangeStartMS
\t\t}
\t\treq.Timeline = FilterTimelineAtDiagnosticFrame(req.Timeline, *req.Settings.DiagnosticFrameIndex, fps, timelineOffsetMS)
\t}
\tcursorCleanup, err := materializeCanonicalCursorRasterAssets(&req)
\tif err != nil {
\t\treturn nil, err
\t}
\tdefer cursorCleanup()
\treturn r.delegate.Render(ctx, req, progress)
}'''
assert old_tail in text
text = text.replace(old_tail, new_tail, 1)

start = text.index('func ExpandTimelineForFidelity(doc TimelineDocument, fps, maxSegments int) TimelineDocument {')
end = text.index('\nfunc cloneTimelineDocument', start)
new_expand = '''func ExpandTimelineForFidelity(doc TimelineDocument, fps, maxSegments int) TimelineDocument {
\toutputFPS := fps
\tif outputFPS <= 0 {
\t\toutputFPS = 30
\t}
\tsampleFPS := outputFPS
\tif sampleFPS > 60 {
\t\tsampleFPS = 60
\t}
\tif maxSegments <= 0 {
\t\tmaxSegments = 300
\t}
\tout := cloneTimelineDocument(doc)
\tfor ti := range out.Tracks {
\t\tsiblings := append([]TimelineClip(nil), out.Tracks[ti].Clips...)
\t\texpanded := make([]TimelineClip, 0, len(siblings))
\t\tfor _, original := range siblings {
\t\t\tclip := normalizeRenderClip(original)
\t\t\tif clip.AudioOnly || out.Tracks[ti].Type == TrackTypeAudio || out.Tracks[ti].Type == TrackTypeMusic {
\t\t\t\tclip.Cursor = nil
\t\t\t\t// The FFmpeg audio graph already evaluates trim/rate, volume
\t\t\t\t// keyframes, fades, mute/solo, and processing continuously. Splitting
\t\t\t\t// an audio clip into visual sampling segments resets its fades and
\t\t\t\t// duplicates the same source hundreds of times.
\t\t\t\texpanded = append(expanded, clip)
\t\t\t\tcontinue
\t\t\t}
\t\t\tcursorOverlays := cursorOverlayClips(clip, siblings, out.Canvas, out.Scenes, outputFPS, maxSegments)
\t\t\tclip.Cursor = nil
\t\t\tif clipNeedsSampling(clip) || clipNeedsCameraSampling(out.Scenes, clip) {
\t\t\t\tsegments := sampleRenderClip(clip, out.Scenes, out.Canvas, sampleFPS, maxSegments)
\t\t\t\tif clip.AssetID != "" && !clip.Muted {
\t\t\t\t\t// Sampled visual segments must not each contribute a soundtrack.
\t\t\t\t\t// Keep one untouched audio-only copy so the mix remains continuous;
\t\t\t\t\t// image assets naturally drop out during media resolution.
\t\t\t\t\tfor index := range segments {
\t\t\t\t\t\tsegments[index].Muted = true
\t\t\t\t\t}
\t\t\t\t\taudioCopy := cloneTimelineClip(clip)
\t\t\t\t\taudioCopy.ID = uuid.NewString()
\t\t\t\t\taudioCopy.AudioOnly = true
\t\t\t\t\taudioCopy.Cursor = nil
\t\t\t\t\texpanded = append(expanded, segments...)
\t\t\t\t\texpanded = append(expanded, audioCopy)
\t\t\t\t} else {
\t\t\t\t\texpanded = append(expanded, segments...)
\t\t\t\t}
\t\t\t} else {
\t\t\t\texpanded = append(expanded, applySceneCamera(clip, out.Scenes, out.Canvas, clip.StartMS+clip.DurationMS/2))
\t\t\t}
\t\t\t// Cursor pixels belong to the owner layer, not to a synthetic global
\t\t\t// topmost track. Keeping them adjacent on the same track preserves track
\t\t\t// visibility/order; the exact canonical path additionally rejects
\t\t\t// overlapping same-track siblings until that ordering case is evidenced.
\t\t\texpanded = append(expanded, cursorOverlays...)
\t\t}
\t\tout.Tracks[ti].Clips = expanded
\t}
\treturn out
}
'''
text = text[:start] + new_expand + text[end:]

old_comment = '''// sample segments for one parent are collapsed to the segment containing the
// exact frame presentation time, or the earliest overlapping sample when the
// authored clip begins partway through the output frame. Cursor overlays keep
// their existing point-sampled behavior until cursor semantics are canonical.'''
new_comment = '''// sample segments for one parent are collapsed to the segment containing the
// exact frame presentation time, or the earliest overlapping sample when the
// authored clip begins partway through the output frame. Canonical cursor
// rasters are already one exact output-frame segment each; unsupported cursor
// combinations retain this same point-containment compatibility filtering.'''
assert old_comment in text
text = text.replace(old_comment, new_comment, 1)

old_cursor = 'func cursorOverlayClips(clip TimelineClip, fps, maxSegments int) []TimelineClip {'
new_cursor = '''func cursorOverlayClips(clip TimelineClip, siblings []TimelineClip, canvas TimelineCanvas, scenes []TimelineScene, fps, maxSegments int) []TimelineClip {
\tif overlays, ok := canonicalCursorRasterOverlayClips(clip, siblings, canvas, scenes, fps, maxSegments); ok {
\t\treturn overlays
\t}
\treturn legacyCursorOverlayClips(clip, fps, maxSegments)
}

func legacyCursorOverlayClips(clip TimelineClip, fps, maxSegments int) []TimelineClip {'''
assert old_cursor in text
text = text.replace(old_cursor, new_cursor, 1)

old_constructor_comment = '''// NewFidelityRenderer adds eased transform/effect keyframes, wipe/zoom
// transitions, cursor overlays, click rings, letter-spacing approximation, and
// annotation normalization without changing the persisted timeline document.'''
new_constructor_comment = '''// NewFidelityRenderer adds eased transform/effect keyframes, wipe/zoom
// transitions, canonical cursor rasters with a bounded compatibility fallback,
// letter-spacing approximation, and annotation normalization without changing
// the persisted timeline document.'''
assert old_constructor_comment in text
text = text.replace(old_constructor_comment, new_constructor_comment, 1)
branch_file.write_text(text)

cap = Path('backend/internal/video/renderer_capabilities.go')
text = cap.read_text()
old_cap = '{Feature: RendererFeatureCursor, Label: "Cursor effects", Supported: true, Partial: true, Notes: "Cursor paths and click rings export through sampled overlays. Click audio is not synthesized."},'
new_cap = '{Feature: RendererFeatureCursor, Label: "Cursor effects", Supported: true, Partial: true, Notes: "Static 2D media cursors up to the bounded fidelity segment limit use cursor-state-v1 output-frame sampling plus deterministic pointer, highlight, and click-ring rasters on the owner track. Smoothing, animated/3D/camera/effect/transition parents and longer clips retain the sampled compatibility fallback. Click audio is not synthesized."},'
assert old_cap in text
cap.write_text(text.replace(old_cap, new_cap, 1))

test = Path('backend/internal/video/renderer_cursor_test.go')
text = test.read_text()
old_import = '''import (
\t"image/png"
\t"math"
\t"os"
\t"strings"
\t"testing"
)'''
new_import = '''import (
\t"image/png"
\t"math"
\t"os"
\t"strings"
\t"testing"

\t"github.com/ajbergh/omnillm-studio/internal/models"
)'''
assert old_import in text
test.write_text(text.replace(old_import, new_import, 1))
