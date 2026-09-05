from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: anchor count={count}, want 1 for {old!r}")
    p.write_text(text.replace(old, new, 1))


replace_once(
    "backend/internal/video/renderer_fidelity.go",
    '\trendererFidelityKindCursorRing    = "cursor_ring"\n',
    '\trendererFidelityKindCursorRing    = "cursor_ring"\n\trendererFidelityKindShapeRaster   = "shape_raster"\n',
)

replace_once(
    "backend/internal/video/renderer_fidelity.go",
    '\tcursorCleanup, err := materializeCanonicalCursorRasterAssets(&req)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tdefer cursorCleanup()\n\treturn r.delegate.Render(ctx, req, progress)\n',
    '\tshapeCleanup, err := materializeCanonicalShapeRasterAssets(&req)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tdefer shapeCleanup()\n\tcursorCleanup, err := materializeCanonicalCursorRasterAssets(&req)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tdefer cursorCleanup()\n\treturn r.delegate.Render(ctx, req, progress)\n',
)

replace_once(
    "backend/internal/video/renderer_fidelity.go",
    '\t\tfor _, original := range siblings {\n\t\t\tclip := normalizeRenderClip(original)\n',
    '\t\tfor _, original := range siblings {\n\t\t\tclip := normalizeRenderClip(original)\n\t\t\tif original.Shape != nil && normalizeTimelineToken(original.Shape.Kind) == ShapeKindRoundedRectangle {\n\t\t\t\tif rasterClip, ok := canonicalRoundedRectangleRasterClip(original, out.Canvas, out.Scenes); ok {\n\t\t\t\t\tclip = rasterClip\n\t\t\t\t}\n\t\t\t}\n',
)

# Add one integration assertion to the new focused test file.
p = Path("backend/internal/video/renderer_shape_raster_test.go")
text = p.read_text()
marker = "func TestParseShapeRasterColorSupportsCanonicalSRGBForms(t *testing.T) {"
if text.count(marker) != 1:
    raise SystemExit("renderer_shape_raster_test.go integration-test anchor mismatch")
integration = r'''func TestExpandTimelineForFidelityUsesRoundedRectangleRasterAsset(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{roundedRectangleTestClip()}
	expanded := ExpandTimelineForFidelity(doc, 30, 300)
	if len(expanded.Tracks[0].Clips) != 1 {
		t.Fatalf("expanded rounded rectangle clips = %d, want 1", len(expanded.Tracks[0].Clips))
	}
	generated := expanded.Tracks[0].Clips[0]
	if generated.Shape != nil || !strings.HasPrefix(generated.AssetID, shapeRasterAssetPrefix) {
		t.Fatalf("rounded rectangle did not become canonical raster media: %+v", generated)
	}
}

'''
p.write_text(text.replace(marker, integration + marker, 1))
