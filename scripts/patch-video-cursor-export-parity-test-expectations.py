from pathlib import Path

advanced = Path('backend/internal/video/renderer_advanced_test.go')
text = advanced.read_text()
old = '''\tif len(expanded.Tracks) < 2 || len(expanded.Tracks[len(expanded.Tracks)-1].Clips) == 0 {
\t\tt.Fatalf("expected cursor overlay track")
\t}
'''
new = '''\tfoundCursor := false
\tfor _, expandedClip := range expanded.Tracks[0].Clips {
\t\tkind, _ := fidelityGeneratedIdentity(expandedClip)
\t\tif kind == rendererFidelityKindCursorPointer {
\t\t\tfoundCursor = true
\t\t\tbreak
\t\t}
\t}
\tif !foundCursor {
\t\tt.Fatalf("expected cursor overlay on the owner track")
\t}
'''
assert old in text
advanced.write_text(text.replace(old, new, 1))

cursor_test = Path('backend/internal/video/renderer_cursor_test.go')
text = cursor_test.read_text()
old = '''\texpanded := ExpandTimelineForFidelity(doc, 100, 120)
\tif len(expanded.Tracks) != 1 {
\t\tt.Fatalf("canonical cursor must stay on the owner track, got %d tracks", len(expanded.Tracks))
\t}
'''
new = '''\tinitialTrackCount := len(doc.Tracks)
\texpanded := ExpandTimelineForFidelity(doc, 100, 120)
\tif len(expanded.Tracks) != initialTrackCount {
\t\tt.Fatalf("canonical cursor added a synthetic track: before=%d after=%d", initialTrackCount, len(expanded.Tracks))
\t}
'''
assert old in text
cursor_test.write_text(text.replace(old, new, 1))
