package news

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

// FormatNewspaperEdition formats a set of stories into a newspaper-style Markdown string.
func FormatNewspaperEdition(input EditionInput) string {
	if len(input.Stories) == 0 {
		return formatNoResults(input)
	}

	var b strings.Builder

	title := "The OmniLLM Daily"
	if input.Intent.IntentType == NewsIntentFrontPage {
		title = "The OmniLLM Front Page"
	}

	b.WriteString("# " + title)
	b.WriteString("\n\n")

	// Topic and dateline
	topicLine := "Non-sports news edition"
	if slug := input.Intent.IssueSlug; slug != "" {
		topicLine = formatIssueDisplayName(slug) + " Edition"
	}

	dateline := input.GeneratedAt.Format("January 2, 2006")
	b.WriteString(fmt.Sprintf("**%s** · Actually Relevant News · %s", topicLine, dateline))
	b.WriteString("\n\n---\n\n")

	// Lead story (first story)
	lead := input.Stories[0]
	writeLeadStory(&b, lead)

	// More Headlines (next 3-5 stories)
	if len(input.Stories) > 1 {
		end := 5
		if len(input.Stories) < end+1 {
			end = len(input.Stories)
		}
		moreStories := input.Stories[1:end]

		b.WriteString("## More Headlines\n\n")
		for _, s := range moreStories {
			writeHeadline(&b, s)
		}
	}

	// News Briefs (6+ stories)
	if len(input.Stories) > 5 {
		briefs := input.Stories[5:]
		b.WriteString("## News Briefs\n\n")
		for _, s := range briefs {
			writeBrief(&b, s)
		}
	}

	// Broadened note
	if input.Broadened {
		b.WriteString("---\n\n")
		b.WriteString("*No exact match was found, so I broadened the edition to the closest current curated stories.*")
		b.WriteString("\n\n")
	}

	// Source notes
	b.WriteString("---\n\n")
	b.WriteString("## Source Notes\n\n")
	b.WriteString("This edition was generated from Actually Relevant's public curated news API. ")
	b.WriteString("Links point to the original source URLs returned by the API.")
	b.WriteString("\n\n")

	return b.String()
}

// formatNoResults returns a message when no stories are found.
func formatNoResults(input EditionInput) string {
	var b strings.Builder
	b.WriteString("# No matching curated stories found\n\n")
	b.WriteString("I checked Actually Relevant for this topic, but it did not return matching published stories.\n\n")
	b.WriteString("Try a broader query such as \"latest technology news\" or \"top global headlines.\"")
	return b.String()
}

// writeLeadStory writes the lead story section.
func writeLeadStory(b *strings.Builder, s Story) {
	b.WriteString("## Lead Story\n\n")

	title := bestTitle(s)
	link := safeMarkdownLink(title, s.SourceURL)
	b.WriteString("### " + link)
	b.WriteString("\n\n")

	// Metadata line
	meta := formatStoryMeta(s)
	if meta != "" {
		b.WriteString("*" + meta + "*")
		b.WriteString("\n\n")
	}

	// Summary
	if s.Summary != "" {
		b.WriteString(escapeText(s.Summary))
		b.WriteString("\n\n")
	}

	// Quote
	if s.Quote != "" {
		attribution := ""
		if s.QuoteAttribution != "" {
			attribution = " — " + escapeText(s.QuoteAttribution)
		}
		b.WriteString(fmt.Sprintf("> %s%s\n\n", escapeText(s.Quote), attribution))
	}

	// Why it matters
	whyMatters := bestWhyMatters(s)
	if whyMatters != "" {
		b.WriteString("**Why it matters:** " + escapeText(whyMatters))
		b.WriteString("\n\n")
	}
}

// writeHeadline writes a single headline entry.
func writeHeadline(b *strings.Builder, s Story) {
	title := bestTitle(s)
	link := safeMarkdownLink(title, s.SourceURL)
	b.WriteString("### " + link)
	b.WriteString("\n")

	meta := formatStoryMeta(s)
	if meta != "" {
		b.WriteString("*" + meta + "*")
		b.WriteString("\n")
	}

	if s.Summary != "" {
		// Use first sentence or first 150 chars
		summary := truncateSummary(s.Summary, 150)
		b.WriteString(escapeText(summary))
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n")
	}
}

// writeBrief writes a compact news brief entry.
func writeBrief(b *strings.Builder, s Story) {
	title := bestTitle(s)
	link := safeMarkdownLink(title, s.SourceURL)

	summary := ""
	if s.Summary != "" {
		summary = truncateSummary(s.Summary, 120)
	}

	if summary != "" {
		b.WriteString(fmt.Sprintf("- **%s:** %s\n", link, escapeText(summary)))
	} else {
		b.WriteString(fmt.Sprintf("- **%s**\n", link))
	}
}

// bestTitle returns the best available title for a story.
func bestTitle(s Story) string {
	if s.TitleLabel != "" {
		return s.TitleLabel
	}
	if s.Title != "" {
		return s.Title
	}
	if s.SourceTitle != "" {
		return s.SourceTitle
	}
	return "Untitled Story"
}

// formatStoryMeta formats the metadata line for a story.
func formatStoryMeta(s Story) string {
	var parts []string

	if s.Issue != nil && s.Issue.Name != "" {
		parts = append(parts, s.Issue.Name)
	}

	if s.Relevance != nil {
		parts = append(parts, fmt.Sprintf("Relevance %d/10", *s.Relevance))
	} else if s.RelevancePre != nil {
		parts = append(parts, fmt.Sprintf("Relevance %d/10", *s.RelevancePre))
	}

	if s.DatePublished != nil && !s.DatePublished.IsZero() {
		parts = append(parts, s.DatePublished.Format("Jan 2, 2006"))
	} else if s.DateCrawled != nil && !s.DateCrawled.IsZero() {
		parts = append(parts, "Published "+s.DateCrawled.Format("Jan 2, 2006"))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

// bestWhyMatters returns the best "why it matters" text.
func bestWhyMatters(s Story) string {
	if s.RelevanceSummary != "" {
		return s.RelevanceSummary
	}
	if s.RelevanceReasons != "" {
		return s.RelevanceReasons
	}
	return ""
}

// safeMarkdownLink creates a Markdown link if the URL is safe, otherwise returns plain text.
func safeMarkdownLink(text, rawURL string) string {
	escaped := escapeText(text)

	if rawURL == "" {
		return escaped
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return escaped
	}

	// Escape brackets in link text
	escaped = strings.ReplaceAll(escaped, "[", "\\[")
	escaped = strings.ReplaceAll(escaped, "]", "\\]")

	return fmt.Sprintf("[%s](%s)", escaped, u.String())
}

// escapeText escapes HTML special characters in user-provided text.
func escapeText(s string) string {
	return html.EscapeString(s)
}

// truncateSummary truncates text to a reasonable length, preferring sentence boundaries.
func truncateSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Try to find a sentence boundary within range
	truncated := s[:maxLen]
	if idx := strings.LastIndex(truncated, ". "); idx > maxLen/2 {
		return s[:idx+1]
	}
	if idx := strings.LastIndex(truncated, "! "); idx > maxLen/2 {
		return s[:idx+1]
	}
	if idx := strings.LastIndex(truncated, "? "); idx > maxLen/2 {
		return s[:idx+1]
	}

	// Fall back to word boundary
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		return s[:idx] + "..."
	}

	return truncated + "..."
}

// formatIssueDisplayName converts an issue slug to a display name.
func formatIssueDisplayName(slug string) string {
	switch slug {
	case "science-technology":
		return "Science & Technology"
	case "planet-climate":
		return "Planet & Climate"
	case "existential-threats":
		return "Existential Threats"
	case "human-development":
		return "Human Development"
	default:
		// Convert kebab-case to Title Case
		parts := strings.Split(slug, "-")
		for i, p := range parts {
			if len(p) > 0 {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
		return strings.Join(parts, " ")
	}
}
