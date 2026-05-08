package sports

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func RenderGamesMarkdown(req SportsRequest, cfg LeagueConfig, rows []GameRow, retrievedAt time.Time) string {
	intentLabel := "Scores"
	if req.Intent == SportsIntentSchedule {
		intentLabel = "Schedule"
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
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s Standings", cfg.DisplayName))
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)

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
		if isSoccerLeague(cfg) {
			b.WriteString(renderSoccerStandingsTable(groupRows, !sectioned))
		} else {
			b.WriteString(renderDefaultStandingsTable(groupRows, !sectioned))
		}
		if i < len(groups)-1 {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func RenderNewsMarkdown(req SportsRequest, leagueName string, rows []NewsRow, retrievedAt time.Time) string {
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

func escapeMarkdownCell(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
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

func renderDefaultStandingsTable(rows []StandingsRow, includeGroup bool) string {
	headers := []string{"Rank", "Team", "W", "L", "Pct", "GB", "Streak"}
	separators := []string{"---:", "---", "---:", "---:", "---:", "---:", "---"}
	if includeGroup && hasAnyGroup(rows) {
		headers = append([]string{"Group"}, headers...)
		separators = append([]string{"---"}, separators...)
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		cells := []string{
			rankCell(row.Rank),
			emptyAsDash(row.Team),
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

func renderSoccerStandingsTable(rows []StandingsRow, includeGroup bool) string {
	headers := []string{"Rank", "Club", "GP", "W", "D", "L", "Pts", "GD"}
	separators := []string{"---:", "---", "---:", "---:", "---:", "---:", "---:", "---:"}
	if includeGroup && hasAnyGroup(rows) {
		headers = append([]string{"Group"}, headers...)
		separators = append([]string{"---"}, separators...)
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		cells := []string{
			rankCell(row.Rank),
			emptyAsDash(row.Team),
			emptyAsDash(row.GamesPlayed),
			emptyAsDash(row.Wins),
			emptyAsDash(row.Draws),
			emptyAsDash(row.Losses),
			emptyAsDash(row.Points),
			emptyAsDash(row.GoalDifferential),
		}
		if includeGroup && hasAnyGroup(rows) {
			cells = append([]string{emptyAsDash(row.Group)}, cells...)
		}
		tableRows = append(tableRows, cells)
	}
	return renderTable(headers, separators, tableRows)
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

func isSoccerLeague(cfg LeagueConfig) bool {
	return cfg.Sport == "soccer"
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
	url = strings.TrimSpace(url)
	if url == "" {
		return "—"
	}
	url = strings.ReplaceAll(url, " ", "%20")
	url = strings.ReplaceAll(url, ")", "%29")
	return "[ESPN](" + url + ")"
}
