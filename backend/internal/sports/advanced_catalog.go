package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	espn "github.com/chinmaykhachane/espn-go"
)

func (c *ESPNClient) LookupScoreboardHeader(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	date := req.Date
	if date == nil {
		today := dateOnly(c.timeNow())
		date = &today
	}
	label := strings.TrimSpace(req.DateLabel)
	if label == "" {
		label = "Today"
	}

	broadcastsOnly := req.GameDetailSubtype == "broadcasts" || hasAnyPhrase(normalizeText(req.RawQuery), "televised", "broadcast", "broadcasting", "national tv", "nationally televised")
	rows := make([][]string, 0, 40)
	for _, cfg := range scoreboardHeaderLeagues() {
		opts := &espn.ScoreboardOptions{Limit: 100}
		opts.SetDate(*date)
		sb, err := c.client.Scoreboard(ctx, cfg.Sport, cfg.League, opts)
		if err != nil {
			continue
		}
		for _, game := range normalizeScoreboard(sb) {
			if broadcastsOnly && strings.TrimSpace(game.Broadcasts) == "" {
				continue
			}
			rows = append(rows, []string{
				cfg.DisplayName,
				emptyAsDash(firstNonEmpty(game.Time, game.Status)),
				emptyAsDash(matchupCell(game, SportsRenderPlainMarkdown)),
				emptyAsDash(compactScoreCell(game)),
				emptyAsDash(game.Broadcasts),
				emptyAsDash(game.Venue),
			})
		}
	}

	title := fmt.Sprintf("### ESPN Games — %s", label)
	if broadcastsOnly {
		title = fmt.Sprintf("### Nationally Televised Games — %s", label)
	}
	table := SimpleTable{
		Headers: []string{"League", "Time/Status", "Matchup", "Score", "Broadcast", "Venue"},
		Rows:    rows,
	}
	if len(table.Rows) == 0 {
		raw, err := c.client.ScoreboardHeader(ctx)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		table = rawJSONTable(raw, req.Limit)
		title = "### ESPN Scoreboard Header"
	}
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentScoreboardHeader,
		DateLabel:   label,
		Markdown:    RenderSimpleMarkdown(title, table, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}

func scoreboardHeaderLeagues() []LeagueConfig {
	leagueIDs := []string{
		espn.LeagueMLB,
		espn.LeagueNBA,
		espn.LeagueWNBA,
		espn.LeagueNHL,
		espn.LeagueNFL,
		espn.LeagueCollegeFootball,
		espn.LeagueEPL,
		espn.LeagueMLS,
		espn.LeagueChampionsLg,
	}
	out := make([]LeagueConfig, 0, len(leagueIDs))
	for _, league := range leagueIDs {
		if cfg, ok := leagueConfigByLeague(league); ok {
			out = append(out, cfg)
		}
	}
	return out
}

func (c *ESPNClient) LookupTeams(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}

	var table SimpleTable
	title := fmt.Sprintf("### %s Teams", cfg.DisplayName)
	if req.Season > 0 {
		paged, err := c.client.SeasonTeams(ctx, cfg.Sport, cfg.League, req.Season, req.Limit)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		title = fmt.Sprintf("### %s Teams (%d)", cfg.DisplayName, req.Season)
		table = c.seasonTeamRefsTable(ctx, cfg, paged, req.Limit)
	} else {
		resp, err := c.client.Teams(ctx, cfg.Sport, cfg.League, req.Limit)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		table = teamsResponseTable(resp, req.Limit)
	}
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentTeams,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(title, table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func teamsResponseTable(resp *espn.TeamsResponse, limit int) SimpleTable {
	if limit <= 0 {
		limit = 100
	}
	rows := make([][]string, 0, limit)
	for _, team := range resp.Flatten() {
		if len(rows) >= limit {
			break
		}
		rows = append(rows, []string{
			emptyAsDash(team.ID),
			emptyAsDash(teamDisplayName(team)),
			emptyAsDash(team.Abbreviation),
			emptyAsDash(team.Location),
			emptyAsDash(team.Name),
		})
	}
	return SimpleTable{Headers: []string{"ID", "Team", "Abbr", "Location", "Name"}, Rows: rows}
}

func (c *ESPNClient) seasonTeamRefsTable(ctx context.Context, cfg LeagueConfig, paged *espn.PagedRefs, limit int) SimpleTable {
	if limit <= 0 {
		limit = 100
	}
	nameByID := map[string]espn.Team{}
	if resp, err := c.client.Teams(ctx, cfg.Sport, cfg.League, 200); err == nil {
		for _, team := range resp.Flatten() {
			if team.ID != "" {
				nameByID[team.ID] = team
			}
		}
	}
	rows := make([][]string, 0, minInt(len(pagedRefsItems(paged)), limit))
	for _, ref := range pagedRefsItems(paged) {
		if len(rows) >= limit {
			break
		}
		id := extractIDFromRef(ref.Ref)
		team := nameByID[id]
		rows = append(rows, []string{
			emptyAsDash(id),
			emptyAsDash(teamDisplayName(team)),
			emptyAsDash(team.Abbreviation),
		})
	}
	return SimpleTable{Headers: []string{"ID", "Team", "Abbr"}, Rows: rows}
}

func (c *ESPNClient) LookupTeamHistory(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	if strings.TrimSpace(req.TeamQuery) == "" {
		return nil, fmt.Errorf("%w: history requires team", ErrTeamNotFound)
	}
	team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}
	raw, err := c.client.TeamHistory(ctx, cfg.Sport, cfg.League, team.ID)
	if err != nil {
		if table := c.teamHistoryFallbackTable(ctx, cfg, *team); len(table.Rows) > 0 {
			retrievedAt := c.timeNow()
			return &SportsLookupResult{
				Intent:        SportsIntentTeamHistory,
				League:        cfg.League,
				LeagueName:    cfg.DisplayName,
				LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
				Sport:         cfg.Sport,
				Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Franchise History", teamDisplayName(*team)), table, retrievedAt),
				Source:        SourceESPN,
				RetrievedAt:   retrievedAt,
				RenderMode:    renderMode(req),
			}, nil
		}
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		table = c.teamHistoryFallbackTable(ctx, cfg, *team)
		if len(table.Rows) == 0 {
			return nil, ErrNoSportsData
		}
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentTeamHistory,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Franchise History", teamDisplayName(*team)), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) teamHistoryFallbackTable(ctx context.Context, cfg LeagueConfig, team espn.Team) SimpleTable {
	path := fmt.Sprintf("/apis/site/v2/sports/%s/%s/teams/%s", cfg.Sport, cfg.League, team.ID)
	raw, err := c.client.GetRaw(ctx, espn.DomainSite, path, nil)
	if err != nil {
		return SimpleTable{}
	}
	var payload struct {
		Team struct {
			ID               string `json:"id"`
			UID              string `json:"uid"`
			Slug             string `json:"slug"`
			Location         string `json:"location"`
			Name             string `json:"name"`
			DisplayName      string `json:"displayName"`
			ShortDisplayName string `json:"shortDisplayName"`
			Abbreviation     string `json:"abbreviation"`
			IsActive         bool   `json:"isActive"`
			Franchise        struct {
				ID          string      `json:"id"`
				DisplayName string      `json:"displayName"`
				Slug        string      `json:"slug"`
				IsActive    bool        `json:"isActive"`
				Venue       *espn.Venue `json:"venue"`
			} `json:"franchise"`
			StandingSummary string `json:"standingSummary"`
		} `json:"team"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SimpleTable{}
	}
	t := payload.Team
	name := firstNonEmpty(t.DisplayName, teamDisplayName(team))
	rows := [][]string{
		{"Team", emptyAsDash(name)},
		{"Abbreviation", emptyAsDash(firstNonEmpty(t.Abbreviation, team.Abbreviation))},
		{"Franchise", emptyAsDash(firstNonEmpty(t.Franchise.DisplayName, name))},
		{"Active", fmt.Sprintf("%t", t.IsActive)},
	}
	if t.Franchise.Venue != nil && strings.TrimSpace(t.Franchise.Venue.FullName) != "" {
		rows = append(rows, []string{"Home venue", t.Franchise.Venue.FullName})
	}
	if strings.TrimSpace(t.StandingSummary) != "" {
		rows = append(rows, []string{"Standing summary", t.StandingSummary})
	}
	return SimpleTable{Headers: []string{"Field", "Value"}, Rows: rows}
}

func (c *ESPNClient) LookupSeasons(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	paged, err := c.client.Seasons(ctx, cfg.Sport, cfg.League, limit)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := refsIDTable(paged, "Season", limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentSeasons,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Seasons", cfg.DisplayName), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) LookupTournaments(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	norm := normalizeText(req.RawQuery)
	majorsOnly := hasAnyPhrase(norm, "majors only", "major only", "major tournaments") ||
		(hasPhrase(norm, "majors") && cfg.League == espn.LeaguePGA)
	raw, err := c.client.Tournaments(ctx, cfg.Sport, cfg.League, majorsOnly)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := refsIDTableFromRaw(raw, "Tournament", req.Limit)
	if len(table.Rows) == 0 {
		table = rawJSONTable(raw, req.Limit)
	}
	if len(table.Rows) == 0 && hasPhrase(norm, "calendar") {
		raw, err = c.client.CoreCalendar(ctx, cfg.Sport, cfg.League)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		table = rawJSONTable(raw, req.Limit)
	}
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	title := fmt.Sprintf("### %s Tournaments", cfg.DisplayName)
	if majorsOnly {
		title = fmt.Sprintf("### %s Major Tournaments", cfg.DisplayName)
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentTournaments,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(title, table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) LookupFantasy(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	game := fantasyGameForQuery(req.RawQuery)
	season := req.Season
	if season <= 0 {
		season = c.timeNow().Year()
	}
	leagueID := firstNonEmpty(strings.TrimSpace(req.TeamQuery), fantasyLeagueIDFromQuery(req.RawQuery))
	if req.GameDetailSubtype != "player_info" && leagueID == "" {
		return nil, ErrNoSportsData
	}
	if leagueID == "" {
		leagueID = "0"
	}

	var raw json.RawMessage
	var err error
	title := fmt.Sprintf("### ESPN Fantasy %s League %s", fantasyGameDisplay(game), leagueID)
	if req.GameDetailSubtype == "player_info" {
		raw, err = c.client.FantasyPlayerInfo(ctx, game, season, leagueID)
		title = fmt.Sprintf("### ESPN Fantasy %s Player Info", fantasyGameDisplay(game))
	} else {
		raw, err = c.client.FantasyLeague(ctx, game, season, leagueID, &espn.FantasyLeagueOptions{
			Views: []espn.FantasyView{espn.FantasyViewSettings, espn.FantasyViewStatus, espn.FantasyViewStandings},
		})
	}
	if err != nil {
		return c.fantasyAvailabilityResult(req, game, season, leagueID, title), nil
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return c.fantasyAvailabilityResult(req, game, season, leagueID, title), nil
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentFantasy,
		Markdown:    RenderSimpleMarkdown(title, table, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}

func (c *ESPNClient) fantasyAvailabilityResult(req SportsRequest, game espn.FantasyGame, season int, leagueID string, title string) *SportsLookupResult {
	status := "Public fantasy league unavailable"
	detail := fmt.Sprintf("ESPN did not expose public fantasy data for %s league %s in season %d.", fantasyGameDisplay(game), leagueID, season)
	if req.GameDetailSubtype == "player_info" {
		status = "Fantasy player-info unavailable"
		detail = fmt.Sprintf("ESPN did not expose public %s player-info rows without an accessible league context for season %d.", fantasyGameDisplay(game), season)
	}
	table := SimpleTable{
		Headers: []string{"Status", "Detail"},
		Rows:    [][]string{{status, detail}},
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentFantasy,
		Markdown:    RenderSimpleMarkdown(title, table, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}
}

func pagedRefsItems(paged *espn.PagedRefs) []espn.Ref {
	if paged == nil {
		return nil
	}
	return paged.Items
}

func refsIDTable(paged *espn.PagedRefs, label string, limit int) SimpleTable {
	if limit <= 0 {
		limit = 100
	}
	rows := make([][]string, 0, minInt(len(pagedRefsItems(paged)), limit))
	for _, ref := range pagedRefsItems(paged) {
		if len(rows) >= limit {
			break
		}
		id := extractIDFromRef(ref.Ref)
		if id == "" {
			continue
		}
		rows = append(rows, []string{strconv.Itoa(len(rows) + 1), id})
	}
	return SimpleTable{Headers: []string{"#", label}, Rows: rows}
}

func refsIDTableFromRaw(raw json.RawMessage, label string, limit int) SimpleTable {
	var paged espn.PagedRefs
	if err := json.Unmarshal(raw, &paged); err != nil {
		return SimpleTable{}
	}
	return refsIDTable(&paged, label, limit)
}

func fantasyLeagueIDFromQuery(raw string) string {
	for _, token := range strings.Fields(normalizeText(raw)) {
		if _, err := strconv.Atoi(token); err == nil {
			return token
		}
	}
	return ""
}

func fantasyGameForQuery(raw string) espn.FantasyGame {
	norm := normalizeText(raw)
	switch {
	case hasAnyPhrase(norm, "fantasy baseball", "baseball"):
		return espn.FantasyBaseball
	case hasAnyPhrase(norm, "fantasy basketball", "basketball"):
		return espn.FantasyBasketball
	case hasAnyPhrase(norm, "fantasy hockey", "hockey"):
		return espn.FantasyHockey
	default:
		return espn.FantasyFootball
	}
}

func fantasyGameDisplay(game espn.FantasyGame) string {
	switch game {
	case espn.FantasyBaseball:
		return "Baseball"
	case espn.FantasyBasketball:
		return "Basketball"
	case espn.FantasyHockey:
		return "Hockey"
	default:
		return "Football"
	}
}
