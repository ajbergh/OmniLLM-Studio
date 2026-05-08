package artifacts

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var unsafeCharsRe = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// SafeFilename normalises name and appends the correct extension for format.
// If name is empty a timestamped default is used.
func SafeFilename(name string, format ArtifactFormat) string {
	ext := "." + ExtensionForFormat(format)
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Sprintf("document-%s%s", time.Now().UTC().Format("20060102-150405"), ext)
	}

	name = unsafeCharsRe.ReplaceAllString(name, "-")
	name = strings.ReplaceAll(name, " ", "-")

	// Collapse repeated dashes
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "document"
	}

	// Truncate to 60 chars before extension
	if len(name) > 60 {
		name = name[:60]
		name = strings.TrimRight(name, "-")
	}

	// Strip any existing extension and add the correct one
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		candidate := name[idx:]
		// only strip if it looks like a known extension
		known := []string{".docx", ".xlsx", ".csv", ".pdf", ".md", ".html", ".json", ".yaml", ".yml", ".txt"}
		for _, k := range known {
			if strings.EqualFold(candidate, k) {
				name = name[:idx]
				break
			}
		}
	}

	return strings.ToLower(name) + ext
}

// SuggestFilename derives a meaningful base name from the user message.
// It strips format-intent phrases to isolate the subject, then takes the
// first 6 words as a slug. The correct extension is handled by SafeFilename.
func SuggestFilename(userMsg string, format ArtifactFormat) string {
	lower := strings.ToLower(userMsg)

	// Strip format-intent phrases
	strippers := []string{
		" as excel", " in excel", " to excel", " excel file", " excel spreadsheet",
		" as xlsx", " export as xlsx", " .xlsx",
		" as csv", " export as csv", " csv file", " comma-separated",
		" as pdf", " as a pdf", " export as pdf", " pdf file",
		" as markdown", " export as markdown", " as md", " markdown file",
		" as html", " export as html", " html file",
		" as json", " export as json", " json file",
		" as yaml", " export as yaml", " yaml file",
		" as yml", " .yaml", " .yml",
		" to a file", " to file", " as a file",
		" put this in", " put it in",
		" make this a", " make this an",
		" export as", " export to",
		" save as", " save to",
		" return as", " output as",
	}
	for _, s := range strippers {
		lower = strings.ReplaceAll(lower, s, " ")
	}

	// Strip leading filler phrases
	fillers := []string{
		"please ", "can you ", "could you ", "i need ", "i want ",
		"write me ", "write a ", "write an ", "write the ",
		"create me ", "create a ", "create an ", "create the ",
		"generate me ", "generate a ", "generate an ", "generate the ",
		"make me ", "make a ", "make an ", "make the ",
		"draft me ", "draft a ", "draft an ", "draft the ",
		"produce a ", "produce an ", "produce the ",
		"give me a ", "give me an ", "give me the ", "give me ",
		"turn this into a ", "turn this into an ",
		"convert this to ", "convert this into ",
	}
	for _, f := range fillers {
		if strings.HasPrefix(lower, f) {
			lower = lower[len(f):]
			break
		}
	}

	words := strings.Fields(lower)
	if len(words) == 0 {
		return SafeFilename("", format)
	}
	if len(words) > 6 {
		words = words[:6]
	}
	return SafeFilename(strings.Join(words, "-"), format)
}
