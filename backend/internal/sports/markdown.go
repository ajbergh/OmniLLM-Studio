package sports

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func RenderGamesMarkdown(req SportsRequest, cfg LeagueConfig, rows []GameRow, retrievedAt time.Time) string {
	switch renderMode(req) {
	case SportsRenderPlainMarkdown:
		return renderGamesPlainMarkdown(req, cfg, rows, retrievedAt)
	case SportsRenderHTMLMarkdown:
		return renderGamesHTMLMarkdown(req, cfg, rows, retrievedAt)
	default:
		return renderGamesEnhancedMarkdown(req, cfg, rows, retrievedAt)
	}
}

func renderGamesPlainMarkdown(req SportsRequest, cfg LeagueConfig, rows []GameRow, retrievedAt time.Time) string {
	intentLabel := "Scores"
	if req.Intent == SportsIntentSchedule {
		intentLabel = "Schedule"
	}
	if wantsPitchingMatchups(req) {
		intentLabel = "Pitching Matchups"
	}

	var b strings.Builder
	title := fmt.Sprintf("### %s %s", cfg.DisplayName, intentLabel)
	if label := strings.TrimSpace(req.DateLabel); label != "" {
		title += " — " + label
	}
	b.WriteString(title)
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)

	if shouldRenderSchedule(req, rows) {
		if wantsPitchingMatchups(req) {
			b.WriteString(renderTable(
				[]string{"Time", "Away", "Home", "Pitching Matchup", "Venue", "Broadcast"},
				[]string{"---", "---", "---", "---", "---", "---"},
				pitchingScheduleRows(rows),
			))
			return strings.TrimSpace(b.String())
		}
		b.WriteString(renderTable(
			[]string{"Time", "Away", "Home", "Venue", "Broadcast"},
			[]string{"---", "---", "---", "---", "---"},
			scheduleRows(rows),
		))
		return strings.TrimSpace(b.String())
	}

	b.WriteString(renderTable(
		[]string{"Status", "Away", "Score", "Home", "Score", "Venue"},
		[]string{"---", "---", "---:", "---", "---:", "---"},
		scoreRows(rows),
	))
	return strings.TrimSpace(b.String())
}

func RenderStandingsMarkdown(req SportsRequest, cfg LeagueConfig, rows []StandingsRow, retrievedAt time.Time) string {
	switch renderMode(req) {
	case SportsRenderPlainMarkdown:
		return renderStandingsPlainMarkdown(req, cfg, rows, retrievedAt)
	case SportsRenderHTMLMarkdown:
		return renderStandingsHTMLMarkdown(req, cfg, rows, retrievedAt)
	default:
		return renderStandingsEnhancedMarkdown(req, cfg, rows, retrievedAt)
	}
}

func renderStandingsPlainMarkdown(req SportsRequest, cfg LeagueConfig, rows []StandingsRow, retrievedAt time.Time) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s Standings", cfg.DisplayName))
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)

	if grouping, ok := standingsGroupingForLeague(cfg); ok {
		writeStructuredStandingsMarkdown(&b, rows, SportsRenderPlainMarkdown, grouping)
		return strings.TrimSpace(b.String())
	}

	groups := orderedGroups(rows)
	sectioned := len(groups) > 1
	for i, group := range groups {
		if sectioned {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("#### ")
			b.WriteString(escapeMarkdownCell(emptyAsDash(group)))
			b.WriteString("\n\n")
		}
		groupRows := rowsForGroup(rows, group)
		if isCricketLeague(cfg) {
			b.WriteString(renderCricketStandingsTable(groupRows, !sectioned, SportsRenderPlainMarkdown))
		} else if isSoccerLeague(cfg) {
			b.WriteString(renderSoccerStandingsTable(groupRows, !sectioned, SportsRenderPlainMarkdown))
		} else {
			b.WriteString(renderDefaultStandingsTable(groupRows, !sectioned, SportsRenderPlainMarkdown))
		}
		if i < len(groups)-1 {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func renderGamesEnhancedMarkdown(req SportsRequest, cfg LeagueConfig, rows []GameRow, retrievedAt time.Time) string {
	intentLabel := "Scores"
	if req.Intent == SportsIntentSchedule {
		intentLabel = "Schedule"
	}
	if wantsPitchingMatchups(req) {
		intentLabel = "Pitching Matchups"
	}
	title := fmt.Sprintf("%s %s", cfg.DisplayName, intentLabel)
	if label := strings.TrimSpace(req.DateLabel); label != "" {
		title += " — " + label
	}

	var b strings.Builder
	b.WriteString(renderLeagueHeader(SportsLookupResult{
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: firstNonEmpty(req.LeagueLogoURL, leagueIdentityForConfig(cfg).LogoURL),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
	}, title, SportsRenderEnhancedMarkdown))

	if shouldRenderSchedule(req, rows) {
		if wantsPitchingMatchups(req) {
			b.WriteString(renderTable(
				[]string{"Time", "Matchup", "Pitching Matchup", "Venue", "Broadcast"},
				[]string{"---", "---", "---", "---", "---"},
				pitchingScheduleRowsMode(rows, SportsRenderEnhancedMarkdown),
			))
			return strings.TrimSpace(b.String())
		}
		b.WriteString(renderTable(
			[]string{"Time", "Matchup", "Venue", "Broadcast"},
			[]string{"---", "---", "---", "---"},
			scheduleRowsMode(rows, SportsRenderEnhancedMarkdown),
		))
		return strings.TrimSpace(b.String())
	}

	b.WriteString(renderTable(
		[]string{"Status", "Matchup", "Score", "Venue"},
		[]string{"---", "---", "---:", "---"},
		scoreRowsMode(rows, SportsRenderEnhancedMarkdown),
	))
	return strings.TrimSpace(b.String())
}

func renderStandingsEnhancedMarkdown(req SportsRequest, cfg LeagueConfig, rows []StandingsRow, retrievedAt time.Time) string {
	var b strings.Builder
	b.WriteString(renderLeagueHeader(SportsLookupResult{
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: firstNonEmpty(req.LeagueLogoURL, leagueIdentityForConfig(cfg).LogoURL),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
	}, fmt.Sprintf("%s Standings", cfg.DisplayName), SportsRenderEnhancedMarkdown))

	if grouping, ok := standingsGroupingForLeague(cfg); ok {
		writeStructuredStandingsMarkdown(&b, rows, SportsRenderEnhancedMarkdown, grouping)
		return strings.TrimSpace(b.String())
	}

	groups := orderedGroups(rows)
	sectioned := len(groups) > 1
	for i, group := range groups {
		if sectioned {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("#### ")
			b.WriteString(escapeMarkdownCell(emptyAsDash(group)))
			b.WriteString("\n\n")
		}
		groupRows := rowsForGroup(rows, group)
		if isCricketLeague(cfg) {
			b.WriteString(renderCricketStandingsTable(groupRows, !sectioned, SportsRenderEnhancedMarkdown))
		} else if isSoccerLeague(cfg) {
			b.WriteString(renderSoccerStandingsTable(groupRows, !sectioned, SportsRenderEnhancedMarkdown))
		} else {
			b.WriteString(renderDefaultStandingsTable(groupRows, !sectioned, SportsRenderEnhancedMarkdown))
		}
		if i < len(groups)-1 {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func renderGamesHTMLMarkdown(req SportsRequest, cfg LeagueConfig, rows []GameRow, retrievedAt time.Time) string {
	intentLabel := "Scores"
	if req.Intent == SportsIntentSchedule {
		intentLabel = "Schedule"
	}
	title := fmt.Sprintf("%s %s", cfg.DisplayName, intentLabel)
	if label := strings.TrimSpace(req.DateLabel); label != "" {
		title += " — " + label
	}

	var b strings.Builder
	b.WriteString(renderLeagueHeader(SportsLookupResult{
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: firstNonEmpty(req.LeagueLogoURL, leagueIdentityForConfig(cfg).LogoURL),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
	}, title, SportsRenderHTMLMarkdown))

	if shouldRenderSchedule(req, rows) {
		writeHTMLTable(&b,
			[]htmlHeader{{"Time", "left"}, {"Matchup", "left"}, {"Venue", "left"}, {"Broadcast", "left"}},
			scheduleRowsHTML(rows),
		)
	} else {
		writeHTMLTable(&b,
			[]htmlHeader{{"Status", "left"}, {"Matchup", "left"}, {"Score", "right"}, {"Venue", "left"}},
			scoreRowsHTML(rows),
		)
	}
	b.WriteString("</div>\n")
	return strings.TrimSpace(b.String())
}

func renderStandingsHTMLMarkdown(req SportsRequest, cfg LeagueConfig, rows []StandingsRow, retrievedAt time.Time) string {
	var b strings.Builder
	b.WriteString(renderLeagueHeader(SportsLookupResult{
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: firstNonEmpty(req.LeagueLogoURL, leagueIdentityForConfig(cfg).LogoURL),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
	}, fmt.Sprintf("%s Standings", cfg.DisplayName), SportsRenderHTMLMarkdown))

	if grouping, ok := standingsGroupingForLeague(cfg); ok {
		writeStructuredStandingsHTML(&b, rows, grouping)
		b.WriteString("</div>\n")
		return strings.TrimSpace(b.String())
	}

	groups := orderedGroups(rows)
	sectioned := len(groups) > 1
	for _, group := range groups {
		if sectioned {
			b.WriteString("<h4>")
			b.WriteString(escapeHTML(emptyAsDash(group)))
			b.WriteString("</h4>\n")
		}
		groupRows := rowsForGroup(rows, group)
		if isCricketLeague(cfg) {
			writeHTMLTable(&b,
				[]htmlHeader{{"Rank", "right"}, {"Team", "left"}, {"M", "right"}, {"W", "right"}, {"L", "right"}, {"T", "right"}, {"N/R", "right"}, {"PT", "right"}, {"NRR", "right"}, {"For", "right"}, {"Against", "right"}},
				cricketStandingsRowsHTML(groupRows),
			)
		} else if isSoccerLeague(cfg) {
			writeHTMLTable(&b,
				[]htmlHeader{{"Rank", "right"}, {"Club", "left"}, {"GP", "right"}, {"W", "right"}, {"D", "right"}, {"L", "right"}, {"Pts", "right"}, {"GD", "right"}},
				soccerStandingsRowsHTML(groupRows),
			)
		} else {
			writeHTMLTable(&b,
				[]htmlHeader{{"Rank", "right"}, {"Team", "left"}, {"W", "right"}, {"L", "right"}, {"Pct", "right"}, {"GB", "right"}, {"Streak", "left"}},
				defaultStandingsRowsHTML(groupRows),
			)
		}
	}
	b.WriteString("</div>\n")
	return strings.TrimSpace(b.String())
}

func RenderNewsMarkdown(req SportsRequest, leagueName string, rows []NewsRow, retrievedAt time.Time) string {
	switch renderMode(req) {
	case SportsRenderPlainMarkdown:
		return renderNewsPlainMarkdown(req, leagueName, rows, retrievedAt)
	case SportsRenderHTMLMarkdown:
		return renderNewsHTMLMarkdown(req, leagueName, rows, retrievedAt)
	default:
		return renderNewsEnhancedMarkdown(req, leagueName, rows, retrievedAt)
	}
}

func renderNewsPlainMarkdown(req SportsRequest, leagueName string, rows []NewsRow, retrievedAt time.Time) string {
	var b strings.Builder
	b.WriteString(newsTitle(req, leagueName))
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)
	b.WriteString(renderTable(
		[]string{"Published", "Headline", "Summary", "Link"},
		[]string{"---", "---", "---", "---"},
		newsRows(rows),
	))
	return strings.TrimSpace(b.String())
}

func renderNewsEnhancedMarkdown(req SportsRequest, leagueName string, rows []NewsRow, retrievedAt time.Time) string {
	var b strings.Builder
	b.WriteString(newsTitle(req, leagueName))
	b.WriteString("\n\n")
	writeEnhancedSourceLine(&b, retrievedAt)
	b.WriteString("_Latest headlines from ESPN are shown with the newest story first._\n\n")

	lead := rows[0]
	b.WriteString("#### Lead Story\n\n")
	if image := renderNewsImage(lead, SportsRenderEnhancedMarkdown); image != "" {
		b.WriteString(image)
		b.WriteString("\n\n")
	}
	b.WriteString("### ")
	b.WriteString(newsHeadlineLink(lead))
	b.WriteString("\n\n")
	if meta := newsMetaMarkdown(lead); meta != "" {
		b.WriteString(meta)
		b.WriteString("\n\n")
	}
	if summary := strings.TrimSpace(lead.Description); summary != "" {
		b.WriteString("> ")
		b.WriteString(escapeMarkdownText(summary))
		b.WriteString("\n\n")
	}
	if link := newsReadLink(lead.URL); link != "" {
		b.WriteString(link)
		b.WriteString("\n\n")
	}

	if len(rows) > 1 {
		b.WriteString("---\n\n")
		b.WriteString("#### More Headlines\n\n")
		for i, row := range rows[1:] {
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(". **")
			b.WriteString(newsHeadlineLink(row))
			b.WriteString("**\n")
			if meta := newsMetaMarkdown(row); meta != "" {
				b.WriteString("   ")
				b.WriteString(meta)
				b.WriteString("\n")
			}
			if summary := strings.TrimSpace(row.Description); summary != "" {
				b.WriteString("   ")
				b.WriteString(escapeMarkdownText(summary))
				b.WriteString("\n")
			}
			if link := newsReadLink(row.URL); link != "" {
				b.WriteString("   ")
				b.WriteString(link)
				b.WriteString("\n")
			}
			if i < len(rows[1:])-1 {
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func renderNewsHTMLMarkdown(req SportsRequest, leagueName string, rows []NewsRow, retrievedAt time.Time) string {
	var b strings.Builder
	b.WriteString(`<div class="sports-news-card">`)
	b.WriteString("<h3>")
	b.WriteString(escapeHTML(strings.TrimPrefix(newsTitle(req, leagueName), "### ")))
	b.WriteString("</h3>")
	b.WriteString("<small>ESPN public API · Retrieved ")
	b.WriteString(escapeHTML(retrievedAt.Local().Format("Jan 2, 2006, 3:04 PM")))
	b.WriteString("</small>")
	for i, row := range rows {
		if i == 0 {
			b.WriteString(`<article class="sports-news-lead">`)
		} else {
			b.WriteString(`<article class="sports-news-item">`)
		}
		if image := renderNewsImage(row, SportsRenderHTMLMarkdown); image != "" {
			b.WriteString(image)
		}
		b.WriteString("<h4>")
		b.WriteString(newsHeadlineLinkHTML(row))
		b.WriteString("</h4>")
		b.WriteString("<small>")
		b.WriteString(escapeHTML(newsMetaText(row)))
		b.WriteString("</small>")
		if row.Description != "" {
			b.WriteString("<p>")
			b.WriteString(escapeHTML(row.Description))
			b.WriteString("</p>")
		}
		b.WriteString("</article>")
	}
	b.WriteString("</div>")
	return strings.TrimSpace(b.String())
}

func escapeMarkdownCell(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

func escapeMarkdownText(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func escapeMarkdownAlt(s string) string {
	s = escapeMarkdownText(s)
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	return s
}

func escapeMarkdownLinkText(s string) string {
	s = escapeMarkdownText(s)
	s = strings.ReplaceAll(s, "[", `\[`)
	s = strings.ReplaceAll(s, "]", `\]`)
	return s
}

func escapeHTML(s string) string {
	return html.EscapeString(strings.TrimSpace(s))
}

func emptyAsDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func writeSourceLine(b *strings.Builder, retrievedAt time.Time) {
	b.WriteString("_Source: ESPN public API. Retrieved: ")
	b.WriteString(retrievedAt.Local().Format("2006-01-02 3:04 PM"))
	b.WriteString("_\n\n")
}

func writeEnhancedSourceLine(b *strings.Builder, retrievedAt time.Time) {
	b.WriteString("_Source: ESPN public API · Retrieved: ")
	b.WriteString(retrievedAt.Local().Format("Jan 2, 2006, 3:04 PM"))
	b.WriteString("_\n\n")
}

func renderLeagueHeader(result SportsLookupResult, title string, mode SportsRenderMode) string {
	var b strings.Builder
	leagueName := firstNonEmpty(result.LeagueName, result.League)
	if mode == SportsRenderHTMLMarkdown {
		b.WriteString("<div class=\"sports-card\">\n")
		b.WriteString("<div class=\"sports-card-header\">")
		if logo := renderLogoImg(result.LeagueLogoURL, leagueName+" logo", 28, 28, mode); logo != "" {
			b.WriteString(logo)
		} else if badge := renderAbbreviationBadge(leagueName, mode); badge != "" {
			b.WriteString(badge)
		}
		b.WriteString("<div><strong>")
		b.WriteString(escapeHTML(title))
		b.WriteString("</strong><br><small>ESPN public API · Retrieved ")
		b.WriteString(escapeHTML(result.RetrievedAt.Local().Format("Jan 2, 2006, 3:04 PM")))
		b.WriteString("</small></div></div>\n")
		return b.String()
	}

	b.WriteString("### ")
	if logo := renderLogoImg(result.LeagueLogoURL, leagueName+" logo", 24, 24, mode); logo != "" {
		b.WriteString(logo)
		b.WriteString(" ")
	} else if badge := renderAbbreviationBadge(leagueName, mode); badge != "" && mode != SportsRenderPlainMarkdown {
		b.WriteString(badge)
		b.WriteString(" ")
	}
	b.WriteString(escapeMarkdownText(title))
	b.WriteString("\n\n")
	writeEnhancedSourceLine(&b, result.RetrievedAt)
	return b.String()
}

func renderLogoImg(rawURL string, alt string, width int, height int, mode SportsRenderMode) string {
	url := sanitizeImageURL(rawURL)
	if url == "" || mode == SportsRenderPlainMarkdown {
		return ""
	}
	alt = strings.TrimSpace(alt)
	if mode == SportsRenderHTMLMarkdown {
		return fmt.Sprintf(`<img src="%s" width="%d" height="%d" alt="%s">`,
			escapeHTML(url), width, height, escapeHTML(alt))
	}
	if alt == "" {
		alt = "logo"
	}
	url = strings.ReplaceAll(url, ")", "%29")
	return "![" + escapeMarkdownAlt(alt) + "](" + url + ")"
}

func renderAbbreviationBadge(abbr string, mode SportsRenderMode) string {
	abbr = strings.TrimSpace(abbr)
	if abbr == "" || mode == SportsRenderPlainMarkdown {
		return ""
	}
	if len(abbr) > 14 {
		fields := strings.Fields(abbr)
		var initials strings.Builder
		for _, field := range fields {
			for _, r := range field {
				initials.WriteString(strings.ToUpper(string(r)))
				break
			}
		}
		if initials.Len() > 0 && initials.Len() <= 8 {
			abbr = initials.String()
		}
	}
	if mode == SportsRenderHTMLMarkdown {
		return `<span class="sports-logo-fallback">` + escapeHTML(abbr) + `</span>`
	}
	return "**" + escapeMarkdownCell(abbr) + "**"
}

func shouldRenderSchedule(req SportsRequest, rows []GameRow) bool {
	if req.Intent == SportsIntentSchedule {
		return true
	}
	for _, row := range rows {
		if strings.TrimSpace(row.AwayScore) != "" || strings.TrimSpace(row.HomeScore) != "" {
			return false
		}
	}
	return true
}

func scoreRows(rows []GameRow) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, []string{
			emptyAsDash(row.Status),
			emptyAsDash(row.AwayTeam),
			emptyAsDash(row.AwayScore),
			emptyAsDash(row.HomeTeam),
			emptyAsDash(row.HomeScore),
			emptyAsDash(row.Venue),
		})
	}
	return out
}

func scheduleRows(rows []GameRow) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status)
		out = append(out, []string{
			emptyAsDash(timeText),
			emptyAsDash(row.AwayTeam),
			emptyAsDash(row.HomeTeam),
			emptyAsDash(row.Venue),
			emptyAsDash(row.Broadcasts),
		})
	}
	return out
}

func pitchingScheduleRows(rows []GameRow) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status)
		out = append(out, []string{
			emptyAsDash(timeText),
			emptyAsDash(row.AwayTeam),
			emptyAsDash(row.HomeTeam),
			emptyAsDash(row.PitchingMatchup),
			emptyAsDash(row.Venue),
			emptyAsDash(row.Broadcasts),
		})
	}
	return out
}

func scoreRowsMode(rows []GameRow, mode SportsRenderMode) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, []string{
			renderStatusBadge(scoreStatusLabel(row), row.StatusType, mode),
			matchupCell(row, mode),
			compactScoreCell(row),
			emptyAsDash(row.Venue),
		})
	}
	return out
}

func scheduleRowsMode(rows []GameRow, mode SportsRenderMode) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status, scoreStatusLabel(row))
		out = append(out, []string{
			renderStatusBadge(timeText, row.StatusType, mode),
			matchupCell(row, mode),
			emptyAsDash(row.Venue),
			emptyAsDash(row.Broadcasts),
		})
	}
	return out
}

func pitchingScheduleRowsMode(rows []GameRow, mode SportsRenderMode) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status, scoreStatusLabel(row))
		out = append(out, []string{
			renderStatusBadge(timeText, row.StatusType, mode),
			matchupCell(row, mode),
			emptyAsDash(row.PitchingMatchup),
			emptyAsDash(row.Venue),
			emptyAsDash(row.Broadcasts),
		})
	}
	return out
}

func scoreRowsHTML(rows []GameRow) [][]htmlCell {
	out := make([][]htmlCell, 0, len(rows))
	for _, row := range rows {
		out = append(out, []htmlCell{
			{text: renderStatusBadge(scoreStatusLabel(row), row.StatusType, SportsRenderHTMLMarkdown)},
			{text: matchupCell(row, SportsRenderHTMLMarkdown)},
			{text: escapeHTML(emptyAsDash(compactScoreCell(row))), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Venue))},
		})
	}
	return out
}

func scheduleRowsHTML(rows []GameRow) [][]htmlCell {
	out := make([][]htmlCell, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status, scoreStatusLabel(row))
		out = append(out, []htmlCell{
			{text: renderStatusBadge(timeText, row.StatusType, SportsRenderHTMLMarkdown)},
			{text: matchupCell(row, SportsRenderHTMLMarkdown)},
			{text: escapeHTML(emptyAsDash(row.Venue))},
			{text: escapeHTML(emptyAsDash(row.Broadcasts))},
		})
	}
	return out
}

func matchupCell(row GameRow, mode SportsRenderMode) string {
	away := renderTeamCell(gameAway(row), mode)
	home := renderTeamCell(gameHome(row), mode)
	if mode == SportsRenderHTMLMarkdown {
		return away + ` <span class="sports-matchup-at">at</span> ` + home
	}
	return away + " at " + home
}

func compactScoreCell(row GameRow) string {
	awayScore := strings.TrimSpace(row.AwayScore)
	homeScore := strings.TrimSpace(row.HomeScore)
	if awayScore != "" || homeScore != "" {
		away := firstNonEmpty(gameAway(row).Abbreviation, gameAway(row).ShortName, gameAway(row).DisplayName)
		home := firstNonEmpty(gameHome(row).Abbreviation, gameHome(row).ShortName, gameHome(row).DisplayName)
		return strings.TrimSpace(fmt.Sprintf("%s %s · %s %s",
			emptyAsDash(away), emptyAsDash(awayScore), emptyAsDash(home), emptyAsDash(homeScore)))
	}
	switch normalizedStatusType(row.Status, row.StatusType) {
	case "scheduled":
		return "Preview"
	case "postponed":
		return "—"
	default:
		if strings.TrimSpace(row.Status) == "" {
			return "Preview"
		}
		return "—"
	}
}

func scoreStatusLabel(row GameRow) string {
	switch normalizedStatusType(row.Status, row.StatusType) {
	case "final":
		return "Final"
	case "live":
		return firstNonEmpty(row.Status, "Live")
	case "scheduled":
		return firstNonEmpty(row.Time, row.Status, "Scheduled")
	case "postponed":
		return firstNonEmpty(row.Status, "Postponed")
	default:
		return firstNonEmpty(row.Status, row.Time)
	}
}

func newsRows(rows []NewsRow) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		headline := firstNonEmpty(row.Headline, row.Description)
		summary := row.Description
		if summary == headline {
			summary = ""
		}
		out = append(out, []string{
			emptyAsDash(row.Published),
			emptyAsDash(headline),
			emptyAsDash(summary),
			newsLinkCell(row.URL),
		})
	}
	return out
}

func renderNewsImage(row NewsRow, mode SportsRenderMode) string {
	url := sanitizeImageURL(row.ImageURL)
	if url == "" || mode == SportsRenderPlainMarkdown {
		return ""
	}
	alt := firstNonEmpty(row.ImageAlt, row.Headline, "ESPN news image")
	if mode == SportsRenderHTMLMarkdown {
		return `<img src="` + escapeHTML(url) + `" alt="` + escapeHTML(alt) + `">`
	}
	return "![Sports news image: " + escapeMarkdownAlt(alt) + "](" + strings.ReplaceAll(url, ")", "%29") + ")"
}

func writeNewsMetaLine(b *strings.Builder, row NewsRow, mode SportsRenderMode) {
	meta := newsMetaText(row)
	if meta == "" {
		return
	}
	if mode == SportsRenderHTMLMarkdown {
		b.WriteString("<small>")
		b.WriteString(escapeHTML(meta))
		b.WriteString("</small>")
		return
	}
	b.WriteString("_")
	b.WriteString(escapeMarkdownText(meta))
	b.WriteString("_\n\n")
}

func newsMetaMarkdown(row NewsRow) string {
	meta := newsMetaText(row)
	if meta == "" {
		return ""
	}
	return "_" + escapeMarkdownText(meta) + "_"
}

func newsMetaText(row NewsRow) string {
	var parts []string
	if row.Published != "" {
		parts = append(parts, row.Published)
	}
	if row.Byline != "" {
		parts = append(parts, row.Byline)
	}
	return strings.Join(parts, " · ")
}

func newsHeadlineLink(row NewsRow) string {
	headline := firstNonEmpty(row.Headline, row.Description, "ESPN story")
	url := safeExternalURL(row.URL)
	if url == "" {
		return escapeMarkdownText(headline)
	}
	return "[" + escapeMarkdownLinkText(headline) + "](" + url + ")"
}

func newsHeadlineLinkHTML(row NewsRow) string {
	headline := firstNonEmpty(row.Headline, row.Description, "ESPN story")
	url := safeExternalURL(row.URL)
	if url == "" {
		return escapeHTML(headline)
	}
	return `<a href="` + escapeHTML(url) + `">` + escapeHTML(headline) + `</a>`
}

func newsReadLink(url string) string {
	url = safeExternalURL(url)
	if url == "" {
		return ""
	}
	return "[Read on ESPN](" + url + ")"
}

func renderTeamCell(team TeamIdentity, mode SportsRenderMode) string {
	name := firstNonEmpty(team.DisplayName, team.ShortName, team.Abbreviation)
	if name == "" {
		return "—"
	}
	if mode == SportsRenderPlainMarkdown {
		return name
	}

	logo := renderLogoImg(team.LogoURL, firstNonEmpty(team.Abbreviation, team.ShortName, name)+" logo", 20, 20, mode)
	if mode == SportsRenderHTMLMarkdown {
		if logo == "" {
			logo = renderAbbreviationBadge(team.Abbreviation, mode)
		}
		return `<span class="sports-team">` + logo + `<span>` + escapeHTML(name) + `</span></span>`
	}
	if logo != "" {
		return logo + " " + name
	}
	if badge := renderAbbreviationBadge(team.Abbreviation, mode); badge != "" {
		return badge + " " + name
	}
	return name
}

func renderStatusBadge(status string, statusType string, mode SportsRenderMode) string {
	label := statusBadgeLabel(status, statusType)
	if mode == SportsRenderPlainMarkdown {
		return label
	}
	if mode == SportsRenderHTMLMarkdown {
		className := "sports-status-" + normalizedStatusType(label, statusType)
		return `<span class="sports-status ` + escapeHTML(className) + `">` + escapeHTML(label) + `</span>`
	}
	switch normalizedStatusType(label, statusType) {
	case "live":
		return "**" + escapeMarkdownCell(label) + "**"
	default:
		return label
	}
}

func statusBadgeLabel(status string, statusType string) string {
	status = strings.TrimSpace(status)
	switch normalizedStatusType(status, statusType) {
	case "final":
		return "Final"
	case "live":
		return firstNonEmpty(status, "Live")
	case "scheduled":
		return firstNonEmpty(status, "Scheduled")
	case "postponed":
		return firstNonEmpty(status, "Postponed")
	default:
		return emptyAsDash(status)
	}
}

func normalizedStatusType(status string, statusType string) string {
	statusType = normalizeText(statusType)
	status = normalizeText(status)
	switch {
	case statusType == "final", statusType == "post", status == "final", strings.Contains(status, "final"):
		return "final"
	case statusType == "live", statusType == "in", strings.Contains(statusType, "progress"), strings.Contains(status, "live"):
		return "live"
	case statusType == "scheduled", statusType == "pre", strings.Contains(status, "scheduled"), strings.Contains(status, "preview"):
		return "scheduled"
	case statusType == "postponed", strings.Contains(status, "postponed"), strings.Contains(status, "canceled"), strings.Contains(status, "cancelled"), strings.Contains(status, "suspended"):
		return "postponed"
	default:
		return "scheduled"
	}
}

func renderDefaultStandingsTable(rows []StandingsRow, includeGroup bool, mode SportsRenderMode) string {
	headers := []string{"Rank", "Team", "W", "L", "Pct", "GB", "Streak"}
	separators := []string{"---:", "---", "---:", "---:", "---:", "---:", "---"}
	if includeGroup && hasAnyGroup(rows) {
		headers = append([]string{"Group"}, headers...)
		separators = append([]string{"---"}, separators...)
	}

	tableRows := make([][]string, 0, len(rows))
	for i, row := range rows {
		team := standingsTeam(row)
		cells := []string{
			standingsRankCell(row, i),
			emptyAsDash(renderTeamCell(team, mode)),
			emptyAsDash(row.Wins),
			emptyAsDash(row.Losses),
			emptyAsDash(row.Pct),
			emptyAsDash(row.GamesBack),
			emptyAsDash(row.Streak),
		}
		if includeGroup && hasAnyGroup(rows) {
			cells = append([]string{emptyAsDash(row.Group)}, cells...)
		}
		tableRows = append(tableRows, cells)
	}
	return renderTable(headers, separators, tableRows)
}

func renderSoccerStandingsTable(rows []StandingsRow, includeGroup bool, mode SportsRenderMode) string {
	headers := []string{"Rank", "Club", "GP", "W", "D", "L", "Pts", "GD"}
	separators := []string{"---:", "---", "---:", "---:", "---:", "---:", "---:", "---:"}
	if includeGroup && hasAnyGroup(rows) {
		headers = append([]string{"Group"}, headers...)
		separators = append([]string{"---"}, separators...)
	}

	tableRows := make([][]string, 0, len(rows))
	for i, row := range rows {
		team := standingsTeam(row)
		cells := []string{
			standingsRankCell(row, i),
			emptyAsDash(renderTeamCell(team, mode)),
			emptyAsDash(row.GamesPlayed),
			emptyAsDash(row.Wins),
			emptyAsDash(row.Draws),
			emptyAsDash(row.Losses),
			emptyAsDash(row.Points),
			emptyAsDash(firstNonEmpty(row.GoalDiff, row.GoalDifferential)),
		}
		if includeGroup && hasAnyGroup(rows) {
			cells = append([]string{emptyAsDash(row.Group)}, cells...)
		}
		tableRows = append(tableRows, cells)
	}
	return renderTable(headers, separators, tableRows)
}

func renderCricketStandingsTable(rows []StandingsRow, includeGroup bool, mode SportsRenderMode) string {
	headers := []string{"Rank", "Team", "M", "W", "L", "T", "N/R", "PT", "NRR", "For", "Against"}
	separators := []string{"---:", "---", "---:", "---:", "---:", "---:", "---:", "---:", "---:", "---:", "---:"}
	if includeGroup && hasAnyGroup(rows) {
		headers = append([]string{"Group"}, headers...)
		separators = append([]string{"---"}, separators...)
	}

	tableRows := make([][]string, 0, len(rows))
	for i, row := range rows {
		team := standingsTeam(row)
		cells := []string{
			standingsRankCell(row, i),
			emptyAsDash(renderTeamCell(team, mode)),
			emptyAsDash(row.GamesPlayed),
			emptyAsDash(row.Wins),
			emptyAsDash(row.Losses),
			emptyAsDash(row.Ties),
			emptyAsDash(row.NoResult),
			emptyAsDash(row.Points),
			emptyAsDash(row.NetRunRate),
			emptyAsDash(row.For),
			emptyAsDash(row.Against),
		}
		if includeGroup && hasAnyGroup(rows) {
			cells = append([]string{emptyAsDash(row.Group)}, cells...)
		}
		tableRows = append(tableRows, cells)
	}
	return renderTable(headers, separators, tableRows)
}

func defaultStandingsRowsHTML(rows []StandingsRow) [][]htmlCell {
	out := make([][]htmlCell, 0, len(rows))
	for i, row := range rows {
		out = append(out, []htmlCell{
			{text: escapeHTML(standingsRankCell(row, i)), align: "right"},
			{text: renderTeamCell(standingsTeam(row), SportsRenderHTMLMarkdown)},
			{text: escapeHTML(emptyAsDash(row.Wins)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Losses)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Pct)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.GamesBack)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Streak))},
		})
	}
	return out
}

func soccerStandingsRowsHTML(rows []StandingsRow) [][]htmlCell {
	out := make([][]htmlCell, 0, len(rows))
	for i, row := range rows {
		out = append(out, []htmlCell{
			{text: escapeHTML(standingsRankCell(row, i)), align: "right"},
			{text: renderTeamCell(standingsTeam(row), SportsRenderHTMLMarkdown)},
			{text: escapeHTML(emptyAsDash(row.GamesPlayed)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Wins)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Draws)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Losses)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Points)), align: "right"},
			{text: escapeHTML(emptyAsDash(firstNonEmpty(row.GoalDiff, row.GoalDifferential))), align: "right"},
		})
	}
	return out
}

func cricketStandingsRowsHTML(rows []StandingsRow) [][]htmlCell {
	out := make([][]htmlCell, 0, len(rows))
	for i, row := range rows {
		out = append(out, []htmlCell{
			{text: escapeHTML(standingsRankCell(row, i)), align: "right"},
			{text: renderTeamCell(standingsTeam(row), SportsRenderHTMLMarkdown)},
			{text: escapeHTML(emptyAsDash(row.GamesPlayed)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Wins)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Losses)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Ties)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.NoResult)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Points)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.NetRunRate)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.For)), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Against)), align: "right"},
		})
	}
	return out
}

type mlbDivisionID struct {
	League   string
	Division string
}

var mlbDivisionOrder = []mlbDivisionID{
	{League: "American League", Division: "East"},
	{League: "American League", Division: "Central"},
	{League: "American League", Division: "West"},
	{League: "National League", Division: "East"},
	{League: "National League", Division: "Central"},
	{League: "National League", Division: "West"},
}

var mlbTeamDivisions = map[string]mlbDivisionID{
	"nyy":                  {League: "American League", Division: "East"},
	"newyorkyankees":       {League: "American League", Division: "East"},
	"tb":                   {League: "American League", Division: "East"},
	"tbr":                  {League: "American League", Division: "East"},
	"tampabayrays":         {League: "American League", Division: "East"},
	"bal":                  {League: "American League", Division: "East"},
	"baltimoreorioles":     {League: "American League", Division: "East"},
	"tor":                  {League: "American League", Division: "East"},
	"torontobluejays":      {League: "American League", Division: "East"},
	"bos":                  {League: "American League", Division: "East"},
	"bostonredsox":         {League: "American League", Division: "East"},
	"cle":                  {League: "American League", Division: "Central"},
	"clevelandguardians":   {League: "American League", Division: "Central"},
	"det":                  {League: "American League", Division: "Central"},
	"detroittigers":        {League: "American League", Division: "Central"},
	"chw":                  {League: "American League", Division: "Central"},
	"cws":                  {League: "American League", Division: "Central"},
	"chicagowhitesox":      {League: "American League", Division: "Central"},
	"kc":                   {League: "American League", Division: "Central"},
	"kcr":                  {League: "American League", Division: "Central"},
	"kansascityroyals":     {League: "American League", Division: "Central"},
	"min":                  {League: "American League", Division: "Central"},
	"minnesotatwins":       {League: "American League", Division: "Central"},
	"hou":                  {League: "American League", Division: "West"},
	"houstonastros":        {League: "American League", Division: "West"},
	"sea":                  {League: "American League", Division: "West"},
	"seattlemariners":      {League: "American League", Division: "West"},
	"tex":                  {League: "American League", Division: "West"},
	"texasrangers":         {League: "American League", Division: "West"},
	"laa":                  {League: "American League", Division: "West"},
	"losangelesangels":     {League: "American League", Division: "West"},
	"ath":                  {League: "American League", Division: "West"},
	"oak":                  {League: "American League", Division: "West"},
	"athletics":            {League: "American League", Division: "West"},
	"oaklandathletics":     {League: "American League", Division: "West"},
	"phi":                  {League: "National League", Division: "East"},
	"philadelphiaphillies": {League: "National League", Division: "East"},
	"nym":                  {League: "National League", Division: "East"},
	"newyorkmets":          {League: "National League", Division: "East"},
	"atl":                  {League: "National League", Division: "East"},
	"atlantabraves":        {League: "National League", Division: "East"},
	"mia":                  {League: "National League", Division: "East"},
	"miamimarlins":         {League: "National League", Division: "East"},
	"wsh":                  {League: "National League", Division: "East"},
	"was":                  {League: "National League", Division: "East"},
	"wsn":                  {League: "National League", Division: "East"},
	"washingtonnationals":  {League: "National League", Division: "East"},
	"chc":                  {League: "National League", Division: "Central"},
	"chicagocubs":          {League: "National League", Division: "Central"},
	"mil":                  {League: "National League", Division: "Central"},
	"milwaukeebrewers":     {League: "National League", Division: "Central"},
	"stl":                  {League: "National League", Division: "Central"},
	"stlouiscardinals":     {League: "National League", Division: "Central"},
	"cin":                  {League: "National League", Division: "Central"},
	"cincinnatireds":       {League: "National League", Division: "Central"},
	"pit":                  {League: "National League", Division: "Central"},
	"pittsburghpirates":    {League: "National League", Division: "Central"},
	"lad":                  {League: "National League", Division: "West"},
	"losangelesdodgers":    {League: "National League", Division: "West"},
	"sd":                   {League: "National League", Division: "West"},
	"sdp":                  {League: "National League", Division: "West"},
	"sandiegopadres":       {League: "National League", Division: "West"},
	"sf":                   {League: "National League", Division: "West"},
	"sfg":                  {League: "National League", Division: "West"},
	"sanfranciscogiants":   {League: "National League", Division: "West"},
	"ari":                  {League: "National League", Division: "West"},
	"arizona":              {League: "National League", Division: "West"},
	"arizonadiamondbacks":  {League: "National League", Division: "West"},
	"col":                  {League: "National League", Division: "West"},
	"coloradorockies":      {League: "National League", Division: "West"},
}

func compactSportsKey(value string) string {
	return strings.ReplaceAll(normalizeText(value), " ", "")
}

func parseStandingsNumber(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" || value == "—" || value == "-" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil
}

func renderTable(headers, separators []string, rows [][]string) string {
	var b strings.Builder
	writeTableLine(&b, headers)
	writeTableLine(&b, separators)
	for _, row := range rows {
		writeTableLine(&b, row)
	}
	b.WriteString("\n")
	return b.String()
}

func writeTableLine(b *strings.Builder, cells []string) {
	b.WriteString("| ")
	for i, cell := range cells {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(escapeMarkdownCell(cell))
	}
	b.WriteString(" |\n")
}

type htmlHeader struct {
	Text  string
	Align string
}

type htmlCell struct {
	text  string
	align string
}

func writeHTMLTable(b *strings.Builder, headers []htmlHeader, rows [][]htmlCell) {
	b.WriteString("<table>\n<thead><tr>")
	for _, header := range headers {
		writeHTMLTableTag(b, "th", header.Align, escapeHTML(header.Text))
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, row := range rows {
		b.WriteString("<tr>")
		for _, cell := range row {
			writeHTMLTableTag(b, "td", cell.align, cell.text)
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody></table>\n")
}

func writeHTMLTableTag(b *strings.Builder, tag string, align string, content string) {
	b.WriteString("<")
	b.WriteString(tag)
	if align != "" {
		b.WriteString(` align="`)
		b.WriteString(escapeHTML(align))
		b.WriteString(`"`)
	}
	b.WriteString(">")
	b.WriteString(content)
	b.WriteString("</")
	b.WriteString(tag)
	b.WriteString(">")
}

func orderedGroups(rows []StandingsRow) []string {
	if len(rows) == 0 {
		return nil
	}
	var groups []string
	seen := map[string]bool{}
	for _, row := range rows {
		group := strings.TrimSpace(row.Group)
		if group == "" {
			group = "Standings"
		}
		if !seen[group] {
			seen[group] = true
			groups = append(groups, group)
		}
	}
	return groups
}

func rowsForGroup(rows []StandingsRow, group string) []StandingsRow {
	out := make([]StandingsRow, 0, len(rows))
	for _, row := range rows {
		rowGroup := strings.TrimSpace(row.Group)
		if rowGroup == "" {
			rowGroup = "Standings"
		}
		if rowGroup == group {
			out = append(out, row)
		}
	}
	return out
}

func hasAnyGroup(rows []StandingsRow) bool {
	for _, row := range rows {
		if strings.TrimSpace(row.Group) != "" {
			return true
		}
	}
	return false
}

func rankCell(rank int) string {
	if rank <= 0 {
		return "—"
	}
	return strconv.Itoa(rank)
}

func standingsRankCell(row StandingsRow, index int) string {
	if row.Rank > 0 {
		return rankCell(row.Rank)
	}
	return strconv.Itoa(index + 1)
}

func isSoccerLeague(cfg LeagueConfig) bool {
	return cfg.Sport == "soccer"
}

func isCricketLeague(cfg LeagueConfig) bool {
	return strings.EqualFold(cfg.Sport, "cricket") || strings.EqualFold(cfg.League, LeagueIPL)
}

func isMLBLeague(cfg LeagueConfig) bool {
	return strings.EqualFold(cfg.League, "mlb")
}

func newsTitle(req SportsRequest, leagueName string) string {
	if strings.TrimSpace(req.AthleteQuery) != "" {
		return "### " + escapeMarkdownCell(req.AthleteQuery) + " News"
	}
	if strings.TrimSpace(req.TeamQuery) != "" {
		return "### " + escapeMarkdownCell(req.TeamQuery) + " News"
	}
	if strings.TrimSpace(leagueName) == "" || strings.EqualFold(leagueName, "sports") {
		return "### Latest Sports News"
	}
	return "### " + escapeMarkdownCell(leagueName) + " News"
}

func newsLinkCell(url string) string {
	url = safeExternalURL(url)
	if url == "" {
		return "—"
	}
	return "[ESPN](" + url + ")"
}

func safeExternalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "http://") {
		raw = "https://" + raw[len("http://"):]
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	out := parsed.String()
	out = strings.ReplaceAll(out, " ", "%20")
	out = strings.ReplaceAll(out, ")", "%29")
	return out
}
