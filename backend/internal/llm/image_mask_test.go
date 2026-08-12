package llm

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngRef(t *testing.T, width, height int, transparent bool) ReferenceImage {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	if transparent && width > 0 && height > 0 {
		img.SetNRGBA(0, 0, color.NRGBA{A: 0})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return ReferenceImage{Data: base64.StdEncoding.EncodeToString(buf.Bytes()), MimeType: "image/png"}
}

func TestValidatePixelMaskPairAcceptsMatchingPNG(t *testing.T) {
	source := pngRef(t, 8, 6, false)
	mask := pngRef(t, 8, 6, true)
	if err := validatePixelMaskPair("gpt-image-2", source, mask); err != nil {
		t.Fatalf("validatePixelMaskPair: %v", err)
	}
}

func TestValidatePixelMaskPairRejectsDimensionMismatch(t *testing.T) {
	source := pngRef(t, 8, 6, false)
	mask := pngRef(t, 6, 8, true)
	if err := validatePixelMaskPair("gpt-image-2", source, mask); err == nil {
		t.Fatal("expected dimension mismatch to fail")
	}
}

func TestValidatePixelMaskPairRejectsMaskWithoutTransparency(t *testing.T) {
	source := pngRef(t, 8, 6, false)
	mask := pngRef(t, 8, 6, false)
	if err := validatePixelMaskPair("gpt-image-2", source, mask); err == nil {
		t.Fatal("expected opaque mask to fail")
	}
}

func TestWebPDimensionsVP8X(t *testing.T) {
	data := make([]byte, 30)
	copy(data[0:4], "RIFF")
	copy(data[8:12], "WEBP")
	copy(data[12:16], "VP8X")
	// Canvas is stored as dimension - 1 in three little-endian bytes.
	widthMinusOne := 639
	heightMinusOne := 479
	data[24] = byte(widthMinusOne)
	data[25] = byte(widthMinusOne >> 8)
	data[26] = byte(widthMinusOne >> 16)
	data[27] = byte(heightMinusOne)
	data[28] = byte(heightMinusOne >> 8)
	data[29] = byte(heightMinusOne >> 16)
	w, h, err := webPDimensions(data)
	if err != nil {
		t.Fatal(err)
	}
	if w != 640 || h != 480 {
		t.Fatalf("dimensions = %dx%d, want 640x480", w, h)
	}
}
