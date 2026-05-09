package urlcontext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reTitle      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reScript     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reScriptLD   = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	reStyle      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reComment    = regexp.MustCompile(`(?is)<!--.*?-->`)
	reMeta       = regexp.MustCompile(`(?is)<meta\s([^>]+?)(?:/>|>)`)
	reAttr       = regexp.MustCompile(`(?i)([\w:-]+)\s*=\s*(?:"([^"]*?)"|'([^']*?)')`)
	reMultiSpace = regexp.MustCompile(`[ \t]{2,}`)
	reMultiLine  = regexp.MustCompile(`\n{3,}`)
	reHTMLEntity = regexp.MustCompile(`&[a-zA-Z]+;|&#[0-9]+;|&#x[0-9a-fA-F]+;`)
)

var htmlEntities = map[string]string{
	"&amp;":    "&",
	"&lt;":     "<",
	"&gt;":     ">",
	"&quot;":   `"`,
	"&apos;":   "'",
	"&nbsp;":   " ",
	"&mdash;":  "—",
	"&ndash;":  "–",
	"&hellip;": "…",
}

// ExtractReadable strips HTML and returns the page title and readable text.
// It extracts JSON-LD structured data and Open Graph meta tags before stripping
// scripts, so news sites and articles that embed machine-readable content yield
// more useful text even when the rendered page is JavaScript-gated.
func ExtractReadable(rawURL string, html []byte, contentType string) *ReadableDocument {
	if len(html) == 0 {
		return &ReadableDocument{FinalURL: rawURL}
	}

	content := string(html)

	// Extract title from <title> tag
	title := ""
	if m := reTitle.FindStringSubmatch(content); len(m) > 1 {
		title = cleanText(reTag.ReplaceAllString(m[1], ""))
	}

	// Extract structured content from JSON-LD and meta tags BEFORE stripping scripts.
	structured := extractJSONLD(content)
	ogTitle, ogDesc := extractMetaContent(content)

	// Prefer og:title over <title> when it's more descriptive
	if ogTitle != "" && title == "" {
		title = ogTitle
	}

	// Remove script, style, comments (after JSON-LD extraction)
	content = reScript.ReplaceAllString(content, " ")
	content = reStyle.ReplaceAllString(content, " ")
	content = reComment.ReplaceAllString(content, " ")

	// Replace block-level tags with newlines for better paragraph separation
	content = replaceBlockTags(content)

	// Strip remaining tags
	content = reTag.ReplaceAllString(content, " ")

	// Decode HTML entities
	content = decodeEntities(content)

	// Normalize whitespace
	content = reMultiSpace.ReplaceAllString(content, " ")
	content = normalizeLines(content)
	content = reMultiLine.ReplaceAllString(content, "\n\n")
	content = strings.TrimSpace(content)

	// Combine structured content + meta description + body text.
	// If JSON-LD gave us good content, prepend it so it's the primary signal.
	var parts []string
	if structured != "" {
		parts = append(parts, structured)
	}
	if ogDesc != "" {
		// Only add meta description if not already covered by JSON-LD
		if !strings.Contains(structured, ogDesc[:min(len(ogDesc), 60)]) {
			parts = append(parts, "Page description: "+ogDesc)
		}
	}
	if content != "" {
		parts = append(parts, content)
	}
	combined := strings.Join(parts, "\n\n")

	return &ReadableDocument{
		Title:       title,
		Text:        combined,
		Markdown:    combined,
		Description: ogDesc,
		FinalURL:    rawURL,
	}
}

// extractJSONLD finds application/ld+json script blocks and extracts article content.
func extractJSONLD(content string) string {
	matches := reScriptLD.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return ""
	}

	var parts []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		jsonText := strings.TrimSpace(m[1])
		if jsonText == "" {
			continue
		}
		var data interface{}
		if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
			continue
		}
		text := extractFromLDData(data)
		if text != "" {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "\n\n")
}

func extractFromLDData(data interface{}) string {
	switch v := data.(type) {
	case map[string]interface{}:
		if graph, ok := v["@graph"]; ok {
			return extractFromLDData(graph)
		}
		return extractFromLDObject(v)
	case []interface{}:
		var parts []string
		for _, item := range v {
			text := extractFromLDData(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func extractFromLDObject(obj map[string]interface{}) string {
	typeVal, _ := obj["@type"].(string)

	switch typeVal {
	case "NewsArticle", "Article", "BlogPosting", "TechArticle", "ScholarlyArticle":
		return extractArticleLD(obj)
	case "WebPage":
		return extractWebPageLD(obj)
	case "ItemList":
		return extractItemListLD(obj)
	case "VideoObject":
		name, _ := obj["name"].(string)
		desc, _ := obj["description"].(string)
		if name != "" || desc != "" {
			return fmt.Sprintf("Video: %s\n%s", name, desc)
		}
	}
	return ""
}

func extractArticleLD(obj map[string]interface{}) string {
	var sb strings.Builder

	if headline, ok := obj["headline"].(string); ok && headline != "" {
		fmt.Fprintf(&sb, "# %s\n\n", headline)
	}

	if desc, ok := obj["description"].(string); ok && desc != "" {
		fmt.Fprintf(&sb, "%s\n\n", desc)
	}

	switch a := obj["author"].(type) {
	case string:
		if a != "" {
			fmt.Fprintf(&sb, "By %s\n\n", a)
		}
	case map[string]interface{}:
		if name, ok := a["name"].(string); ok && name != "" {
			fmt.Fprintf(&sb, "By %s\n\n", name)
		}
	case []interface{}:
		var names []string
		for _, item := range a {
			if m, ok := item.(map[string]interface{}); ok {
				if name, ok := m["name"].(string); ok && name != "" {
					names = append(names, name)
				}
			}
		}
		if len(names) > 0 {
			fmt.Fprintf(&sb, "By %s\n\n", strings.Join(names, ", "))
		}
	}

	if date, ok := obj["datePublished"].(string); ok && date != "" {
		fmt.Fprintf(&sb, "Published: %s\n\n", date)
	}

	if body, ok := obj["articleBody"].(string); ok && body != "" {
		sb.WriteString(body)
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String())
}

func extractWebPageLD(obj map[string]interface{}) string {
	name, _ := obj["name"].(string)
	desc, _ := obj["description"].(string)
	if name == "" && desc == "" {
		return ""
	}
	if desc != "" {
		return name + "\n" + desc
	}
	return name
}

func extractItemListLD(obj map[string]interface{}) string {
	items, ok := obj["itemListElement"].([]interface{})
	if !ok || len(items) == 0 {
		return ""
	}
	var headlines []string
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if name != "" && len(name) > 20 { // Skip very short nav items
			headlines = append(headlines, "- "+name)
		}
	}
	if len(headlines) == 0 {
		return ""
	}
	return "## Article List\n\n" + strings.Join(headlines, "\n")
}

// extractMetaContent finds og:title, og:description, and name=description meta tags.
func extractMetaContent(content string) (title, description string) {
	for _, m := range reMeta.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		attrs := parseAttrs(m[1])
		prop := strings.ToLower(attrs["property"])
		name := strings.ToLower(attrs["name"])
		val := attrs["content"]
		if val == "" {
			continue
		}
		val = decodeEntities(val)
		switch prop {
		case "og:title":
			if title == "" {
				title = val
			}
		case "og:description":
			if description == "" {
				description = val
			}
		}
		switch name {
		case "description":
			if description == "" {
				description = val
			}
		case "twitter:description":
			if description == "" {
				description = val
			}
		}
	}
	return
}

// parseAttrs extracts key=value attribute pairs from an HTML tag's attribute string.
func parseAttrs(s string) map[string]string {
	attrs := make(map[string]string)
	for _, m := range reAttr.FindAllStringSubmatch(s, -1) {
		if len(m) < 3 {
			continue
		}
		key := strings.ToLower(m[1])
		val := m[2]
		if val == "" {
			val = m[3]
		}
		attrs[key] = val
	}
	return attrs
}

// IsNavigationOnly returns true when extracted text looks like a navigation skeleton —
// many short lines typical of JS-rendered homepages where only the shell was served.
// This does NOT trigger when JSON-LD or other structured data provided real content,
// since those are prepended and push the total length above the thresholds.
func IsNavigationOnly(text string) bool {
	if len(text) > 3000 {
		return false
	}
	lines := strings.Split(text, "\n")
	var nonEmpty []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) < 5 {
		return false
	}
	substantive := 0
	for _, l := range nonEmpty {
		if len(l) > 100 {
			substantive++
		}
	}
	return substantive < 2
}

// replaceBlockTags replaces common block-level HTML tags with newlines.
func replaceBlockTags(s string) string {
	blockTags := []string{
		"</p>", "</div>", "</article>", "</section>",
		"</h1>", "</h2>", "</h3>", "</h4>", "</h5>", "</h6>",
		"</li>", "</tr>", "</td>", "</th>",
		"<br>", "<br/>", "<br />",
		"</blockquote>", "</pre>",
	}
	for _, tag := range blockTags {
		s = strings.ReplaceAll(strings.ToLower(s), tag, tag+"\n")
	}
	return s
}

// decodeEntities replaces common HTML entities.
func decodeEntities(s string) string {
	return reHTMLEntity.ReplaceAllStringFunc(s, func(entity string) string {
		if v, ok := htmlEntities[entity]; ok {
			return v
		}
		return " "
	})
}

// normalizeLines trims each line and removes blank-only lines beyond 2 consecutive.
func normalizeLines(s string) string {
	var buf bytes.Buffer
	lines := strings.Split(s, "\n")
	blank := 0
	for _, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if line == "" {
			blank++
			if blank <= 1 {
				buf.WriteString("\n")
			}
		} else {
			blank = 0
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

// cleanText trims whitespace and collapses internal spaces.
func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = reMultiSpace.ReplaceAllString(s, " ")
	return s
}

// IsBinaryContent returns true if the byte slice appears to be binary (not text).
func IsBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	nullCount := 0
	for _, b := range sample {
		if b == 0 {
			nullCount++
		}
	}
	return nullCount*100/len(sample) > 1
}
