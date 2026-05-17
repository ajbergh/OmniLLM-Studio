package sports

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

func (c *ESPNClient) LookupOdds(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	if cfg, ok := leagueConfigForRequest(req); ok {
		return c.lookupLeagueOdds(ctx, cfg, req)
	}
	return c.lookupBroadOdds(ctx, req)
}

func (c *ESPNClient) lookupLeagueOdds(ctx context.Context, cfg LeagueConfig, req SportsRequest) (*SportsLookupResult, error) {
	req.League = cfg.League
	req.Sport = cfg.Sport
	req.Intent = SportsIntentOdds

	rows, err := c.scoreboardOddsRows(ctx, cfg, req)
	if err != nil {
		return nil, err
	}
	if req.TeamQuery != "" {
		rows = filterOddsRowsByTeam(rows, req.TeamQuery)
	}
	if len(rows) == 0 {
		return nil, ErrNoOdds
	}
	if err := ValidateOddsRows(req, rows); err != nil {
		return nil, err
	}

	retrievedAt := c.timeNow()
	leagueLogoURL := leagueIdentityForConfig(cfg).LogoURL
	req.LeagueLogoURL = leagueLogoURL
	return &SportsLookupResult{
		Intent:        SportsIntentOdds,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueLogoURL,
		Sport:         cfg.Sport,
		DateLabel:     req.DateLabel,
		Markdown:      RenderOddsMarkdown(req, cfg, rows, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) lookupBroadOdds(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	req.Intent = SportsIntentOdds
	var rows []OddsRow
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimitForIntent(SportsIntentOdds)
	}

	for _, cfg := range leagueConfigs {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		leagueReq := req
		leagueReq.League = cfg.League
		leagueReq.Sport = cfg.Sport
		leagueReq.Limit = minInt(limit, 25)
		leagueRows, err := c.scoreboardOddsRows(ctx, cfg, leagueReq)
		if err != nil {
			continue
		}
		rows = append(rows, leagueRows...)
		if len(rows) >= limit {
			rows = rows[:limit]
			break
		}
	}
	if len(rows) == 0 {
		return nil, ErrNoOdds
	}
	if err := ValidateOddsRows(req, rows); err != nil {
		return nil, err
	}

	retrievedAt := c.timeNow()
	cfg := LeagueConfig{DisplayName: "Sports"}
	return &SportsLookupResult{
		Intent:      SportsIntentOdds,
		LeagueName:  cfg.DisplayName,
		DateLabel:   req.DateLabel,
		Markdown:    RenderOddsMarkdown(req, cfg, rows, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}

func (c *ESPNClient) scoreboardOddsRows(ctx context.Context, cfg LeagueConfig, req SportsRequest) ([]OddsRow, error) {
	opts := &espn.ScoreboardOptions{}
	if req.Date != nil {
		opts.SetDate(*req.Date)
	}
	if req.Limit > 0 {
		opts.Limit = req.Limit
	} else {
		opts.Limit = 100
	}

	sb, err := c.client.Scoreboard(ctx, cfg.Sport, cfg.League, opts)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	return normalizeOdds(sb, cfg), nil
}

func normalizeOdds(sb *espn.Scoreboard, cfg LeagueConfig) []OddsRow {
	if sb == nil || len(sb.Events) == 0 {
		return nil
	}

	rows := make([]OddsRow, 0, len(sb.Events))
	for _, ev := range sb.Events {
		var comp espn.Competition
		if len(ev.Competitions) > 0 {
			comp = ev.Competitions[0]
		}
		odds, ok := chooseOddsSummary(comp.Odds)
		if !ok {
			continue
		}

		away, home := splitCompetitors(comp.Competitors)
		if away == nil && home == nil {
			continue
		}

		status := chooseStatus(ev.Status, comp.Status)
		eventDate := firstNonEmpty(comp.Date, ev.Date)
		row := OddsRow{
			LeagueName: cfg.DisplayName,
			Date:       formatGameDate(eventDate),
			Time:       formatGameTime(eventDate),
			Status:     statusText(status, eventDate),
			StatusType: statusKind(status),
			Provider:   strings.TrimSpace(odds.Provider.Name),
			Spread:     spreadText(odds, away, home),
			OverUnder:  overUnderText(odds),
		}
		if away != nil {
			row.Away = extractTeamIdentityFromCompetitor(*away)
			row.AwayTeam = row.Away.DisplayName
			row.AwayAbbr = row.Away.Abbreviation
			if odds.AwayTeamOdds != nil {
				row.AwayMoneyLine = moneyLineText(odds.AwayTeamOdds.MoneyLine)
			}
		}
		if home != nil {
			row.Home = extractTeamIdentityFromCompetitor(*home)
			row.HomeTeam = row.Home.DisplayName
			row.HomeAbbr = row.Home.Abbreviation
			if odds.HomeTeamOdds != nil {
				row.HomeMoneyLine = moneyLineText(odds.HomeTeamOdds.MoneyLine)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func chooseOddsSummary(odds []espn.OddsSummary) (espn.OddsSummary, bool) {
	for _, item := range odds {
		if hasOddsData(item) {
			return item, true
		}
	}
	return espn.OddsSummary{}, false
}

func hasOddsData(odds espn.OddsSummary) bool {
	if strings.TrimSpace(odds.Details) != "" || odds.OverUnder != 0 || odds.Spread != 0 {
		return true
	}
	if odds.HomeTeamOdds != nil && (odds.HomeTeamOdds.MoneyLine != 0 || odds.HomeTeamOdds.SpreadOdds != 0) {
		return true
	}
	if odds.AwayTeamOdds != nil && (odds.AwayTeamOdds.MoneyLine != 0 || odds.AwayTeamOdds.SpreadOdds != 0) {
		return true
	}
	return false
}

func filterOddsRowsByTeam(rows []OddsRow, teamQuery string) []OddsRow {
	query := normalizeText(teamQuery)
	if query == "" {
		return rows
	}
	filtered := make([]OddsRow, 0, len(rows))
	for _, row := range rows {
		if oddsRowMatchesTeam(row, query) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func oddsRowMatchesTeam(row OddsRow, query string) bool {
	fields := []string{
		row.AwayTeam, row.AwayAbbr, row.HomeTeam, row.HomeAbbr,
		row.Away.DisplayName, row.Away.ShortName, row.Away.Location,
		row.Home.DisplayName, row.Home.ShortName, row.Home.Location,
	}
	for _, field := range fields {
		norm := normalizeText(field)
		if norm == "" {
			continue
		}
		if norm == query || strings.Contains(norm, query) || strings.Contains(query, norm) {
			return true
		}
	}
	return false
}

func spreadText(odds espn.OddsSummary, away, home *espn.Competitor) string {
	if strings.TrimSpace(odds.Details) != "" {
		return strings.TrimSpace(odds.Details)
	}
	if odds.Spread == 0 {
		return ""
	}
	favorite := ""
	if odds.AwayTeamOdds != nil && odds.AwayTeamOdds.Favorite && away != nil {
		favorite = teamShortLabel(away.Team)
	}
	if odds.HomeTeamOdds != nil && odds.HomeTeamOdds.Favorite && home != nil {
		favorite = teamShortLabel(home.Team)
	}
	if favorite != "" {
		return favorite + " " + signedBettingNumber(odds.Spread)
	}
	return signedBettingNumber(odds.Spread)
}

func overUnderText(odds espn.OddsSummary) string {
	if odds.OverUnder == 0 {
		return ""
	}
	total := formatOddsFloat(odds.OverUnder)
	over := moneyLineText(odds.OverOdds)
	under := moneyLineText(odds.UnderOdds)
	if over != "" || under != "" {
		return fmt.Sprintf("%s (O %s / U %s)", total, emptyAsDash(over), emptyAsDash(under))
	}
	return total
}

func moneyLineText(value float64) string {
	if value == 0 {
		return ""
	}
	return signedBettingNumber(value)
}

func signedBettingNumber(value float64) string {
	if value == 0 {
		return ""
	}
	text := formatOddsFloat(value)
	if value > 0 {
		return "+" + text
	}
	return text
}

func formatOddsFloat(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return formatFloatStat(value)
}

func teamShortLabel(team espn.Team) string {
	return firstNonEmpty(team.Abbreviation, team.ShortDisplayName, team.DisplayName, team.Name)
}

func RenderOddsMarkdown(req SportsRequest, cfg LeagueConfig, rows []OddsRow, retrievedAt time.Time) string {
	switch renderMode(req) {
	case SportsRenderPlainMarkdown:
		return renderOddsPlainMarkdown(req, cfg, rows, retrievedAt)
	case SportsRenderHTMLMarkdown:
		return renderOddsHTMLMarkdown(req, cfg, rows, retrievedAt)
	default:
		return renderOddsEnhancedMarkdown(req, cfg, rows, retrievedAt)
	}
}

func renderOddsPlainMarkdown(req SportsRequest, cfg LeagueConfig, rows []OddsRow, retrievedAt time.Time) string {
	var b strings.Builder
	titleName := cfg.DisplayName
	if titleName == "" {
		titleName = "Sports"
	}
	if strings.TrimSpace(req.TeamQuery) != "" && !strings.EqualFold(titleName, "sports") {
		titleName = req.TeamQuery
	}
	b.WriteString("### ")
	b.WriteString(escapeMarkdownCell(titleName))
	b.WriteString(" Betting Odds")
	if label := strings.TrimSpace(req.DateLabel); label != "" {
		b.WriteString(" — ")
		b.WriteString(escapeMarkdownCell(label))
	}
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)

	includeLeague := strings.EqualFold(titleName, "sports") || oddsRowsHaveMultipleLeagues(rows)
	headers := []string{"Date", "Time", "Away", "Away ML", "Home", "Home ML", "Spread", "O/U", "Provider"}
	separators := []string{"---", "---", "---", "---:", "---", "---:", "---", "---", "---"}
	if includeLeague {
		headers = append([]string{"League"}, headers...)
		separators = append([]string{"---"}, separators...)
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status)
		cells := []string{
			emptyAsDash(row.Date),
			emptyAsDash(timeText),
			emptyAsDash(row.AwayTeam),
			emptyAsDash(row.AwayMoneyLine),
			emptyAsDash(row.HomeTeam),
			emptyAsDash(row.HomeMoneyLine),
			emptyAsDash(row.Spread),
			emptyAsDash(row.OverUnder),
			emptyAsDash(row.Provider),
		}
		if includeLeague {
			cells = append([]string{emptyAsDash(row.LeagueName)}, cells...)
		}
		tableRows = append(tableRows, cells)
	}
	b.WriteString(renderTable(headers, separators, tableRows))
	return strings.TrimSpace(b.String())
}

func renderOddsEnhancedMarkdown(req SportsRequest, cfg LeagueConfig, rows []OddsRow, retrievedAt time.Time) string {
	var b strings.Builder
	titleName := oddsTitleName(req, cfg)
	title := titleName + " Betting Odds"
	if label := strings.TrimSpace(req.DateLabel); label != "" {
		title += " — " + label
	}
	b.WriteString(renderLeagueHeader(SportsLookupResult{
		League:        cfg.League,
		LeagueName:    titleName,
		LeagueLogoURL: firstNonEmpty(req.LeagueLogoURL, leagueIdentityForConfig(cfg).LogoURL),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
	}, title, SportsRenderEnhancedMarkdown))

	includeLeague := strings.EqualFold(titleName, "sports") || oddsRowsHaveMultipleLeagues(rows)
	headers := []string{"Date", "Time", "Matchup", "Moneyline", "Spread", "O/U", "Provider"}
	separators := []string{"---", "---", "---", "---:", "---", "---", "---"}
	if includeLeague {
		headers = append([]string{"League"}, headers...)
		separators = append([]string{"---"}, separators...)
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status)
		moneyline := fmt.Sprintf("%s %s · %s %s",
			emptyAsDash(firstNonEmpty(oddsAway(row).Abbreviation, oddsAway(row).ShortName, oddsAway(row).DisplayName)),
			emptyAsDash(row.AwayMoneyLine),
			emptyAsDash(firstNonEmpty(oddsHome(row).Abbreviation, oddsHome(row).ShortName, oddsHome(row).DisplayName)),
			emptyAsDash(row.HomeMoneyLine),
		)
		cells := []string{
			emptyAsDash(row.Date),
			emptyAsDash(timeText),
			oddsMatchupCell(row, SportsRenderEnhancedMarkdown),
			moneyline,
			emptyAsDash(row.Spread),
			emptyAsDash(row.OverUnder),
			emptyAsDash(row.Provider),
		}
		if includeLeague {
			cells = append([]string{emptyAsDash(row.LeagueName)}, cells...)
		}
		tableRows = append(tableRows, cells)
	}
	b.WriteString(renderTable(headers, separators, tableRows))
	return strings.TrimSpace(b.String())
}

func renderOddsHTMLMarkdown(req SportsRequest, cfg LeagueConfig, rows []OddsRow, retrievedAt time.Time) string {
	var b strings.Builder
	titleName := oddsTitleName(req, cfg)
	title := titleName + " Betting Odds"
	if label := strings.TrimSpace(req.DateLabel); label != "" {
		title += " — " + label
	}
	b.WriteString(renderLeagueHeader(SportsLookupResult{
		League:        cfg.League,
		LeagueName:    titleName,
		LeagueLogoURL: firstNonEmpty(req.LeagueLogoURL, leagueIdentityForConfig(cfg).LogoURL),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
	}, title, SportsRenderHTMLMarkdown))

	headers := []htmlHeader{{"Date", "left"}, {"Time", "left"}, {"Matchup", "left"}, {"Moneyline", "right"}, {"Spread", "left"}, {"O/U", "left"}, {"Provider", "left"}}
	includeLeague := strings.EqualFold(titleName, "sports") || oddsRowsHaveMultipleLeagues(rows)
	if includeLeague {
		headers = append([]htmlHeader{{"League", "left"}}, headers...)
	}

	htmlRows := make([][]htmlCell, 0, len(rows))
	for _, row := range rows {
		timeText := firstNonEmpty(row.Time, row.Status)
		moneyline := fmt.Sprintf("%s %s · %s %s",
			emptyAsDash(firstNonEmpty(oddsAway(row).Abbreviation, oddsAway(row).ShortName, oddsAway(row).DisplayName)),
			emptyAsDash(row.AwayMoneyLine),
			emptyAsDash(firstNonEmpty(oddsHome(row).Abbreviation, oddsHome(row).ShortName, oddsHome(row).DisplayName)),
			emptyAsDash(row.HomeMoneyLine),
		)
		cells := []htmlCell{
			{text: escapeHTML(emptyAsDash(row.Date))},
			{text: escapeHTML(emptyAsDash(timeText))},
			{text: oddsMatchupCell(row, SportsRenderHTMLMarkdown)},
			{text: escapeHTML(moneyline), align: "right"},
			{text: escapeHTML(emptyAsDash(row.Spread))},
			{text: escapeHTML(emptyAsDash(row.OverUnder))},
			{text: escapeHTML(emptyAsDash(row.Provider))},
		}
		if includeLeague {
			cells = append([]htmlCell{{text: escapeHTML(emptyAsDash(row.LeagueName))}}, cells...)
		}
		htmlRows = append(htmlRows, cells)
	}
	writeHTMLTable(&b, headers, htmlRows)
	b.WriteString("</div>\n")
	return strings.TrimSpace(b.String())
}

func oddsTitleName(req SportsRequest, cfg LeagueConfig) string {
	titleName := cfg.DisplayName
	if titleName == "" {
		titleName = "Sports"
	}
	if strings.TrimSpace(req.TeamQuery) != "" && !strings.EqualFold(titleName, "sports") {
		titleName = req.TeamQuery
	}
	return titleName
}

func oddsMatchupCell(row OddsRow, mode SportsRenderMode) string {
	away := renderTeamCell(oddsAway(row), mode)
	home := renderTeamCell(oddsHome(row), mode)
	if mode == SportsRenderHTMLMarkdown {
		return away + ` <span class="sports-matchup-at">at</span> ` + home
	}
	return away + " at " + home
}

func oddsRowsHaveMultipleLeagues(rows []OddsRow) bool {
	if len(rows) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, row := range rows {
		league := strings.TrimSpace(row.LeagueName)
		if league == "" {
			continue
		}
		seen[league] = true
		if len(seen) > 1 {
			return true
		}
	}
	return false
}
