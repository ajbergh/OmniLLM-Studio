package music

import (
	"fmt"
	"strings"
)

func AssemblePrompt(req GenerateRequest) string {
	var lines []string
	duration := strings.TrimSpace(req.Options.Duration)
	genre := strings.TrimSpace(req.Options.Genre)
	mood := strings.TrimSpace(req.Options.Mood)

	opening := "Create a music track"
	if duration != "" || genre != "" {
		parts := []string{"Create"}
		if duration != "" {
			parts = append(parts, duration)
		}
		if genre != "" {
			parts = append(parts, genre)
		}
		parts = append(parts, "track")
		opening = strings.Join(parts, " ")
	}
	if mood != "" {
		opening += " with " + mood + " mood"
	}
	lines = append(lines, strings.TrimSpace(opening)+".")

	appendLine := func(label, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, fmt.Sprintf("%s: %s.", label, value))
		}
	}
	if len(req.Options.Instruments) > 0 {
		appendLine("Instrumentation", strings.Join(req.Options.Instruments, ", "))
	}
	appendLine("Era", req.Options.Era)
	if req.Options.BPM != nil {
		appendLine("Tempo", fmt.Sprintf("%d BPM", *req.Options.BPM))
	}
	appendLine("Key/scale", req.Options.Scale)
	appendLine("Structure", req.Options.Structure)
	appendLine("Energy curve", req.Options.EnergyCurve)

	vocalMode := strings.TrimSpace(req.VocalMode)
	if vocalMode == "" {
		if req.Instrumental {
			vocalMode = "Instrumental only, no vocals"
		} else {
			vocalMode = "Auto"
		}
	}
	appendLine("Vocals", vocalMode)
	appendLine("Language", req.Options.Language)

	if lyrics := strings.TrimSpace(req.Lyrics); lyrics != "" {
		lines = append(lines, "Lyrics:")
		lines = append(lines, lyrics)
	}
	if notes := strings.TrimSpace(req.Options.ProductionNotes); notes != "" {
		lines = append(lines, "Production notes:")
		lines = append(lines, notes)
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		lines = append(lines, "User direction:")
		lines = append(lines, prompt)
	}
	if avoid := strings.TrimSpace(req.Options.NegativeSteer); avoid != "" {
		lines = append(lines, "Avoid: "+avoid+".")
	}
	if req.Instrumental && !strings.Contains(strings.ToLower(strings.Join(lines, "\n")), "no vocals") {
		lines = append(lines, "Instrumental only, no vocals.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func DeriveTitle(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "Untitled Track"
	}
	replacer := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	title := strings.Join(strings.Fields(replacer.Replace(prompt)), " ")
	if len([]rune(title)) > 48 {
		r := []rune(title)
		title = string(r[:48])
		title = strings.TrimSpace(title)
	}
	return title
}
