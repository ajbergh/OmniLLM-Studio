package sports

// advanced_extended.go — Lookup methods for the extended ESPN capabilities
// introduced in test-plan groups Q10, Q46, Q52, Q53, Q58, Q62, Q63, Q68–Q76.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	espn "github.com/chinmaykhachane/espn-go"
)

// ─── Q10: ESPN Search ────────────────────────────────────────────────────────

// LookupSearch executes a free-text ESPN search and returns results formatted
// as a markdown list.  AthleteQuery holds the search term.
func (c *ESPNClient) LookupSearch(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	query := strings.TrimSpace(req.AthleteQuery)
	if query == "" {
		query = req.RawQuery
	}
	opts := &espn.SearchOptions{Limit: 10}
	if req.Limit > 0 && req.Limit < 25 {
		opts.Limit = req.Limit
	}
	raw, err := c.client.Search(ctx, query, opts)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	entities := normalizeSearchEntities(raw, "")
	if len(entities) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentSearch,
		Markdown:    renderSearchEntitiesMarkdown(query, entities, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}

// ─── Q46: QBR (Quarterback Rating) ──────────────────────────────────────────

// LookupQBR fetches the ESPN Total QBR leaderboard for the given season/league.
// If AthleteQuery is set the result is filtered to that quarterback's entry.
func (c *ESPNClient) LookupQBR(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		// QBR is NFL-only; default to NFL
		if cfg, ok = leagueConfigByLeague(espn.LeagueNFL); !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
		}
	}
	raw, err := c.client.QBR(ctx, cfg.League, req.Season, nil)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	if aq := strings.TrimSpace(req.AthleteQuery); aq != "" {
		table = filterTableByAthlete(table, aq)
		if len(table.Rows) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrAthleteNotFound, aq)
		}
	}
	title := fmt.Sprintf("### %s QBR", cfg.DisplayName)
	if req.Season > 0 {
		title = fmt.Sprintf("### %s QBR (%d)", cfg.DisplayName, req.Season)
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentQBR,
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

// ─── Q52: Athlete head-to-head comparison ───────────────────────────────────

// LookupAthleteComparison resolves two athletes and calls CoreAthleteVsAthlete.
func (c *ESPNClient) LookupAthleteComparison(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	// Resolve first athlete
	req1 := req
	req1.AthleteQuery = req.AthleteQuery
	entity1, err := c.resolveAthlete(ctx, req1)
	if err != nil {
		return nil, fmt.Errorf("resolving first athlete %q: %w", req.AthleteQuery, err)
	}
	// Resolve second athlete
	req2 := req
	req2.AthleteQuery = req.SecondAthleteQuery
	entity2, err := c.resolveAthlete(ctx, req2)
	if err != nil {
		return nil, fmt.Errorf("resolving second athlete %q: %w", req.SecondAthleteQuery, err)
	}
	// Determine league config from first athlete
	cfg, ok := leagueConfigByAlias(entity1.League)
	if !ok {
		cfg, ok = leagueConfigForRequest(req)
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, entity1.League)
	}
	raw, err := c.client.CoreAthleteVsAthlete(ctx, cfg.Sport, cfg.League, entity1.ID, entity2.ID)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	title := fmt.Sprintf("### %s vs %s", entity1.Name, entity2.Name)
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentAthleteComparison,
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

// ─── Q53: Hot Zones ──────────────────────────────────────────────────────────

// LookupHotZones resolves an athlete and fetches their ESPN hot-zone data.
func (c *ESPNClient) LookupHotZones(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	entity, err := c.resolveAthlete(ctx, req)
	if err != nil {
		return nil, err
	}
	cfg, ok := leagueConfigByAlias(entity.League)
	if !ok {
		cfg, ok = leagueConfigForRequest(req)
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, entity.League)
	}
	raw, err := c.client.CoreAthleteHotZones(ctx, cfg.Sport, cfg.League, entity.ID)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	title := fmt.Sprintf("### %s Hot Zones", entity.Name)
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentHotZones,
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

// ─── Q58 / Q62 / Q63 / Q68: Game-level detail ───────────────────────────────

// LookupGameDetail finds the most recent (or current) game for the requested
// team and calls the sub-endpoint indicated by GameDetailSubtype:
//   - "officials"     → Officials
//   - "probabilities" → Probabilities (win probability)
//   - "predictor"     → Predictor
//   - "gamepackage"   → CDNGame (full game package)
//   - "summary"       → SummaryRaw (default)
func (c *ESPNClient) LookupGameDetail(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}

	eventID, gameName, err := c.resolveRecentGame(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}

	var raw json.RawMessage
	var title string

	switch req.GameDetailSubtype {
	case "officials":
		raw, err = c.client.Officials(ctx, cfg.Sport, cfg.League, eventID, eventID)
		title = fmt.Sprintf("### Officials: %s", gameName)
	case "probabilities":
		raw, err = c.client.Probabilities(ctx, cfg.Sport, cfg.League, eventID, eventID)
		title = fmt.Sprintf("### Win Probability: %s", gameName)
	case "predictor":
		raw, err = c.client.Predictor(ctx, cfg.Sport, cfg.League, eventID, eventID)
		title = fmt.Sprintf("### ESPN Predictor: %s", gameName)
	case "gamepackage":
		cdnSlug := cdnSportSlug(cfg)
		raw, err = c.client.CDNGame(ctx, cdnSlug, eventID, espn.CDNViewGame)
		title = fmt.Sprintf("### Game Package: %s", gameName)
	default:
		raw, err = c.client.SummaryRaw(ctx, cfg.Sport, cfg.League, eventID)
		title = fmt.Sprintf("### Summary: %s", gameName)
	}
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}

	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentGameDetail,
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

// resolveRecentGame locates the most recent game for a team by scanning the
// current scoreboard.  Returns the ESPN event ID and a human-readable name.
func (c *ESPNClient) resolveRecentGame(ctx context.Context, cfg LeagueConfig, teamQuery string) (string, string, error) {
	sb, err := c.client.Scoreboard(ctx, cfg.Sport, cfg.League, &espn.ScoreboardOptions{Limit: 100})
	if err != nil {
		return "", "", wrapESPNError(ctx, err)
	}
	teamNorm := normalizeText(teamQuery)
	for _, event := range sb.Events {
		for _, comp := range event.Competitions {
			for _, competitor := range comp.Competitors {
				tn := competitor.Team
				if matchesTeamNorm(tn, teamNorm) {
					return event.ID, event.Name, nil
				}
			}
		}
	}
	// Fall back: try team schedule for most recent completed game
	if teamQuery != "" {
		team, terr := c.resolveTeam(ctx, cfg, teamQuery)
		if terr == nil {
			schedRaw, serr := c.client.TeamSchedule(ctx, cfg.Sport, cfg.League, team.ID, 0, espn.SeasonRegular)
			if serr == nil {
				if eid := extractMostRecentEventID(schedRaw); eid != "" {
					return eid, teamDisplayName(*team) + " (most recent game)", nil
				}
			}
		}
	}
	return "", "", fmt.Errorf("%w: %s", ErrNoMatchingGames, teamQuery)
}

// matchesTeamNorm reports whether the ESPN Team's normalized display names
// contain the given normalized team query fragment.
func matchesTeamNorm(t espn.Team, norm string) bool {
	for _, s := range []string{t.DisplayName, t.ShortDisplayName, t.Location, t.Name, t.Nickname} {
		if strings.Contains(normalizeText(s), norm) {
			return true
		}
	}
	return false
}

// extractMostRecentEventID parses a raw team-schedule JSON and returns the
// most recent completed event's ID, or "" if none can be found.
func extractMostRecentEventID(raw json.RawMessage) string {
	var payload struct {
		Events []struct {
			ID     string `json:"id"`
			Status struct {
				Type struct {
					Completed bool `json:"completed"`
				} `json:"type"`
			} `json:"status"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	last := ""
	for _, ev := range payload.Events {
		if ev.Status.Type.Completed {
			last = ev.ID
		}
	}
	return last
}

// cdnSportSlug converts an ESPN LeagueConfig to the CDN sport slug required
// by CDNGame (e.g. "nfl" instead of "football").
func cdnSportSlug(cfg LeagueConfig) string {
	switch cfg.League {
	case espn.LeagueNFL:
		return "nfl"
	case espn.LeagueNBA:
		return "nba"
	case espn.LeagueMLB:
		return "mlb"
	case espn.LeagueNHL:
		return "nhl"
	case espn.LeagueCollegeFootball:
		return "college-football"
	case espn.LeagueMensCollegeBball:
		return "mens-college-basketball"
	default:
		return cfg.League
	}
}

// ─── Q69–Q72: Champions history ──────────────────────────────────────────────

// LookupChampions fetches the postseason scoreboard for the requested season
// and identifies the championship game winner.  When Season is 0 the current
// or most recent postseason is used.
func (c *ESPNClient) LookupChampions(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}

	opts := &espn.ScoreboardOptions{
		SeasonType: espn.SeasonPostseason,
		Limit:      100,
	}
	if req.Season > 0 {
		opts.Year = req.Season
	}
	sb, err := c.client.Scoreboard(ctx, cfg.Sport, cfg.League, opts)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}

	rows := normalizeChampionData(sb, cfg)
	if len(rows) == 0 {
		return nil, ErrNoSportsData
	}

	title := fmt.Sprintf("### %s Postseason Results", cfg.DisplayName)
	if req.Season > 0 {
		title = fmt.Sprintf("### %s %d Postseason", cfg.DisplayName, req.Season)
	}
	table := SimpleTable{
		Headers: []string{"Game", "Date", "Winner", "Score", "Loser"},
		Rows:    rows,
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentChampions,
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

// normalizeChampionData extracts winner information from postseason scoreboard
// events.  Each completed event becomes one row.
func normalizeChampionData(sb *espn.Scoreboard, cfg LeagueConfig) [][]string {
	if sb == nil {
		return nil
	}
	var rows [][]string
	for _, event := range sb.Events {
		if !event.Status.Type.Completed {
			continue
		}
		for _, comp := range event.Competitions {
			winner, loser, winScore, loseScore := "", "", "", ""
			for _, competitor := range comp.Competitors {
				name := firstNonEmpty(competitor.Team.DisplayName, competitor.Team.ShortDisplayName)
				score := competitor.Score
				if competitor.Winner {
					winner = name
					winScore = score
				} else {
					loser = name
					loseScore = score
				}
			}
			if winner == "" {
				continue
			}
			date := ""
			if len(comp.Date) >= 10 {
				date = comp.Date[:10]
			}
			rows = append(rows, []string{event.Name, date, winner, winScore + "-" + loseScore, loser})
		}
	}
	return rows
}

// ─── Q73–Q74: Draft ──────────────────────────────────────────────────────────

// LookupDraft fetches draft pick data for the given season.
// Uses SeasonDraft for historical seasons or the site Draft endpoint for the
// current/ongoing draft.
func (c *ESPNClient) LookupDraft(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}

	var raw json.RawMessage
	var err error
	title := fmt.Sprintf("### %s Draft", cfg.DisplayName)

	if req.Season > 0 {
		raw, err = c.client.SeasonDraft(ctx, cfg.Sport, cfg.League, req.Season)
		title = fmt.Sprintf("### %s %d Draft", cfg.DisplayName, req.Season)
	} else {
		// Current draft (live or most recent)
		raw, err = c.client.Draft(ctx, cfg.Sport, cfg.League)
	}
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}

	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentDraft,
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

// ─── Q75–Q76: Coaches ────────────────────────────────────────────────────────

// LookupCoaches fetches the coaching roster for the given league/team.
// The ESPN core API returns paged $ref objects; this function dereferences the
// first page (up to limit) and formats them as a table.
func (c *ESPNClient) LookupCoaches(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	// If a specific team is requested, look up that team's coaches via their
	// TeamRecord endpoint (which returns coaching staff in some leagues) or fall
	// back to the league-wide list.
	if req.TeamQuery != "" {
		team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
		if err != nil {
			return nil, err
		}
		// Use the Coach endpoint to get the team's current coach by ID
		raw, err := c.client.TeamRecord(ctx, cfg.Sport, cfg.League, team.ID)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		table := rawJSONTable(raw, limit)
		if len(table.Rows) == 0 {
			return nil, ErrNoSportsData
		}
		title := fmt.Sprintf("### %s Coaching Staff", teamDisplayName(*team))
		retrievedAt := c.timeNow()
		return &SportsLookupResult{
			Intent:        SportsIntentCoaches,
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

	// League-wide coaches list
	pagedRefs, err := c.client.Coaches(ctx, cfg.Sport, cfg.League, req.Season, limit)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	rows := normalizeCoachRefs(pagedRefs)
	if len(rows) == 0 {
		return nil, ErrNoSportsData
	}
	table := SimpleTable{
		Headers: []string{"#", "Ref URL"},
		Rows:    rows,
	}
	title := fmt.Sprintf("### %s Coaches", cfg.DisplayName)
	if req.Season > 0 {
		title = fmt.Sprintf("### %s %d Coaches", cfg.DisplayName, req.Season)
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentCoaches,
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

// normalizeCoachRefs converts ESPN PagedRefs items to simple table rows.
func normalizeCoachRefs(paged *espn.PagedRefs) [][]string {
	if paged == nil {
		return nil
	}
	rows := make([][]string, 0, len(paged.Items))
	for i, ref := range paged.Items {
		rows = append(rows, []string{fmt.Sprintf("%d", i+1), ref.Ref})
	}
	return rows
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

// filterTableByAthlete filters a SimpleTable keeping only rows where any cell
// contains the normalised athlete name fragment.
func filterTableByAthlete(t SimpleTable, athlete string) SimpleTable {
	norm := normalizeText(athlete)
	filtered := SimpleTable{Headers: t.Headers}
	for _, row := range t.Rows {
		for _, cell := range row {
			if strings.Contains(normalizeText(cell), norm) {
				filtered.Rows = append(filtered.Rows, row)
				break
			}
		}
	}
	return filtered
}

// renderSearchEntitiesMarkdown formats ESPN search results as a markdown list.
func renderSearchEntitiesMarkdown(query string, entities []SearchEntity, retrievedAt interface{}) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### ESPN Search: \"%s\"\n\n", query))
	for _, e := range entities {
		name := e.Name
		if e.Team != "" {
			name += fmt.Sprintf(" (%s)", e.Team)
		}
		if e.League != "" {
			name += fmt.Sprintf(" — %s", strings.ToUpper(e.League))
		}
		if e.URL != "" {
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", name, e.URL))
		} else {
			sb.WriteString(fmt.Sprintf("- %s\n", name))
		}
	}
	return sb.String()
}
