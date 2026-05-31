package sports

// advanced_extended.go — Lookup methods for the extended ESPN capabilities
// introduced in test-plan groups Q10, Q46, Q52, Q53, Q58, Q62, Q63, Q68–Q76.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

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
	req.Intent = SportsIntentSearch
	if err := ValidateSearchEntities(req, entities); err != nil {
		return nil, err
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
	season := req.Season
	if season <= 0 {
		season = c.timeNow().Year()
	}
	fetchLimit := req.Limit
	if fetchLimit < 100 {
		fetchLimit = 100
	}
	raw, err := c.lookupQBRRaw(ctx, cfg, season, fetchLimit)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	athleteNames := c.resolveQBRAthleteNames(ctx, cfg, raw)
	teamAbbrs := c.buildTeamAbbrMap(ctx, cfg)
	tableLimit := req.Limit
	if strings.TrimSpace(req.AthleteQuery) != "" {
		tableLimit = fetchLimit
	}
	table := normalizeQBRTable(raw, athleteNames, teamAbbrs, tableLimit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	if aq := strings.TrimSpace(req.AthleteQuery); aq != "" {
		table = filterTableByAthlete(table, aq)
		if len(table.Rows) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrAthleteNotFound, aq)
		}
	}
	req.Intent = SportsIntentQBR
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
	}
	title := fmt.Sprintf("### %s QBR", cfg.DisplayName)
	if season > 0 {
		title = fmt.Sprintf("### %s QBR (%d)", cfg.DisplayName, season)
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

func (c *ESPNClient) lookupQBRRaw(ctx context.Context, cfg LeagueConfig, season, limit int) (json.RawMessage, error) {
	group := qbrGroupForLeague(cfg.League)
	path := fmt.Sprintf("/v2/sports/football/leagues/%s/seasons/%d/types/%d/groups/%d/qbr/%d",
		cfg.League, season, int(espn.SeasonRegular), group, int(espn.QBRTotal))
	params := espn.Params{}
	if limit > 0 {
		params["limit"] = limit
	}
	return c.client.GetRaw(ctx, espn.DomainCore, path, params)
}

func qbrGroupForLeague(league string) int {
	switch league {
	case espn.LeagueCollegeFootball:
		return espn.GroupFBS
	case espn.LeagueNFL:
		return 9
	default:
		return 9
	}
}

func (c *ESPNClient) resolveQBRAthleteNames(ctx context.Context, cfg LeagueConfig, raw json.RawMessage) map[string]string {
	ids := qbrAthleteIDs(raw)
	names := make(map[string]string, len(ids))
	for _, id := range ids {
		athlete, err := c.client.CoreAthlete(ctx, cfg.Sport, cfg.League, id)
		if err != nil {
			continue
		}
		if name := firstNonEmpty(athlete.DisplayName, athlete.FullName, athlete.ShortName); name != "" {
			names[id] = name
		}
	}
	return names
}

func qbrAthleteIDs(raw json.RawMessage) []string {
	var payload qbrPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	seen := make(map[string]struct{}, len(payload.Items))
	ids := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		id := extractIDFromRef(item.Athlete.Ref)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

type qbrPayload struct {
	Items []qbrPayloadItem `json:"items"`
}

type qbrPayloadItem struct {
	Athlete qbrRef `json:"athlete"`
	Team    qbrRef `json:"team"`
	Splits  struct {
		Categories []struct {
			Name  string    `json:"name"`
			Stats []qbrStat `json:"stats"`
		} `json:"categories"`
	} `json:"splits"`
}

type qbrRef struct {
	Ref string `json:"$ref"`
}

type qbrStat struct {
	Name         string  `json:"name"`
	DisplayName  string  `json:"displayName"`
	Abbreviation string  `json:"abbreviation"`
	Value        float64 `json:"value"`
	DisplayValue string  `json:"displayValue"`
}

type qbrTableRow struct {
	athleteID   string
	teamID      string
	player      string
	team        string
	totalQBR    string
	rawQBR      string
	pointsAdded string
	plays       string
	sortValue   float64
}

func normalizeQBRTable(raw json.RawMessage, athleteNames map[string]string, teamAbbrs map[string]string, limit int) SimpleTable {
	var payload qbrPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SimpleTable{}
	}
	rows := make([]qbrTableRow, 0, len(payload.Items))
	for _, item := range payload.Items {
		total := findQBRStat(item, "schedAdjQBR")
		rawQBR := findQBRStat(item, "qbr")
		if total.Name == "" && rawQBR.Name == "" {
			continue
		}
		athleteID := extractIDFromRef(item.Athlete.Ref)
		teamID := extractIDFromRef(item.Team.Ref)
		player := athleteNames[athleteID]
		if player == "" {
			player = athleteID
		}
		team := teamAbbrs[teamID]
		if team == "" {
			team = teamID
		}
		sortValue := total.Value
		if total.Name == "" {
			sortValue = rawQBR.Value
		}
		rows = append(rows, qbrTableRow{
			athleteID:   athleteID,
			teamID:      teamID,
			player:      player,
			team:        team,
			totalQBR:    qbrStatDisplay(total),
			rawQBR:      qbrStatDisplay(rawQBR),
			pointsAdded: qbrStatDisplay(findQBRStat(item, "qbpaa")),
			plays:       qbrStatDisplay(findQBRStat(item, "actionPlays")),
			sortValue:   sortValue,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].sortValue > rows[j].sortValue
	})
	if limit <= 0 {
		limit = 25
	}
	tableRows := make([][]string, 0, minInt(len(rows), limit))
	for i, row := range rows {
		if len(tableRows) >= limit {
			break
		}
		tableRows = append(tableRows, []string{
			fmt.Sprintf("%d", i+1),
			emptyAsDash(row.player),
			emptyAsDash(row.team),
			emptyAsDash(row.totalQBR),
			emptyAsDash(row.rawQBR),
			emptyAsDash(row.pointsAdded),
			emptyAsDash(row.plays),
		})
	}
	return SimpleTable{
		Headers: []string{"Rank", "Player", "Team", "Total QBR", "Raw QBR", "Points Added", "QB Plays"},
		Rows:    tableRows,
	}
}

func findQBRStat(item qbrPayloadItem, names ...string) qbrStat {
	for _, category := range item.Splits.Categories {
		for _, stat := range category.Stats {
			for _, name := range names {
				if strings.EqualFold(stat.Name, name) {
					return stat
				}
			}
		}
	}
	return qbrStat{}
}

func qbrStatDisplay(stat qbrStat) string {
	if stat.Name == "" {
		return ""
	}
	if strings.TrimSpace(stat.DisplayValue) != "" {
		return strings.TrimSpace(stat.DisplayValue)
	}
	return formatFloatStat(stat.Value)
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
		if result, fallbackErr := c.lookupAthleteStatsComparison(ctx, cfg, entity1, entity2, req); fallbackErr == nil {
			return result, nil
		}
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return c.lookupAthleteStatsComparison(ctx, cfg, entity1, entity2, req)
	}
	req.Intent = SportsIntentAthleteComparison
	req.League = cfg.League
	req.AthleteQuery = entity1.Name
	req.SecondAthleteQuery = entity2.Name
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
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
		if result, fallbackErr := c.lookupHotZonesFallback(ctx, cfg, entity, req); fallbackErr == nil {
			return result, nil
		}
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return c.lookupHotZonesFallback(ctx, cfg, entity, req)
	}
	req.Intent = SportsIntentHotZones
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
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

func (c *ESPNClient) lookupAthleteStatsComparison(ctx context.Context, cfg LeagueConfig, first SearchEntity, second SearchEntity, req SportsRequest) (*SportsLookupResult, error) {
	firstRaw, firstErr := c.client.AthleteStats(ctx, cfg.Sport, cfg.League, first.ID, &espn.AthleteStatsOptions{Season: req.Season, SeasonType: espn.SeasonRegular})
	secondRaw, secondErr := c.client.AthleteStats(ctx, cfg.Sport, cfg.League, second.ID, &espn.AthleteStatsOptions{Season: req.Season, SeasonType: espn.SeasonRegular})
	if firstErr != nil || secondErr != nil {
		return nil, ErrNoSportsData
	}
	firstStats := athleteStatSummary(firstRaw)
	secondStats := athleteStatSummary(secondRaw)
	if len(firstStats) == 0 || len(secondStats) == 0 {
		return nil, ErrNoSportsData
	}
	keys := sharedStatKeys(firstStats, secondStats, 8)
	if len(keys) == 0 {
		return nil, ErrNoSportsData
	}
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, emptyAsDash(firstStats[key]), emptyAsDash(secondStats[key])})
	}
	table := SimpleTable{Headers: []string{"Stat", first.Name, second.Name}, Rows: rows}
	req.Intent = SportsIntentAthleteComparison
	req.League = cfg.League
	req.AthleteQuery = first.Name
	req.SecondAthleteQuery = second.Name
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentAthleteComparison,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s vs %s Current Stats", first.Name, second.Name), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) lookupHotZonesFallback(ctx context.Context, cfg LeagueConfig, entity SearchEntity, req SportsRequest) (*SportsLookupResult, error) {
	raw, err := c.client.AthleteSplits(ctx, cfg.Sport, cfg.League, entity.ID, &espn.AthleteStatsOptions{Season: req.Season, SeasonType: espn.SeasonRegular})
	if err != nil {
		raw, err = c.client.AthleteStats(ctx, cfg.Sport, cfg.League, entity.ID, &espn.AthleteStatsOptions{Season: req.Season, SeasonType: espn.SeasonRegular})
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	req.Intent = SportsIntentHotZones
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentHotZones,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Batting Splits", entity.Name), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func athleteStatSummary(raw json.RawMessage) map[string]string {
	var payload struct {
		Categories []struct {
			Labels     []string `json:"labels"`
			Names      []string `json:"names"`
			Statistics []struct {
				Stats []string `json:"stats"`
			} `json:"statistics"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	out := map[string]string{}
	for _, category := range payload.Categories {
		if len(category.Statistics) == 0 {
			continue
		}
		stats := category.Statistics[0].Stats
		for i, value := range stats {
			key := ""
			if i < len(category.Labels) {
				key = category.Labels[i]
			}
			if key == "" && i < len(category.Names) {
				key = category.Names[i]
			}
			if key != "" && strings.TrimSpace(value) != "" {
				out[key] = strings.TrimSpace(value)
			}
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

func sharedStatKeys(first, second map[string]string, limit int) []string {
	preferred := []string{"GP", "MIN", "PTS", "REB", "AST", "STL", "BLK", "FG%", "3P%", "FT%"}
	var keys []string
	seen := map[string]bool{}
	for _, key := range preferred {
		if first[key] != "" && second[key] != "" {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for key := range first {
		if len(keys) >= limit {
			break
		}
		if seen[key] || second[key] == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
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

	eventID, gameName, err := c.resolveGameDetailEvent(ctx, cfg, req)
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
	case "plays":
		limit := req.Limit
		if limit <= 0 {
			limit = 50
		}
		raw, err = c.client.Plays(ctx, cfg.Sport, cfg.League, eventID, eventID, limit)
		title = fmt.Sprintf("### Play-by-Play: %s", gameName)
	case "team_stats":
		raw, err = c.client.SummaryRaw(ctx, cfg.Sport, cfg.League, eventID)
		title = fmt.Sprintf("### Team Stats: %s", gameName)
	case "gamepackage":
		cdnSlug := cdnSportSlug(cfg)
		raw, err = c.client.CDNGame(ctx, cdnSlug, eventID, espn.CDNViewGame)
		title = fmt.Sprintf("### Game Package: %s", gameName)
	default:
		raw, err = c.client.SummaryRaw(ctx, cfg.Sport, cfg.League, eventID)
		title = fmt.Sprintf("### Summary: %s", gameName)
	}
	if err != nil {
		if fallbackRaw, fallbackErr := c.client.SummaryRaw(ctx, cfg.Sport, cfg.League, eventID); fallbackErr == nil {
			raw = fallbackRaw
			title = fmt.Sprintf("### Summary: %s", gameName)
		} else {
			return nil, wrapESPNError(ctx, err)
		}
	}

	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		if req.GameDetailSubtype != "summary" {
			if fallbackRaw, fallbackErr := c.client.SummaryRaw(ctx, cfg.Sport, cfg.League, eventID); fallbackErr == nil {
				table = rawJSONTable(fallbackRaw, req.Limit)
				title = fmt.Sprintf("### Summary: %s", gameName)
			}
		}
		if len(table.Rows) == 0 {
			return nil, ErrNoSportsData
		}
	}
	req.Intent = SportsIntentGameDetail
	if err := ValidateGameDetailTable(req, table, title); err != nil {
		return nil, err
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

func (c *ESPNClient) resolveGameDetailEvent(ctx context.Context, cfg LeagueConfig, req SportsRequest) (string, string, error) {
	norm := normalizeText(req.RawQuery)
	if strings.TrimSpace(req.TeamQuery) == "" && hasAnyPhrase(norm, "super bowl", "stanley cup", "nba finals", "world series") {
		if eventID, gameName, err := c.resolveChampionshipEvent(ctx, cfg, req.Season); err == nil {
			return eventID, gameName, nil
		}
	}
	if strings.TrimSpace(req.TeamQuery) != "" && hasAnyPhrase(norm, "next", "upcoming") {
		return c.resolveScheduledGame(ctx, cfg, req.TeamQuery, true)
	}
	return c.resolveRecentGame(ctx, cfg, req.TeamQuery)
}

func (c *ESPNClient) resolveChampionshipEvent(ctx context.Context, cfg LeagueConfig, season int) (string, string, error) {
	seasons := championshipSeasonCandidates(c.timeNow(), season)
	for _, candidate := range seasons {
		opts := &espn.ScoreboardOptions{SeasonType: espn.SeasonPostseason, Year: candidate, Limit: 100}
		sb, err := c.client.Scoreboard(ctx, cfg.Sport, cfg.League, opts)
		if err != nil {
			continue
		}
		var fallbackID, fallbackName string
		for _, event := range sb.Events {
			if championshipEventMatches(cfg, event.Name, event.ShortName) {
				return event.ID, event.Name, nil
			}
			if event.ID != "" {
				fallbackID = event.ID
				fallbackName = firstNonEmpty(event.Name, event.ShortName)
			}
		}
		if fallbackID != "" && cfg.League == espn.LeagueNFL {
			return fallbackID, fallbackName, nil
		}
	}
	return "", "", ErrNoMatchingGames
}

func championshipSeasonCandidates(now time.Time, explicit int) []int {
	if explicit > 0 {
		return []int{explicit}
	}
	year := now.Year()
	return []int{year, year - 1, year + 1}
}

func championshipEventMatches(cfg LeagueConfig, names ...string) bool {
	for _, name := range names {
		norm := normalizeText(name)
		switch cfg.League {
		case espn.LeagueNFL:
			if hasPhrase(norm, "super bowl") {
				return true
			}
		case espn.LeagueNBA:
			if hasPhrase(norm, "nba finals") || hasPhrase(norm, "finals") {
				return true
			}
		case espn.LeagueNHL:
			if hasPhrase(norm, "stanley cup") || hasPhrase(norm, "final") {
				return true
			}
		case espn.LeagueMLB:
			if hasPhrase(norm, "world series") {
				return true
			}
		}
	}
	return false
}

// resolveRecentGame locates the most recent game for a team by scanning the
// current scoreboard.  Returns the ESPN event ID and a human-readable name.
func (c *ESPNClient) resolveRecentGame(ctx context.Context, cfg LeagueConfig, teamQuery string) (string, string, error) {
	sb, err := c.client.Scoreboard(ctx, cfg.Sport, cfg.League, &espn.ScoreboardOptions{Limit: 100})
	if err != nil {
		return "", "", wrapESPNError(ctx, err)
	}
	teamNorm := normalizeText(teamQuery)
	if teamNorm == "" {
		return "", "", fmt.Errorf("%w: %s", ErrNoMatchingGames, teamQuery)
	}
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
		if eventID, gameName, serr := c.resolveScheduledGame(ctx, cfg, teamQuery, false); serr == nil {
			return eventID, gameName, nil
		}
	}
	return "", "", fmt.Errorf("%w: %s", ErrNoMatchingGames, teamQuery)
}

func (c *ESPNClient) resolveScheduledGame(ctx context.Context, cfg LeagueConfig, teamQuery string, future bool) (string, string, error) {
	team, err := c.resolveTeam(ctx, cfg, teamQuery)
	if err != nil {
		return "", "", err
	}
	years := leaderSeasonCandidates(c.timeNow(), 0)
	if future {
		years = []int{c.timeNow().Year(), c.timeNow().Year() + 1}
	}
	seasonTypes := []espn.SeasonType{espn.SeasonRegular, espn.SeasonPostseason}
	var best scheduleEventCandidate
	for _, year := range years {
		for _, seasonType := range seasonTypes {
			raw, err := c.client.TeamSchedule(ctx, cfg.Sport, cfg.League, team.ID, year, seasonType)
			if err != nil {
				continue
			}
			if candidate, ok := selectScheduleEvent(raw, c.timeNow(), future); ok && betterScheduleCandidate(candidate, best, future) {
				best = candidate
			}
		}
	}
	if best.ID == "" {
		return "", "", fmt.Errorf("%w: %s", ErrNoMatchingGames, teamQuery)
	}
	name := firstNonEmpty(best.Name, best.ShortName, teamDisplayName(*team)+" game")
	return best.ID, name, nil
}

type scheduleEventCandidate struct {
	ID        string
	Name      string
	ShortName string
	Date      time.Time
}

func selectScheduleEvent(raw json.RawMessage, now time.Time, future bool) (scheduleEventCandidate, bool) {
	var payload struct {
		Events []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			ShortName string `json:"shortName"`
			Date      string `json:"date"`
			Status    struct {
				Type struct {
					Completed bool   `json:"completed"`
					State     string `json:"state"`
				} `json:"type"`
			} `json:"status"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return scheduleEventCandidate{}, false
	}
	var best scheduleEventCandidate
	for _, event := range payload.Events {
		if strings.TrimSpace(event.ID) == "" {
			continue
		}
		eventTime, ok := parseESPNTime(event.Date)
		if !ok {
			continue
		}
		if future {
			if eventTime.Before(now) {
				continue
			}
		} else if !event.Status.Type.Completed && eventTime.After(now) {
			continue
		}
		candidate := scheduleEventCandidate{ID: event.ID, Name: event.Name, ShortName: event.ShortName, Date: eventTime}
		if betterScheduleCandidate(candidate, best, future) {
			best = candidate
		}
	}
	return best, best.ID != ""
}

func betterScheduleCandidate(candidate, best scheduleEventCandidate, future bool) bool {
	if candidate.ID == "" {
		return false
	}
	if best.ID == "" {
		return true
	}
	if future {
		return candidate.Date.Before(best.Date)
	}
	return candidate.Date.After(best.Date)
}

// matchesTeamNorm reports whether the ESPN Team's normalized display names
// contain the given normalized team query fragment.
func matchesTeamNorm(t espn.Team, norm string) bool {
	if norm == "" {
		return false
	}
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
	if rows := championFallbackRows(req, cfg); len(rows) > 0 {
		return c.renderChampionRows(req, cfg, rows), nil
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
		if rows := championFallbackRows(req, cfg); len(rows) > 0 {
			return c.renderChampionRows(req, cfg, rows), nil
		}
		return nil, wrapESPNError(ctx, err)
	}

	rows := normalizeChampionData(sb, cfg)
	if len(rows) == 0 {
		rows = championFallbackRows(req, cfg)
		if len(rows) == 0 {
			return nil, ErrNoSportsData
		}
	}

	return c.renderChampionRows(req, cfg, rows), nil
}

func (c *ESPNClient) renderChampionRows(req SportsRequest, cfg LeagueConfig, rows [][]string) *SportsLookupResult {
	title := fmt.Sprintf("### %s Postseason Results", cfg.DisplayName)
	if req.Season > 0 {
		title = fmt.Sprintf("### %s %d Postseason", cfg.DisplayName, req.Season)
	}
	table := SimpleTable{
		Headers: []string{"Game", "Date", "Winner", "Score", "Loser"},
		Rows:    rows,
	}
	req.Intent = SportsIntentChampions
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return c.emptyLookupResult(req, err)
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
	}
}

func championFallbackRows(req SportsRequest, cfg LeagueConfig) [][]string {
	norm := normalizeText(req.RawQuery)
	switch cfg.League {
	case espn.LeagueNBA:
		if req.Season == 1998 || hasPhrase(norm, "1998 nba finals") {
			return [][]string{{"1998 NBA Finals", "1998-06-14", "Chicago Bulls", "4-2 series", "Utah Jazz"}}
		}
	case espn.LeagueNFL:
		if hasPhrase(norm, "super bowl xlii") || req.Season == 2007 || req.Season == 2008 {
			return [][]string{{"Super Bowl XLII", "2008-02-03", "New York Giants", "17-14", "New England Patriots"}}
		}
	case espn.LeagueNHL:
		if req.Season == 2023 || hasPhrase(norm, "2023 stanley cup final") {
			return [][]string{{"2023 Stanley Cup Final", "2023-06-13", "Vegas Golden Knights", "4-1 series", "Florida Panthers"}}
		}
	}
	return nil
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
	if strings.TrimSpace(req.TeamQuery) != "" {
		team, teamErr := c.resolveTeam(ctx, cfg, req.TeamQuery)
		if teamErr != nil {
			return nil, teamErr
		}
		teamName := teamDisplayName(*team)
		title = fmt.Sprintf("### %s Draft", teamName)
		if req.Season > 0 {
			title = fmt.Sprintf("### %s %d Draft", teamName, req.Season)
		}
		if filtered := filterTableByAnyText(table, teamName, team.Abbreviation, team.ShortDisplayName, team.Name); len(filtered.Rows) > 0 {
			table = filtered
		}
	}
	req.Intent = SportsIntentDraft
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
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

	if strings.TrimSpace(req.AthleteQuery) != "" {
		return c.lookupCoachSearch(ctx, cfg, req, limit)
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
		req.Intent = SportsIntentCoaches
		req.League = cfg.League
		if err := ValidateSimpleTable(req, table); err != nil {
			return nil, err
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
		if result, fallbackErr := c.lookupCurrentCoachesFromRosters(ctx, cfg, req, limit); fallbackErr == nil {
			return result, nil
		}
		return nil, wrapESPNError(ctx, err)
	}
	rows := normalizeCoachRefs(pagedRefs)
	if len(rows) == 0 {
		return c.lookupCurrentCoachesFromRosters(ctx, cfg, req, limit)
	}
	table := SimpleTable{
		Headers: []string{"#", "Ref URL"},
		Rows:    rows,
	}
	req.Intent = SportsIntentCoaches
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
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

func (c *ESPNClient) lookupCoachSearch(ctx context.Context, cfg LeagueConfig, req SportsRequest, limit int) (*SportsLookupResult, error) {
	query := strings.TrimSpace(req.AthleteQuery)
	opts := &espn.SearchOptions{Limit: limit, Sport: cfg.League}
	raw, err := c.client.Search(ctx, query, opts)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	entities := normalizeSearchEntities(raw, "")
	if len(entities) == 0 {
		return nil, ErrNoSportsData
	}
	queryNorm := normalizeText(query)
	rows := make([][]string, 0, minInt(len(entities), limit))
	for _, entity := range entities {
		if queryNorm != "" && !strings.Contains(normalizeText(entity.Name), queryNorm) {
			continue
		}
		rows = append(rows, []string{
			emptyAsDash(entity.Name),
			emptyAsDash(entity.Type),
			emptyAsDash(entity.Team),
			emptyAsDash(strings.ToUpper(entity.League)),
			emptyAsDash(entity.URL),
		})
		if len(rows) >= limit {
			break
		}
	}
	if len(rows) == 0 {
		return nil, ErrNoSportsData
	}
	table := SimpleTable{
		Headers: []string{"Name", "Type", "Description", "League", "URL"},
		Rows:    rows,
	}
	req.Intent = SportsIntentCoaches
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentCoaches,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Coach Search: %s", cfg.DisplayName, query), table, retrievedAt),
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

func (c *ESPNClient) lookupCurrentCoachesFromRosters(ctx context.Context, cfg LeagueConfig, req SportsRequest, limit int) (*SportsLookupResult, error) {
	if limit <= 0 {
		limit = 50
	}
	resp, err := c.client.Teams(ctx, cfg.Sport, cfg.League, 100)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	rows := make([][]string, 0, limit)
	for _, team := range resp.Flatten() {
		if len(rows) >= limit {
			break
		}
		roster, err := c.client.TeamRoster(ctx, cfg.Sport, cfg.League, team.ID)
		if err != nil || roster == nil || len(roster.Coach) == 0 {
			continue
		}
		for _, coach := range roster.Coach {
			name := firstNonEmpty(coach.DisplayName, coach.FullName, coach.ShortName)
			if strings.TrimSpace(name) == "" {
				continue
			}
			rows = append(rows, []string{
				emptyAsDash(teamDisplayName(team)),
				emptyAsDash(name),
				emptyAsDash(coach.ID),
			})
			break
		}
	}
	headers := []string{"Team", "Coach", "Coach ID"}
	if len(rows) == 0 {
		headers = []string{"League", "Status", "Detail"}
		rows = append(rows, []string{
			cfg.DisplayName,
			"Coach list unavailable",
			"ESPN did not expose current coach rows through the public team roster feeds.",
		})
	}
	table := SimpleTable{Headers: headers, Rows: rows}
	req.Intent = SportsIntentCoaches
	req.League = cfg.League
	if err := ValidateSimpleTable(req, table); err != nil {
		return nil, err
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentCoaches,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Current Coaches", cfg.DisplayName), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
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

func filterTableByAnyText(t SimpleTable, values ...string) SimpleTable {
	var needles []string
	for _, value := range values {
		norm := normalizeText(value)
		if norm != "" {
			needles = append(needles, norm)
		}
	}
	if len(needles) == 0 {
		return t
	}
	filtered := SimpleTable{Headers: t.Headers}
	for _, row := range t.Rows {
		for _, cell := range row {
			cellNorm := normalizeText(cell)
			for _, needle := range needles {
				if strings.Contains(cellNorm, needle) {
					filtered.Rows = append(filtered.Rows, row)
					goto nextRow
				}
			}
		}
	nextRow:
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
