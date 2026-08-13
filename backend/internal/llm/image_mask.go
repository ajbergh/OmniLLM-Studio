package llm

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// validatePixelMaskPair verifies that an exact pixel mask is usable with its
// source image before provider dispatch. Image Studio emits PNG masks with
// transparent pixels for the selected edit region.
func validatePixelMaskPair(model string, source, mask ReferenceImage) error {
	if strings.TrimSpace(source.Data) == "" {
		return fmt.Errorf("pixel mask requires a base image")
	}
	if strings.TrimSpace(mask.Data) == "" {
		return fmt.Errorf("pixel mask data is empty")
	}

	sourceBytes, err := base64.StdEncoding.DecodeString(source.Data)
	if err != nil {
		return fmt.Errorf("decode base image: %w", err)
	}
	maskBytes, err := base64.StdEncoding.DecodeString(mask.Data)
	if err != nil {
		return fmt.Errorf("decode mask image: %w", err)
	}

	sourceW, sourceH, err := rasterDimensions(sourceBytes, source.MimeType)
	if err != nil {
		return fmt.Errorf("inspect base image: %w", err)
	}
	maskW, maskH, err := rasterDimensions(maskBytes, mask.MimeType)
	if err != nil {
		return fmt.Errorf("inspect mask image: %w", err)
	}
	if sourceW != maskW || sourceH != maskH {
		return fmt.Errorf("mask dimensions %dx%d must match base image dimensions %dx%d", maskW, maskH, sourceW, sourceH)
	}

	if !strings.EqualFold(strings.TrimSpace(mask.MimeType), "image/png") {
		return fmt.Errorf("pixel mask must be PNG, got %q", mask.MimeType)
	}
	if err := validateMaskHasTransparentSelection(maskBytes); err != nil {
		return err
	}

	// DALL-E 2's edit endpoint is stricter than GPT Image models. Keep its
	// historical PNG requirement explicit instead of allowing an opaque
	// provider-side format error.
	if strings.EqualFold(strings.TrimSpace(model), "dall-e-2") && !strings.EqualFold(strings.TrimSpace(source.MimeType), "image/png") {
		return fmt.Errorf("dall-e-2 masked edits require a PNG base image")
	}
	return nil
}

func validateMaskHasTransparentSelection(data []byte) error {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode PNG mask: %w", err)
	}
	if format != "png" {
		return fmt.Errorf("pixel mask must decode as PNG")
	}
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xffff {
				return nil
			}
		}
	}
	return fmt.Errorf("pixel mask contains no transparent edit region")
}

func rasterDimensions(data []byte, mimeType string) (int, int, error) {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "image/webp" || looksLikeWebP(data) {
		return webPDimensions(data)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("invalid image dimensions")
	}
	return cfg.Width, cfg.Height, nil
}

func looksLikeWebP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// webPDimensions reads the canvas size from VP8, VP8L, or VP8X headers without
// adding a CGO/native decoder dependency. Only dimensions are needed for mask
// validation; the original WebP bytes continue to the provider unchanged.
func webPDimensions(data []byte) (int, int, error) {
	if !looksLikeWebP(data) || len(data) < 30 {
		return 0, 0, fmt.Errorf("invalid WebP header")
	}
	chunk := string(data[12:16])
	switch chunk {
	case "VP8X":
		if len(data) < 30 {
			return 0, 0, fmt.Errorf("truncated VP8X header")
		}
		w := 1 + int(data[24]) + int(data[25])<<8 + int(data[26])<<16
		h := 1 + int(data[27]) + int(data[28])<<8 + int(data[29])<<16
		return w, h, nil
	case "VP8L":
		if len(data) < 25 || data[20] != 0x2f {
			return 0, 0, fmt.Errorf("invalid VP8L header")
		}
		b1, b2, b3, b4 := data[21], data[22], data[23], data[24]
		w := 1 + int(b1) + (int(b2)&0x3f)<<8
		h := 1 + (int(b2) >> 6) + int(b3)<<2 + (int(b4)&0x0f)<<10
		return w, h, nil
	case "VP8 ":
		if len(data) < 30 || data[23] != 0x9d || data[24] != 0x01 || data[25] != 0x2a {
			return 0, 0, fmt.Errorf("invalid VP8 frame header")
		}
		w := int(binary.LittleEndian.Uint16(data[26:28]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(data[28:30]) & 0x3fff)
		if w == 0 || h == 0 {
			return 0, 0, fmt.Errorf("invalid VP8 dimensions")
		}
		return w, h, nil
	default:
		return 0, 0, fmt.Errorf("unsupported WebP chunk %q", chunk)
	}
}
