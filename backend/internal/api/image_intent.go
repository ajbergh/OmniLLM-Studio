package api

import "strings"

// ImageIntent represents the result of detecting whether a user message
// is requesting image generation.
type ImageIntent struct {
	RequiresImageGeneration bool
	Prompt                  string // cleaned prompt to pass to the image generator
	Confidence              float64
	Reason                  string
}

// detectImageIntent uses keyword-based heuristics to determine if the user
// is requesting image generation. It is intentionally conservative to avoid
// false positives on normal conversational text.
func detectImageIntent(content string) ImageIntent {
	lower := strings.ToLower(strings.TrimSpace(content))

	// Non-image phrases: talking about existing images, not generating new ones.
	nonImagePhrases := []string{
		"what is an image",
		"describe the image",
		"describe this image",
		"analyze this image",
		"analyze the image",
		"explain this image",
		"explain the image",
		"look at this image",
		"look at the image",
		"in this image",
		"the image shows",
		"can you see the image",
		"do you see the image",
		"image processing",
		"image recognition",
		"image file",
		"image format",
		"image size",
		"image url",
		"how to create an image",
		"what does an image",
		"why is the image",
		"when was the image",
		"who is in the image",
		"what's in the image",
		"what is in the image",
	}
	for _, phrase := range nonImagePhrases {
		if strings.Contains(lower, phrase) {
			return ImageIntent{Confidence: 0.05, Reason: "non_image_pattern"}
		}
	}

	// Strong trigger phrases — unambiguous generation intent.
	// Each entry is (phrase-to-match, prefix-to-strip-for-prompt).
	// Longer / more-specific phrases come first so they take precedence.
	type triggerEntry struct {
		phrase string
		prefix string
	}
	triggers := []triggerEntry{
		// "generate … image …"
		{"generate an image of ", "generate an image of "},
		{"generate a image of ", "generate a image of "},
		{"generate image of ", "generate image of "},
		{"generate an image:", "generate an image:"},
		{"generate a image:", "generate a image:"},
		{"generate an image", "generate an image"},
		{"generate a image", "generate a image"},
		{"generate image", "generate image"},
		// "generate … picture …"
		{"generate a picture of ", "generate a picture of "},
		{"generate a picture:", "generate a picture:"},
		{"generate a picture", "generate a picture"},
		{"generate an picture", "generate an picture"},
		// "generate … photo …"
		{"generate a photo of ", "generate a photo of "},
		{"generate a photo:", "generate a photo:"},
		{"generate a photo", "generate a photo"},
		{"generate an photo", "generate an photo"},
		// "create … image …"
		{"create an image of ", "create an image of "},
		{"create a image of ", "create a image of "},
		{"create an image:", "create an image:"},
		{"create a image:", "create a image:"},
		{"create an image", "create an image"},
		{"create a image", "create a image"},
		// "create … picture / photo …"
		{"create a picture of ", "create a picture of "},
		{"create a picture", "create a picture"},
		{"create a photo of ", "create a photo of "},
		{"create a photo:", "create a photo:"},
		{"create a photo", "create a photo"},
		// "draw …"
		{"draw me a ", "draw me a "},
		{"draw me an ", "draw me an "},
		{"draw a picture of ", "draw a picture of "},
		{"draw an image of ", "draw an image of "},
		{"draw a ", "draw a "},
		{"draw an ", "draw an "},
		// "make … image / picture / photo …"
		{"make an image of ", "make an image of "},
		{"make a image of ", "make a image of "},
		{"make me an image of ", "make me an image of "},
		{"make me a picture of ", "make me a picture of "},
		{"make a picture of ", "make a picture of "},
		{"make a picture", "make a picture"},
		{"make me a picture", "make me a picture"},
		{"make a photo", "make a photo"},
		// "paint / illustrate / render / produce …"
		{"paint a ", "paint a "},
		{"paint an ", "paint an "},
		{"design an image of ", "design an image of "},
		{"illustrate ", "illustrate "},
		{"render an image of ", "render an image of "},
		{"render a image of ", "render a image of "},
		{"produce an image of ", "produce an image of "},
		// "show me … / give me …"
		{"show me an image of ", "show me an image of "},
		{"show me a picture of ", "show me a picture of "},
		{"show me a photo of ", "show me a photo of "},
		{"show me a picture", "show me a picture"},
		{"give me an image of ", "give me an image of "},
		{"give me a picture of ", "give me a picture of "},
		{"give me a photo of ", "give me a photo of "},
		{"give me a picture", "give me a picture"},
	}
	for _, t := range triggers {
		if strings.Contains(lower, t.phrase) {
			prompt := buildImagePrompt(content, lower, t.prefix)
			return ImageIntent{
				RequiresImageGeneration: true,
				Prompt:                  prompt,
				Confidence:              0.95,
				Reason:                  "trigger_phrase",
			}
		}
	}

	return ImageIntent{Confidence: 0.1, Reason: "no_signal"}
}

// buildImagePrompt extracts the image description from the user's message.
// It first tries to strip a leading imperative prefix (e.g. "generate an image of "),
// falling back to the full content if nothing matches.
func buildImagePrompt(content, lower, prefix string) string {
	// Find where the prefix ends in the lowercased string, then use the
	// same offset in the original-case content so we preserve capitalisation.
	idx := strings.Index(lower, prefix)
	if idx >= 0 {
		tail := strings.TrimSpace(content[idx+len(prefix):])
		// Remove trailing punctuation like "." or "!"
		tail = strings.TrimRight(tail, ".!?")
		if tail != "" {
			return tail
		}
	}
	return strings.TrimSpace(content)
}
