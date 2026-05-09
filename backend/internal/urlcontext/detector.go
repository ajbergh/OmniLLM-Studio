package urlcontext

import (
	"regexp"
	"strings"
)

// urlPattern matches http(s) URLs in text, including Markdown and angle-bracket forms.
var urlPattern = regexp.MustCompile(`(?i)(?:<(https?://[^\s>]+)>|\[(?:[^\]]*)\]\((https?://[^\s)]+)\)|(https?://[^\s<>"'\)\]\}\,\;\:]+))`)

// trailingPunct is stripped from the end of bare URLs (not Markdown or angle-bracket forms).
var trailingPunct = strings.NewReplacer(
	".", "", ",", "", ";", "", ":", "", ")", "", "]", "", "}", "", `"`, "", "'", "",
)

// ExtractURLs finds all http/https URLs in a message, deduplicates them, and caps
// at maxURLs. It handles bare URLs, Markdown [text](url), and <url> angle-bracket forms.
func ExtractURLs(message string, maxURLs int) []string {
	seen := make(map[string]struct{})
	var out []string

	matches := urlPattern.FindAllStringSubmatch(message, -1)
	for _, m := range matches {
		var raw string
		switch {
		case m[1] != "": // angle-bracket <url>
			raw = m[1]
		case m[2] != "": // Markdown [text](url)
			raw = m[2]
		case m[3] != "": // bare URL
			raw = stripTrailingPunct(m[3])
		}
		if raw == "" {
			continue
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
		if maxURLs > 0 && len(out) >= maxURLs {
			break
		}
	}
	return out
}

// stripTrailingPunct removes common trailing punctuation from bare URLs.
func stripTrailingPunct(s string) string {
	for {
		if len(s) == 0 {
			break
		}
		last := s[len(s)-1]
		if last == '.' || last == ',' || last == ';' || last == ':' ||
			last == ')' || last == ']' || last == '}' ||
			last == '"' || last == '\'' {
			s = s[:len(s)-1]
		} else {
			break
		}
	}
	return s
}
