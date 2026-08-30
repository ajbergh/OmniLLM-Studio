from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected exactly one match, found {count}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1))


path = 'backend/internal/video/parity_fixture_pixelate.go'
replace_once(path,
'''\tParityPixelateOpaqueFixtureName       = "parity-pixelate-opaque-v1"\n\tParityPixelateDecodedVideoFixtureName = "parity-pixelate-decoded-video-v1"\n\tParityPixelateAlphaFixtureName        = "parity-pixelate-alpha-png-v1"\n''',
'''\tParityPixelateOpaqueFixtureName       = "parity-pixelate-opaque-v1"\n\tParityPixelateDecodedVideoFixtureName = "parity-pixelate-decoded-video-v1"\n\tParityPixelateAlphaFixtureName        = "parity-pixelate-alpha-png-v1"\n\tParityPixelateAlphaVideoFixtureName   = "parity-pixelate-alpha-video-v1"\n''')
replace_once(path,
'''func ParityPixelateAlphaFixture() (TimelineDocument, []ParityFixtureAsset) {\n\treturn parityPixelateFixture(\n\t\tParityPixelateAlphaFixtureName,\n\t\tparityPixelateOpaqueCanvasWidth,\n\t\tparityPixelateOpaqueCanvasHeight,\n\t\t"#19324A",\n\t\t"asset-alpha",\n\t\tParityFixtureAsset{ID: "asset-alpha", Kind: "image", Width: 512, Height: 512, DurationMS: parityPixelateDurationMS, Description: "deterministic NRGBA PNG with hidden RGB and 0/64/128/192/255 alpha"},\n\t\t"Partial-alpha PNG pixelate source",\n\t)\n}\n''',
'''func ParityPixelateAlphaFixture() (TimelineDocument, []ParityFixtureAsset) {\n\treturn parityPixelateFixture(\n\t\tParityPixelateAlphaFixtureName,\n\t\tparityPixelateOpaqueCanvasWidth,\n\t\tparityPixelateOpaqueCanvasHeight,\n\t\t"#19324A",\n\t\t"asset-alpha",\n\t\tParityFixtureAsset{ID: "asset-alpha", Kind: "image", Width: 512, Height: 512, DurationMS: parityPixelateDurationMS, Description: "deterministic NRGBA PNG with hidden RGB and 0/64/128/192/255 alpha"},\n\t\t"Partial-alpha PNG pixelate source",\n\t)\n}\n\n// ParityPixelateAlphaVideoFixture moves two partial-alpha regions over the same\n// non-black project background used by the PNG alpha control. The source is a\n// two-second VP9 WebM carrying alpha_mode=1. Frame content changes continuously\n// so stale presentation identity and alpha loss cannot accidentally pass on a\n// static raster.\nfunc ParityPixelateAlphaVideoFixture() (TimelineDocument, []ParityFixtureAsset) {\n\treturn parityPixelateFixture(\n\t\tParityPixelateAlphaVideoFixtureName,\n\t\tparityPixelateOpaqueCanvasWidth,\n\t\tparityPixelateOpaqueCanvasHeight,\n\t\t"#19324A",\n\t\t"asset-alpha-video",\n\t\tParityFixtureAsset{ID: "asset-alpha-video", Kind: "video", Width: 512, Height: 512, DurationMS: parityPixelateDurationMS, Description: "deterministic changing VP9 WebM with alpha_mode=1 and partial-alpha moving regions"},\n\t\t"Partial-alpha VP9 pixelate source",\n\t)\n}\n''')
replace_once(path,
'''func ParityPixelateAlphaFrameSamples() []ParityFrameSample {\n\treturn parityPixelateFrameSamples("pixelate-alpha")\n}\n''',
'''func ParityPixelateAlphaFrameSamples() []ParityFrameSample {\n\treturn parityPixelateFrameSamples("pixelate-alpha")\n}\n\n// ParityPixelateAlphaVideoFrameSamples uses the same canonical output-frame\n// identities while requiring the changing transparent decoder to present each\n// requested source frame.\nfunc ParityPixelateAlphaVideoFrameSamples() []ParityFrameSample {\n\treturn parityPixelateFrameSamples("pixelate-alpha-video")\n}\n''')
replace_once(path,
'''func ParityPixelateAlphaRegionBounds() ParityBounds {\n\treturn parityPixelateRegionBounds(parityPixelateOpaqueCanvasWidth, parityPixelateOpaqueCanvasHeight)\n}\n''',
'''func ParityPixelateAlphaRegionBounds() ParityBounds {\n\treturn parityPixelateRegionBounds(parityPixelateOpaqueCanvasWidth, parityPixelateOpaqueCanvasHeight)\n}\n\n// ParityPixelateAlphaVideoRegionBounds shares the transparent-PNG rectangle so\n// the retained video evidence changes only source codec/presentation semantics.\nfunc ParityPixelateAlphaVideoRegionBounds() ParityBounds {\n\treturn parityPixelateRegionBounds(parityPixelateOpaqueCanvasWidth, parityPixelateOpaqueCanvasHeight)\n}\n''')
replace_once(path,
'''func ParityPixelateAlphaRegionFrames(samples []ParityFrameSample) []ParityFixtureRegionFrame {\n\treturn parityPixelateRegionFrames(samples, ParityPixelateAlphaRegionBounds())\n}\n''',
'''func ParityPixelateAlphaRegionFrames(samples []ParityFrameSample) []ParityFixtureRegionFrame {\n\treturn parityPixelateRegionFrames(samples, ParityPixelateAlphaRegionBounds())\n}\n\n// ParityPixelateAlphaVideoRegionFrames binds changing transparent-video output\n// to exact canonical frame identities and the established pixelate rectangle.\nfunc ParityPixelateAlphaVideoRegionFrames(samples []ParityFrameSample) []ParityFixtureRegionFrame {\n\treturn parityPixelateRegionFrames(samples, ParityPixelateAlphaVideoRegionBounds())\n}\n''')

path = 'backend/internal/video/parity_fixture_pixelate_test.go'
replace_once(path,
'''func TestParityPixelateAlphaFixtureIsValidAndUsesNonBlackBackground(t *testing.T) {\n\tdoc, assets := ParityPixelateAlphaFixture()\n\tvalidated, err := ValidateTimelineDocument(doc)\n\tif err != nil {\n\t\tt.Fatalf("ValidateTimelineDocument() error = %v", err)\n\t}\n\tif validated.Canvas.Width != 512 || validated.Canvas.Height != 512 || validated.Canvas.FPS != 30 {\n\t\tt.Fatalf("canvas = %dx%d@%d, want 512x512@30", validated.Canvas.Width, validated.Canvas.Height, validated.Canvas.FPS)\n\t}\n\tif validated.Canvas.Background != "#19324A" {\n\t\tt.Fatalf("background = %q, want #19324A", validated.Canvas.Background)\n\t}\n\tif len(assets) != 2 || assets[0].ID != "asset-alpha" || assets[0].Kind != "image" || assets[0].Width != 512 || assets[0].Height != 512 || assets[1].ID != "asset-audio" {\n\t\tt.Fatalf("assets = %#v, want alpha PNG plus audio harness asset", assets)\n\t}\n\tassertIsolatedPixelateFixture(t, validated)\n}\n''',
'''func TestParityPixelateAlphaFixtureIsValidAndUsesNonBlackBackground(t *testing.T) {\n\tdoc, assets := ParityPixelateAlphaFixture()\n\tvalidated, err := ValidateTimelineDocument(doc)\n\tif err != nil {\n\t\tt.Fatalf("ValidateTimelineDocument() error = %v", err)\n\t}\n\tif validated.Canvas.Width != 512 || validated.Canvas.Height != 512 || validated.Canvas.FPS != 30 {\n\t\tt.Fatalf("canvas = %dx%d@%d, want 512x512@30", validated.Canvas.Width, validated.Canvas.Height, validated.Canvas.FPS)\n\t}\n\tif validated.Canvas.Background != "#19324A" {\n\t\tt.Fatalf("background = %q, want #19324A", validated.Canvas.Background)\n\t}\n\tif len(assets) != 2 || assets[0].ID != "asset-alpha" || assets[0].Kind != "image" || assets[0].Width != 512 || assets[0].Height != 512 || assets[1].ID != "asset-audio" {\n\t\tt.Fatalf("assets = %#v, want alpha PNG plus audio harness asset", assets)\n\t}\n\tassertIsolatedPixelateFixture(t, validated)\n}\n\nfunc TestParityPixelateAlphaVideoFixtureIsValidAndUsesChangingVideoSource(t *testing.T) {\n\tdoc, assets := ParityPixelateAlphaVideoFixture()\n\tvalidated, err := ValidateTimelineDocument(doc)\n\tif err != nil {\n\t\tt.Fatalf("ValidateTimelineDocument() error = %v", err)\n\t}\n\tif validated.Canvas.Width != 512 || validated.Canvas.Height != 512 || validated.Canvas.FPS != 30 {\n\t\tt.Fatalf("canvas = %dx%d@%d, want 512x512@30", validated.Canvas.Width, validated.Canvas.Height, validated.Canvas.FPS)\n\t}\n\tif validated.Canvas.Background != "#19324A" {\n\t\tt.Fatalf("background = %q, want #19324A", validated.Canvas.Background)\n\t}\n\tif len(assets) != 2 || assets[0].ID != "asset-alpha-video" || assets[0].Kind != "video" || assets[0].Width != 512 || assets[0].Height != 512 || assets[0].DurationMS != 2000 || assets[1].ID != "asset-audio" {\n\t\tt.Fatalf("assets = %#v, want changing alpha VP9 plus audio harness asset", assets)\n\t}\n\tassertIsolatedPixelateFixture(t, validated)\n}\n''')
replace_once(path,
'''func TestParityPixelateAlphaSamplesAndRegionsStayFrameBound(t *testing.T) {\n\tsamples := ParityPixelateAlphaFrameSamples()\n\twantFrames := []int64{0, 15, 30, 59}\n\tassertPixelateFrames(t, samples, wantFrames)\n\n\tbounds := ParityPixelateAlphaRegionBounds()\n\twantBounds := (ParityBounds{MinX: 71, MinY: 94, MaxX: 474, MaxY: 401})\n\tif bounds != wantBounds {\n\t\tt.Fatalf("bounds = %#v, want %#v", bounds, wantBounds)\n\t}\n\tassertPixelateRegionFrames(t, samples, bounds, ParityPixelateAlphaRegionFrames(samples))\n}\n''',
'''func TestParityPixelateAlphaSamplesAndRegionsStayFrameBound(t *testing.T) {\n\tsamples := ParityPixelateAlphaFrameSamples()\n\twantFrames := []int64{0, 15, 30, 59}\n\tassertPixelateFrames(t, samples, wantFrames)\n\n\tbounds := ParityPixelateAlphaRegionBounds()\n\twantBounds := (ParityBounds{MinX: 71, MinY: 94, MaxX: 474, MaxY: 401})\n\tif bounds != wantBounds {\n\t\tt.Fatalf("bounds = %#v, want %#v", bounds, wantBounds)\n\t}\n\tassertPixelateRegionFrames(t, samples, bounds, ParityPixelateAlphaRegionFrames(samples))\n}\n\nfunc TestParityPixelateAlphaVideoSamplesAndRegionsStayFrameBound(t *testing.T) {\n\tsamples := ParityPixelateAlphaVideoFrameSamples()\n\twantFrames := []int64{0, 15, 30, 59}\n\tassertPixelateFrames(t, samples, wantFrames)\n\n\tbounds := ParityPixelateAlphaVideoRegionBounds()\n\twantBounds := (ParityBounds{MinX: 71, MinY: 94, MaxX: 474, MaxY: 401})\n\tif bounds != wantBounds {\n\t\tt.Fatalf("bounds = %#v, want %#v", bounds, wantBounds)\n\t}\n\tassertPixelateRegionFrames(t, samples, bounds, ParityPixelateAlphaVideoRegionFrames(samples))\n}\n''')

path = 'backend/cmd/video-pixelate-parity-fixture/main.go'
replace_once(path,
'''\tvariant := flag.String("variant", "opaque-png", "fixture variant: opaque-png, decoded-video, or alpha-png")\n''',
'''\tvariant := flag.String("variant", "opaque-png", "fixture variant: opaque-png, decoded-video, alpha-png, or alpha-video")\n''')
replace_once(path,
'''\tcase "alpha-png":\n\t\tname = video.ParityPixelateAlphaFixtureName\n\t\tdoc, assets = video.ParityPixelateAlphaFixture()\n\t\tsamples = video.ParityPixelateAlphaFrameSamples()\n\t\tframes = video.ParityPixelateAlphaRegionFrames(samples)\n\tdefault:\n\t\texitf("unknown --variant %q (want opaque-png, decoded-video, or alpha-png)", *variant)\n''',
'''\tcase "alpha-png":\n\t\tname = video.ParityPixelateAlphaFixtureName\n\t\tdoc, assets = video.ParityPixelateAlphaFixture()\n\t\tsamples = video.ParityPixelateAlphaFrameSamples()\n\t\tframes = video.ParityPixelateAlphaRegionFrames(samples)\n\tcase "alpha-video":\n\t\tname = video.ParityPixelateAlphaVideoFixtureName\n\t\tdoc, assets = video.ParityPixelateAlphaVideoFixture()\n\t\tsamples = video.ParityPixelateAlphaVideoFrameSamples()\n\t\tframes = video.ParityPixelateAlphaVideoRegionFrames(samples)\n\tdefault:\n\t\texitf("unknown --variant %q (want opaque-png, decoded-video, alpha-png, or alpha-video)", *variant)\n''')

path = 'backend/cmd/video-parity-fixture/main.go'
replace_once(path,
'''\t\t{"-f", "lavfi", "-i", "testsrc2=size=512x512:rate=1:duration=1", "-frames:v", "1", filepath.Join(mediaDir, "asset-square.png")},\n\t\t// A swept tone plus deterministic one-second impulses makes sample-offset\n''',
'''\t\t{"-f", "lavfi", "-i", "testsrc2=size=512x512:rate=1:duration=1", "-frames:v", "1", filepath.Join(mediaDir, "asset-square.png")},\n\t\t// Two moving partial-alpha regions over transparent RGB prove both frame\n\t\t// presentation identity and alpha preservation. WebM advertises\n\t\t// alpha_mode=1 even though ffprobe's native VP9 path reports yuv420p.\n\t\t{"-f", "lavfi", "-i", "nullsrc=s=512x512:r=30:d=2,format=rgba,geq=r='mod(X+N*7,256)':g='mod(Y+N*11,256)':b='mod(X+Y+N*13,256)':a='if(between(mod(X+N*5,512),60,240)*between(Y,70,230),160,if(between(mod(X+N*3,512),280,430)*between(Y,270,450),230,0))'", "-c:v", "libvpx-vp9", "-lossless", "1", "-pix_fmt", "yuva420p", "-auto-alt-ref", "0", "-metadata:s:v:0", "alpha_mode=1", filepath.Join(mediaDir, "asset-alpha-video.webm")},\n\t\t// A swept tone plus deterministic one-second impulses makes sample-offset\n''')

path = 'scripts/video-parity-capture.mjs'
replace_once(path,
'''    'asset-alpha': ['asset-alpha.png', 'image/png'],\n    'asset-audio': ['asset-audio.wav', 'audio/wav'],\n''',
'''    'asset-alpha': ['asset-alpha.png', 'image/png'],\n    'asset-alpha-video': ['asset-alpha-video.webm', 'video/webm'],\n    'asset-audio': ['asset-audio.wav', 'audio/wav'],\n''')

path = 'scripts/video-pixelate-parity-assert.mjs'
replace_once(path,
'''const decodedVideoFixture = fixture.name === 'parity-pixelate-decoded-video-v1';\nconst evidence = [];\n''',
'''const decodedVideoFixture = fixture.name === 'parity-pixelate-decoded-video-v1';\nconst alphaVideoFixture = fixture.name === 'parity-pixelate-alpha-video-v1';\nconst presentationVideoFixture = decodedVideoFixture || alphaVideoFixture;\nconst codecColorChannelTolerance = decodedVideoFixture ? 3 : (alphaVideoFixture ? 4 : null);\nconst evidence = [];\n''')
replace_once(path,
'''    const fallback = stage.querySelector('[data-preview-shape-painter-deferred="pixelate-css-approximation"]');\n    return {\n''',
'''    const fallback = stage.querySelector('[data-preview-shape-painter-deferred="pixelate-css-approximation"]');\n    const video = stage.querySelector('[data-video-preview-media="true"]');\n    return {\n''')
replace_once(path,
'''      css_fallback_marker_present: Boolean(fallback),\n    };\n''',
'''      css_fallback_marker_present: Boolean(fallback),\n      video_presentation_request_token: video?.dataset.videoPreviewPresentationRequestToken ?? null,\n      video_presentation_ready_token: video?.dataset.videoPreviewPresentationReadyToken ?? null,\n      video_presentation_status: video?.dataset.videoPreviewPresentationStatus ?? null,\n      video_presentation_media_time: video?.dataset.videoPreviewPresentationMediaTime ?? null,\n      video_presentation_attempts: video?.dataset.videoPreviewPresentationAttempts ?? null,\n    };\n''')
replace_once(path,
'''    if (decodedVideoFixture) {\n      if (presentations.length === 0) {\n''',
'''    if (presentationVideoFixture) {\n      if (state.video_presentation_status !== 'ready'\n        || !state.video_presentation_request_token\n        || state.video_presentation_ready_token !== state.video_presentation_request_token) {\n        throw new Error(`frame ${sample.frame_index} did not retain matching preview presentation-token proof: ${JSON.stringify(state)}`);\n      }\n      if (presentations.length === 0) {\n''')
replace_once(path,
'''    const codecRegion = decodedVideoFixture\n      ? await page.evaluate(async ({ frameIndex, snapshotID, canvasWidth }) => {\n''',
'''    const codecRegion = presentationVideoFixture\n      ? await page.evaluate(async ({ frameIndex, snapshotID, canvasWidth, channelTolerance }) => {\n''')
replace_once(path,
'''              if (delta > 3) pixelWithin = false;\n''',
'''              if (delta > channelTolerance) pixelWithin = false;\n''')
replace_once(path,
'''            channel_tolerance: 3,\n''',
'''            channel_tolerance: channelTolerance,\n''')
replace_once(path,
'''      }, { frameIndex: sample.frame_index, snapshotID: seedResult.snapshot_id, canvasWidth })\n      : null;\n\n    if (codecRegion && (codecRegion.max_channel_delta > 3 || codecRegion.pixel_pass_rate !== 1)) {\n      throw new Error(`frame ${sample.frame_index} exceeded decoded H.264 ±3 RGB gate: ${JSON.stringify(codecRegion)}`);\n    }\n''',
'''      }, {\n        frameIndex: sample.frame_index,\n        snapshotID: seedResult.snapshot_id,\n        canvasWidth,\n        channelTolerance: codecColorChannelTolerance,\n      })\n      : null;\n\n    if (codecRegion && (codecRegion.max_channel_delta > codecColorChannelTolerance || codecRegion.pixel_pass_rate !== 1)) {\n      const sourceKind = alphaVideoFixture ? 'transparent VP9' : 'decoded H.264';\n      throw new Error(`frame ${sample.frame_index} exceeded ${sourceKind} ±${codecColorChannelTolerance} RGB gate: ${JSON.stringify(codecRegion)}`);\n    }\n''')
replace_once(path,
'''  decoded_frame_identity_gate: decodedVideoFixture,\n  codec_color_channel_tolerance: decodedVideoFixture ? 3 : null,\n''',
'''  decoded_frame_identity_gate: presentationVideoFixture,\n  transparent_video_alpha_gate: alphaVideoFixture,\n  codec_color_channel_tolerance: codecColorChannelTolerance,\n''')

print('alpha-video fixture patch applied')
