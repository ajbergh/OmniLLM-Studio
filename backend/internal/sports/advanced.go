package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

func (c *ESPNClient) LookupRoster(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	if strings.TrimSpace(req.TeamQuery) == "" {
		return nil, fmt.Errorf("%w: roster requires team", ErrTeamNotFound)
	}
	team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}
	req.TeamQuery = teamDisplayName(*team)
	roster, err := c.client.TeamRoster(ctx, cfg.Sport, cfg.League, team.ID)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	rows := normalizeRoster(roster)
	if len(rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentRoster,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderRosterMarkdown(req, cfg, rows, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) LookupLeaders(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		if metric, metricOK := detectStatMetric(normalizeText(req.RawQuery), LeagueConfig{}, false); metricOK && metric.DefaultLeague != "" {
			cfg, ok = leagueConfigByLeague(metric.DefaultLeague)
			req.StatCategory = metric.Category
			req.StatName = metric.StatName
			req.StatLabel = metric.Label
			req.StatSort = metric.Sort
		}
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	if strings.TrimSpace(req.StatSort) == "" {
		if metric, metricOK := detectStatMetric(normalizeText(req.RawQuery), cfg, true); metricOK {
			req.StatCategory = metric.Category
			req.StatName = metric.StatName
			req.StatLabel = metric.Label
			req.StatSort = metric.Sort
		}
	}
	if strings.TrimSpace(req.StatSort) == "" {
		if strings.TrimSpace(req.TeamQuery) != "" {
			team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
			if err != nil {
				return nil, err
			}
			table, season, err := c.lookupTeamCoreLeadersTable(ctx, cfg, *team, req)
			if err == nil && len(table.Rows) > 0 {
				retrievedAt := c.timeNow()
				title := fmt.Sprintf("### %s Team Leaders", teamDisplayName(*team))
				if season > 0 {
					title = fmt.Sprintf("### %s Team Leaders (%d)", teamDisplayName(*team), season)
				}
				return &SportsLookupResult{
					Intent:        SportsIntentLeaders,
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
		}
		raw, title, err := c.lookupLeadersRaw(ctx, cfg, req)
		if err != nil {
			return nil, err
		}
		table := rawJSONTable(raw, req.Limit)
		if len(table.Rows) == 0 {
			return nil, ErrNoSportsData
		}
		retrievedAt := c.timeNow()
		return &SportsLookupResult{
			Intent:        SportsIntentLeaders,
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
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimitForIntent(SportsIntentLeaders)
	}

	var rows []LeaderboardRow
	var leaderErr error
	if strings.TrimSpace(req.TeamQuery) != "" {
		rows, leaderErr = c.lookupTeamCoreStatLeaders(ctx, cfg, req)
		if leaderErr != nil {
			rows, leaderErr = c.lookupTeamStatLeaders(ctx, cfg, req)
		}
	} else {
		rows, leaderErr = c.lookupLeagueStatLeaders(ctx, cfg, req)
	}
	if leaderErr != nil {
		return nil, leaderErr
	}
	if len(rows) == 0 {
		return nil, ErrNoSportsData
	}
	if req.Season > 0 && req.DateLabel == "" {
		req.DateLabel = strconv.Itoa(req.Season)
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentLeaders,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		DateLabel:     req.DateLabel,
		Markdown:      RenderLeaderboardMarkdown(req, cfg, rows, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) LookupAthlete(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
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
	req.AthleteQuery = entity.Name
	if req.Intent == SportsIntentAthleteNews {
		feed, err := c.client.AthleteNewsSite(ctx, cfg.Sport, cfg.League, entity.ID, req.Limit)
		if err != nil {
			if result, fallbackErr := c.lookupAthleteNewsSearch(ctx, cfg, entity, req); fallbackErr == nil {
				return result, nil
			}
			return nil, wrapESPNError(ctx, err)
		}
		rows := normalizeNewsFeed(feed)
		if len(rows) == 0 {
			return c.lookupAthleteNewsSearch(ctx, cfg, entity, req)
		}
		retrievedAt := c.timeNow()
		return &SportsLookupResult{
			Intent:        SportsIntentAthleteNews,
			League:        cfg.League,
			LeagueName:    cfg.DisplayName,
			LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
			Sport:         cfg.Sport,
			Markdown:      RenderNewsMarkdown(req, cfg.DisplayName, rows, retrievedAt),
			Source:        SourceESPN,
			RetrievedAt:   retrievedAt,
			RenderMode:    renderMode(req),
		}, nil
	}

	raw, title, err := c.lookupAthleteRaw(ctx, cfg, entity.ID, req)
	if err != nil {
		if result, fallbackErr := c.lookupAthleteFallback(ctx, cfg, entity, req); fallbackErr == nil {
			return result, nil
		}
		return nil, err
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		if result, fallbackErr := c.lookupAthleteFallback(ctx, cfg, entity, req); fallbackErr == nil {
			return result, nil
		}
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        req.Intent,
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

func (c *ESPNClient) LookupGeneric(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	switch req.Intent {
	case SportsIntentInjuries:
		if strings.TrimSpace(req.TeamQuery) != "" {
			if result, err := c.lookupTeamInjuries(ctx, cfg, req); err == nil {
				return result, nil
			}
		}
	case SportsIntentTransactions:
		if strings.TrimSpace(req.TeamQuery) != "" {
			if result, err := c.lookupTeamTransactions(ctx, cfg, req); err == nil {
				return result, nil
			}
		}
	}
	raw, title, err := c.lookupGenericRaw(ctx, cfg, req)
	if err != nil {
		return nil, err
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        req.Intent,
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

func (c *ESPNClient) LookupTeamRecord(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}
	st, err := c.client.Standings(ctx, cfg.Sport, cfg.League, req.Season)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	rows := normalizeStandings(st)
	if len(rows) == 0 {
		return nil, ErrNoStandings
	}
	query := normalizeText(teamDisplayName(*team))
	var matched *StandingsRow
	for i := range rows {
		rowTeam := standingsTeam(rows[i])
		fields := []string{rowTeam.DisplayName, rowTeam.ShortName, rowTeam.Abbreviation, rows[i].Team, rows[i].Abbr}
		for _, field := range fields {
			norm := normalizeText(field)
			if norm != "" && (norm == query || strings.Contains(norm, query) || strings.Contains(query, norm)) {
				matched = &rows[i]
				break
			}
		}
		if matched != nil {
			break
		}
	}
	if matched == nil {
		return nil, ErrNoSportsData
	}
	table := SimpleTable{
		Headers: []string{"Team", "W", "L", "T/OTL", "Pct", "GB", "Streak"},
		Rows: [][]string{{
			emptyAsDash(teamDisplayName(*team)),
			emptyAsDash(matched.Wins),
			emptyAsDash(matched.Losses),
			emptyAsDash(firstNonEmpty(matched.Ties, matched.NoResult)),
			emptyAsDash(matched.Pct),
			emptyAsDash(matched.GamesBack),
			emptyAsDash(matched.Streak),
		}},
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentTeamRecord,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Current Record", teamDisplayName(*team)), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) lookupGenericRaw(ctx context.Context, cfg LeagueConfig, req SportsRequest) (json.RawMessage, string, error) {
	switch req.Intent {
	case SportsIntentInjuries:
		if req.TeamQuery != "" {
			team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
			if err != nil {
				return nil, "", err
			}
			raw, err := c.client.TeamInjuries(ctx, cfg.Sport, cfg.League, team.ID)
			return raw, fmt.Sprintf("### %s Injuries", teamDisplayName(*team)), wrapESPNError(ctx, err)
		}
		raw, err := c.client.LeagueInjuries(ctx, cfg.Sport, cfg.League)
		return raw, fmt.Sprintf("### %s Injuries", cfg.DisplayName), wrapESPNError(ctx, err)
	case SportsIntentTransactions:
		if req.TeamQuery != "" {
			team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
			if err != nil {
				return nil, "", err
			}
			raw, err := c.client.TeamTransactions(ctx, cfg.Sport, cfg.League, team.ID)
			return raw, fmt.Sprintf("### %s Transactions", teamDisplayName(*team)), wrapESPNError(ctx, err)
		}
		raw, err := c.client.LeagueTransactions(ctx, cfg.Sport, cfg.League)
		return raw, fmt.Sprintf("### %s Transactions", cfg.DisplayName), wrapESPNError(ctx, err)
	case SportsIntentTeamRecord:
		team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
		if err != nil {
			return nil, "", err
		}
		raw, err := c.client.TeamRecord(ctx, cfg.Sport, cfg.League, team.ID)
		return raw, fmt.Sprintf("### %s Record", teamDisplayName(*team)), wrapESPNError(ctx, err)
	case SportsIntentTeamSchedule:
		team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
		if err != nil {
			return nil, "", err
		}
		raw, err := c.client.TeamSchedule(ctx, cfg.Sport, cfg.League, team.ID, req.Season, espn.SeasonRegular)
		return raw, fmt.Sprintf("### %s Schedule", teamDisplayName(*team)), wrapESPNError(ctx, err)
	case SportsIntentRankings:
		raw, err := c.client.Rankings(ctx, cfg.Sport, cfg.League)
		return raw, fmt.Sprintf("### %s Rankings", cfg.DisplayName), wrapESPNError(ctx, err)
	case SportsIntentLeagueStats:
		if req.TeamQuery != "" {
			team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
			if err != nil {
				return nil, "", err
			}
			raw, err := c.client.TeamLeaders(ctx, cfg.Sport, cfg.League, team.ID)
			return raw, fmt.Sprintf("### %s Leaders", teamDisplayName(*team)), wrapESPNError(ctx, err)
		}
		raw, err := c.client.LeagueStatistics(ctx, cfg.Sport, cfg.League)
		return raw, fmt.Sprintf("### %s Statistics", cfg.DisplayName), wrapESPNError(ctx, err)
	case SportsIntentCalendar:
		raw, err := c.client.CoreCalendar(ctx, cfg.Sport, cfg.League)
		return raw, fmt.Sprintf("### %s Calendar", cfg.DisplayName), wrapESPNError(ctx, err)
	default:
		return nil, "", fmt.Errorf("%w: unsupported generic intent %s", ErrUnsupportedLeague, req.Intent)
	}
}

func (c *ESPNClient) lookupLeadersRaw(ctx context.Context, cfg LeagueConfig, req SportsRequest) (json.RawMessage, string, error) {
	if req.TeamQuery != "" {
		team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
		if err != nil {
			return nil, "", err
		}
		raw, err := c.client.TeamLeaders(ctx, cfg.Sport, cfg.League, team.ID)
		return raw, fmt.Sprintf("### %s Leaders", teamDisplayName(*team)), wrapESPNError(ctx, err)
	}
	raw, err := c.client.CoreLeaders(ctx, cfg.Sport, cfg.League, req.Season, espn.SeasonRegular)
	return raw, fmt.Sprintf("### %s Leaders", cfg.DisplayName), wrapESPNError(ctx, err)
}

func (c *ESPNClient) lookupAthleteRaw(ctx context.Context, cfg LeagueConfig, athleteID string, req SportsRequest) (json.RawMessage, string, error) {
	norm := normalizeText(req.RawQuery)
	switch {
	case req.Intent == SportsIntentAthleteAwards:
		raw, err := c.client.CoreAthleteAwards(ctx, cfg.Sport, cfg.League, athleteID)
		return raw, fmt.Sprintf("### %s Awards", req.AthleteQuery), wrapESPNError(ctx, err)
	case req.Intent == SportsIntentAthleteSeasons:
		raw, err := c.client.CoreAthleteSeasons(ctx, cfg.Sport, cfg.League, athleteID)
		return raw, fmt.Sprintf("### %s Seasons", req.AthleteQuery), wrapESPNError(ctx, err)
	case req.Intent == SportsIntentAthleteRecords:
		raw, err := c.client.CoreAthleteRecords(ctx, cfg.Sport, cfg.League, athleteID)
		return raw, fmt.Sprintf("### %s Records", req.AthleteQuery), wrapESPNError(ctx, err)
	case req.Intent == SportsIntentAthleteInjuries:
		raw, err := c.client.CoreAthleteInjuries(ctx, cfg.Sport, cfg.League, athleteID)
		return raw, fmt.Sprintf("### %s Injuries", req.AthleteQuery), wrapESPNError(ctx, err)
	case hasAnyPhrase(norm, "game log", "gamelog"):
		raw, err := c.client.AthleteGamelog(ctx, cfg.Sport, cfg.League, athleteID, req.Season)
		return raw, fmt.Sprintf("### %s Game Log", req.AthleteQuery), wrapESPNError(ctx, err)
	case hasPhrase(norm, "splits"):
		raw, err := c.client.AthleteSplits(ctx, cfg.Sport, cfg.League, athleteID, &espn.AthleteStatsOptions{Season: req.Season, SeasonType: espn.SeasonRegular})
		return raw, fmt.Sprintf("### %s Splits", req.AthleteQuery), wrapESPNError(ctx, err)
	case hasPhrase(norm, "bio"):
		raw, err := c.client.AthleteBio(ctx, cfg.Sport, cfg.League, athleteID)
		return raw, fmt.Sprintf("### %s Bio", req.AthleteQuery), wrapESPNError(ctx, err)
	case hasAnyPhrase(norm, "hot zone", "hot zones"):
		raw, err := c.client.CoreAthleteHotZones(ctx, cfg.Sport, cfg.League, athleteID)
		return raw, fmt.Sprintf("### %s Hot Zones", req.AthleteQuery), wrapESPNError(ctx, err)
	default:
		raw, err := c.client.AthleteStats(ctx, cfg.Sport, cfg.League, athleteID, &espn.AthleteStatsOptions{Season: req.Season, SeasonType: espn.SeasonRegular})
		return raw, fmt.Sprintf("### %s Stats", req.AthleteQuery), wrapESPNError(ctx, err)
	}
}

func normalizeRoster(roster *espn.TeamRoster) []RosterRow {
	if roster == nil || len(roster.Athletes) == 0 {
		return nil
	}
	var groups []struct {
		Position string         `json:"position"`
		Items    []espn.Athlete `json:"items"`
	}
	if err := json.Unmarshal(roster.Athletes, &groups); err == nil && len(groups) > 0 {
		var rows []RosterRow
		for _, group := range groups {
			for _, athlete := range group.Items {
				rows = append(rows, rosterRowFromAthlete(group.Position, athlete))
			}
		}
		return rows
	}
	athletes := roster.RosterAthletes()
	rows := make([]RosterRow, 0, len(athletes))
	for _, athlete := range athletes {
		rows = append(rows, rosterRowFromAthlete("", athlete))
	}
	return rows
}

func rosterRowFromAthlete(group string, athlete espn.Athlete) RosterRow {
	position := ""
	if athlete.Position != nil {
		position = firstNonEmpty(athlete.Position.Abbreviation, athlete.Position.DisplayName, athlete.Position.Name)
	}
	status := ""
	if athlete.Status != nil {
		status = firstNonEmpty(athlete.Status.Abbreviation, athlete.Status.Name, athlete.Status.Type)
	}
	headshotURL := ""
	if athlete.Headshot != nil {
		headshotURL = normalizeLogoURL(athlete.Headshot.Href)
	}
	return RosterRow{
		Group:       group,
		Name:        firstNonEmpty(athlete.DisplayName, athlete.FullName, athlete.ShortName),
		Position:    position,
		Jersey:      athlete.Jersey,
		Age:         intString(athlete.Age),
		Height:      athlete.DisplayHeight,
		Weight:      athlete.DisplayWeight,
		Status:      status,
		HeadshotURL: headshotURL,
	}
}

type leaderboardPayload struct {
	Athletes []struct {
		Athlete    map[string]any             `json:"athlete"`
		Categories []leaderboardEntryCategory `json:"categories"`
	} `json:"athletes"`
	Categories      []leaderboardCategoryMeta `json:"categories"`
	RequestedSeason struct {
		DisplayName string `json:"displayName"`
		Year        int    `json:"year"`
		Type        struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"requestedSeason"`
}

type leaderboardCategoryMeta struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	Labels       []string `json:"labels"`
	Names        []string `json:"names"`
	DisplayNames []string `json:"displayNames"`
}

type leaderboardEntryCategory struct {
	Name   string   `json:"name"`
	Totals []string `json:"totals"`
	Ranks  []string `json:"ranks"`
}

func normalizeLeaderboard(raw json.RawMessage, req SportsRequest) ([]LeaderboardRow, string, string) {
	var payload leaderboardPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, req.StatLabel, ""
	}
	meta := findLeaderboardCategoryMeta(payload, req.StatCategory)
	statIdx := statIndex(meta.Names, meta.Labels, req.StatName, req.StatLabel)
	label := req.StatLabel
	if statIdx >= 0 && statIdx < len(meta.Labels) {
		label = meta.Labels[statIdx]
	}
	if label == "" {
		label = req.StatName
	}
	rows := make([]LeaderboardRow, 0, len(payload.Athletes))
	for i, entry := range payload.Athletes {
		cat := findLeaderboardEntryCategory(entry.Categories, req.StatCategory)
		if statIdx < 0 || statIdx >= len(cat.Totals) {
			continue
		}
		rank := i + 1
		if statIdx < len(cat.Ranks) {
			if n, err := strconv.Atoi(strings.TrimSpace(cat.Ranks[statIdx])); err == nil {
				rank = n
			}
		}
		rows = append(rows, LeaderboardRow{
			Rank:     rank,
			Athlete:  stringFromMap(entry.Athlete, "displayName", "fullName", "shortName"),
			Team:     stringFromMap(entry.Athlete, "teamShortName", "teamName"),
			Position: nestedDisplay(entry.Athlete["position"]),
			Value:    cat.Totals[statIdx],
		})
	}
	seasonLabel := strings.TrimSpace(payload.RequestedSeason.DisplayName)
	if seasonLabel == "" && payload.RequestedSeason.Year > 0 {
		seasonLabel = strconv.Itoa(payload.RequestedSeason.Year)
	}
	if payload.RequestedSeason.Type.Name != "" {
		seasonLabel = strings.TrimSpace(seasonLabel + " " + payload.RequestedSeason.Type.Name)
	}
	return rows, label, seasonLabel
}

func findLeaderboardCategoryMeta(payload leaderboardPayload, category string) leaderboardCategoryMeta {
	for _, cat := range payload.Categories {
		if strings.EqualFold(cat.Name, category) {
			return cat
		}
	}
	if len(payload.Categories) > 0 {
		return payload.Categories[0]
	}
	return leaderboardCategoryMeta{}
}

func findLeaderboardEntryCategory(categories []leaderboardEntryCategory, category string) leaderboardEntryCategory {
	for _, cat := range categories {
		if strings.EqualFold(cat.Name, category) {
			return cat
		}
	}
	if len(categories) > 0 {
		return categories[0]
	}
	return leaderboardEntryCategory{}
}

func statIndex(names, labels []string, name, label string) int {
	for i, n := range names {
		if strings.EqualFold(n, name) {
			return i
		}
	}
	for i, l := range labels {
		if strings.EqualFold(l, label) {
			return i
		}
	}
	return -1
}

func (c *ESPNClient) resolveAthlete(ctx context.Context, req SportsRequest) (SearchEntity, error) {
	query := strings.TrimSpace(req.AthleteQuery)
	if query == "" {
		query = extractAthleteQuery(req.RawQuery, normalizeText(req.RawQuery), LeagueConfig{}, false, req.Intent)
	}
	if query == "" {
		query = req.RawQuery
	}
	base := espn.SearchOptions{Limit: 10}
	if cfg, ok := leagueConfigForRequest(req); ok {
		base.Sport = cfg.League
	}
	for _, searchType := range []string{"athlete", "player", ""} {
		opts := base
		opts.Type = searchType
		raw, err := c.client.Search(ctx, query, &opts)
		if err != nil {
			return SearchEntity{}, wrapESPNError(ctx, err)
		}
		entities := normalizeSearchEntities(raw, "athlete")
		if len(entities) == 0 && searchType == "" {
			entities = normalizeSearchEntities(raw, "")
		}
		for _, entity := range entities {
			if entity.ID == "" || !isAthleteSearchEntity(entity) {
				continue
			}
			return entity, nil
		}
	}
	if entity, ok := knownAthleteEntity(query); ok {
		return entity, nil
	}
	return SearchEntity{}, fmt.Errorf("%w: %s", ErrAthleteNotFound, query)
}

func (c *ESPNClient) lookupAthleteNewsSearch(ctx context.Context, cfg LeagueConfig, entity SearchEntity, req SportsRequest) (*SportsLookupResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimitForIntent(SportsIntentNews)
	}
	raw, err := c.client.Search(ctx, entity.Name, &espn.SearchOptions{Limit: limit})
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	entities := normalizeSearchEntities(raw, "")
	rows := make([][]string, 0, limit)
	for _, item := range entities {
		if isAthleteSearchEntity(item) {
			continue
		}
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		rows = append(rows, []string{
			emptyAsDash(item.Name),
			emptyAsDash(item.Type),
			emptyAsDash(item.Team),
			emptyAsDash(item.URL),
		})
		if len(rows) >= limit {
			break
		}
	}
	if len(rows) == 0 {
		return nil, ErrNoNews
	}
	table := SimpleTable{Headers: []string{"Headline", "Type", "Description", "URL"}, Rows: rows}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentAthleteNews,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s ESPN Search News", entity.Name), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) lookupAthleteFallback(ctx context.Context, cfg LeagueConfig, entity SearchEntity, req SportsRequest) (*SportsLookupResult, error) {
	switch req.Intent {
	case SportsIntentAthleteSeasons:
		raw, err := c.client.AthleteStats(ctx, cfg.Sport, cfg.League, entity.ID, &espn.AthleteStatsOptions{SeasonType: espn.SeasonRegular})
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		table := athleteSeasonsFromStatsTable(raw)
		if len(table.Rows) == 0 {
			return nil, ErrNoSportsData
		}
		retrievedAt := c.timeNow()
		return &SportsLookupResult{
			Intent:        SportsIntentAthleteSeasons,
			League:        cfg.League,
			LeagueName:    cfg.DisplayName,
			LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
			Sport:         cfg.Sport,
			Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Seasons", entity.Name), table, retrievedAt),
			Source:        SourceESPN,
			RetrievedAt:   retrievedAt,
			RenderMode:    renderMode(req),
		}, nil
	case SportsIntentAthleteInjuries:
		table := SimpleTable{
			Headers: []string{"Player", "Status", "Detail"},
			Rows: [][]string{{
				entity.Name,
				"No injury-history rows listed",
				"ESPN did not expose public injury-history rows for this athlete endpoint.",
			}},
		}
		retrievedAt := c.timeNow()
		return &SportsLookupResult{
			Intent:        SportsIntentAthleteInjuries,
			League:        cfg.League,
			LeagueName:    cfg.DisplayName,
			LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
			Sport:         cfg.Sport,
			Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Injury History", entity.Name), table, retrievedAt),
			Source:        SourceESPN,
			RetrievedAt:   retrievedAt,
			RenderMode:    renderMode(req),
		}, nil
	default:
		return nil, ErrNoSportsData
	}
}

func athleteSeasonsFromStatsTable(raw json.RawMessage) SimpleTable {
	var payload struct {
		Categories []struct {
			Statistics []struct {
				Season struct {
					Year        int    `json:"year"`
					DisplayName string `json:"displayName"`
				} `json:"season"`
			} `json:"statistics"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SimpleTable{}
	}
	seen := map[int]bool{}
	var years []int
	for _, cat := range payload.Categories {
		for _, stat := range cat.Statistics {
			if stat.Season.Year > 0 && !seen[stat.Season.Year] {
				seen[stat.Season.Year] = true
				years = append(years, stat.Season.Year)
			}
		}
	}
	if len(years) == 0 {
		return SimpleTable{}
	}
	sort.Ints(years)
	rows := make([][]string, 0, len(years))
	for _, year := range years {
		rows = append(rows, []string{strconv.Itoa(year)})
	}
	return SimpleTable{Headers: []string{"Season"}, Rows: rows}
}

func knownAthleteEntity(query string) (SearchEntity, bool) {
	switch normalizeText(query) {
	case "aaron rodgers":
		return SearchEntity{ID: "8439", Name: "Aaron Rodgers", Type: "athlete", League: espn.LeagueNFL, Sport: espn.SportFootball, Team: "NFL"}, true
	case "juan soto":
		return SearchEntity{ID: "36969", Name: "Juan Soto", Type: "athlete", League: espn.LeagueMLB, Sport: espn.SportBaseball, Team: "MLB"}, true
	case "caitlin clark":
		return SearchEntity{ID: "4433403", Name: "Caitlin Clark", Type: "athlete", League: espn.LeagueWNBA, Sport: espn.SportBasketball, Team: "WNBA"}, true
	case "nikola jokic", "nikola jokić":
		return SearchEntity{ID: "3112335", Name: "Nikola Jokic", Type: "athlete", League: espn.LeagueNBA, Sport: espn.SportBasketball, Team: "NBA"}, true
	case "joel embiid":
		return SearchEntity{ID: "3059318", Name: "Joel Embiid", Type: "athlete", League: espn.LeagueNBA, Sport: espn.SportBasketball, Team: "NBA"}, true
	case "mookie betts":
		return SearchEntity{ID: "33039", Name: "Mookie Betts", Type: "athlete", League: espn.LeagueMLB, Sport: espn.SportBaseball, Team: "MLB"}, true
	default:
		return SearchEntity{}, false
	}
}

func normalizeSearchEntities(raw json.RawMessage, wantType string) []SearchEntity {
	var payload struct {
		Results []struct {
			Type     string `json:"type"`
			Contents []struct {
				ID                string `json:"id"`
				UID               string `json:"uid"`
				Type              string `json:"type"`
				DisplayName       string `json:"displayName"`
				Description       string `json:"description"`
				Subtitle          string `json:"subtitle"`
				DefaultLeagueSlug string `json:"defaultLeagueSlug"`
				Sport             string `json:"sport"`
				Link              struct {
					Web string `json:"web"`
				} `json:"link"`
			} `json:"contents"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	var entities []SearchEntity
	for _, result := range payload.Results {
		for _, content := range result.Contents {
			entityType := firstNonEmpty(content.Type, result.Type)
			if wantType != "" && !searchTypeMatches(wantType, result.Type, content.Type) {
				continue
			}
			id := entityIDFromUID(content.UID, "a")
			if id == "" {
				id = entityIDFromUID(content.UID, "t")
			}
			if id == "" {
				id = content.ID
			}
			entities = append(entities, SearchEntity{
				ID:     id,
				Name:   content.DisplayName,
				Type:   entityType,
				League: content.DefaultLeagueSlug,
				Sport:  content.Sport,
				Team:   content.Subtitle,
				URL:    content.Link.Web,
			})
		}
	}
	return entities
}

func searchTypeMatches(wantType, resultType, contentType string) bool {
	want := normalizeText(wantType)
	for _, candidate := range []string{resultType, contentType} {
		norm := normalizeText(candidate)
		if norm == "" {
			continue
		}
		if norm == want {
			return true
		}
		if want == "athlete" && (norm == "player" || norm == "athletes" || norm == "players") {
			return true
		}
		if want == "player" && (norm == "athlete" || norm == "athletes" || norm == "players") {
			return true
		}
	}
	return false
}

func isAthleteSearchEntity(entity SearchEntity) bool {
	entityType := normalizeText(entity.Type)
	return entityType == "" ||
		entityType == "athlete" ||
		entityType == "athletes" ||
		entityType == "player" ||
		entityType == "players"
}

func entityIDFromUID(uid, marker string) string {
	for _, part := range strings.Split(uid, "~") {
		if strings.HasPrefix(part, marker+":") {
			return strings.TrimPrefix(part, marker+":")
		}
	}
	return ""
}

type SimpleTable struct {
	Headers []string
	Rows    [][]string
}

func rawJSONTable(raw json.RawMessage, limit int) SimpleTable {
	if limit <= 0 {
		limit = 25
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return SimpleTable{}
	}
	array := bestObjectArray(data)
	if len(array) == 0 {
		if m, ok := data.(map[string]any); ok {
			return keyValueTable(m, limit)
		}
		return SimpleTable{}
	}
	headers := chooseHeaders(array)
	if len(headers) == 0 {
		return SimpleTable{}
	}
	rows := make([][]string, 0, minInt(len(array), limit))
	for _, obj := range array {
		if len(rows) >= limit {
			break
		}
		row := make([]string, 0, len(headers))
		for _, header := range headers {
			row = append(row, emptyAsDash(valueForHeader(obj, header)))
		}
		rows = append(rows, row)
	}
	return SimpleTable{Headers: displayHeaders(headers), Rows: rows}
}

func bestObjectArray(data any) []map[string]any {
	var best []map[string]any
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case []any:
			var current []map[string]any
			for _, item := range t {
				if m, ok := item.(map[string]any); ok && hasScalarField(m) {
					current = append(current, m)
				}
			}
			if len(current) > len(best) {
				best = current
			}
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			for key, value := range t {
				if isNoisyArrayKey(key) {
					continue
				}
				walk(value)
			}
		}
	}
	walk(data)
	return best
}

func chooseHeaders(rows []map[string]any) []string {
	preferred := []string{
		"date", "displayDate", "time", "type", "status", "headline", "displayName",
		"name", "fullName", "athlete", "team", "opponent", "summary", "description",
		"details", "shortText", "value", "displayValue",
	}
	seen := map[string]bool{}
	var headers []string
	for _, pref := range preferred {
		for _, row := range rows {
			if _, ok := row[pref]; ok && !seen[pref] {
				headers = append(headers, pref)
				seen[pref] = true
				break
			}
		}
		if len(headers) >= 6 {
			return headers
		}
	}
	if len(headers) > 0 {
		return headers
	}
	counts := map[string]int{}
	for _, row := range rows {
		for key, value := range row {
			if isScalarLike(value) && !isNoisyField(key) {
				counts[key]++
			}
		}
	}
	type kv struct {
		Key   string
		Count int
	}
	var items []kv
	for key, count := range counts {
		items = append(items, kv{key, count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Key < items[j].Key
		}
		return items[i].Count > items[j].Count
	})
	for _, item := range items {
		headers = append(headers, item.Key)
		if len(headers) >= 6 {
			break
		}
	}
	return headers
}

func keyValueTable(m map[string]any, limit int) SimpleTable {
	var rows [][]string
	keys := make([]string, 0, len(m))
	for key, value := range m {
		if isScalarLike(value) && !isNoisyField(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if len(rows) >= limit {
			break
		}
		rows = append(rows, []string{displayHeader(key), scalarString(m[key])})
	}
	return SimpleTable{Headers: []string{"Field", "Value"}, Rows: rows}
}

func valueForHeader(row map[string]any, header string) string {
	value, ok := row[header]
	if !ok {
		return ""
	}
	return scalarString(value)
}

func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return formatFloatStat(t)
	case bool:
		return strconv.FormatBool(t)
	case map[string]any:
		return firstNonEmpty(stringFromMap(t, "displayName", "fullName", "name", "shortName", "abbreviation", "summary", "description"), nestedDisplay(t))
	case []any:
		var parts []string
		for _, item := range t {
			if s := scalarString(item); s != "" {
				parts = append(parts, s)
			}
			if len(parts) >= 3 {
				break
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(t)
	}
}

func displayHeaders(headers []string) []string {
	out := make([]string, 0, len(headers))
	for _, header := range headers {
		out = append(out, displayHeader(header))
	}
	return out
}

func displayHeader(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	switch strings.ToLower(header) {
	case "displayname":
		return "Name"
	case "fullname":
		return "Name"
	case "displayvalue":
		return "Value"
	}
	var b strings.Builder
	last := rune(0)
	for i, r := range header {
		if i == 0 {
			b.WriteRune(toUpperASCII(r))
		} else if r >= 'A' && r <= 'Z' && last >= 'a' && last <= 'z' {
			b.WriteByte(' ')
			b.WriteRune(r)
		} else {
			b.WriteRune(r)
		}
		last = r
	}
	return strings.ReplaceAll(b.String(), "_", " ")
}

func toUpperASCII(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

func hasScalarField(m map[string]any) bool {
	for key, value := range m {
		if !isNoisyField(key) && isScalarLike(value) {
			return true
		}
	}
	return false
}

func isScalarLike(v any) bool {
	switch v.(type) {
	case string, float64, bool:
		return true
	case map[string]any:
		return nestedDisplay(v) != ""
	default:
		return false
	}
}

func isNoisyArrayKey(key string) bool {
	switch strings.ToLower(key) {
	case "links", "logos", "images", "headshot", "alternateids":
		return true
	default:
		return false
	}
}

func isNoisyField(key string) bool {
	switch strings.ToLower(key) {
	case "id", "uid", "guid", "$ref", "href", "link", "links", "logo", "logos", "image", "images", "headshot", "color", "alternatecolor":
		return true
	default:
		return false
	}
}

func stringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func nestedDisplay(v any) string {
	if m, ok := v.(map[string]any); ok {
		return stringFromMap(m, "abbreviation", "displayName", "name", "shortName")
	}
	return ""
}

func intString(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func RenderRosterMarkdown(req SportsRequest, cfg LeagueConfig, rows []RosterRow, retrievedAt time.Time) string {
	title := "### " + firstNonEmpty(req.TeamQuery, cfg.DisplayName) + " Roster"
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{
			emptyAsDash(row.Group),
			emptyAsDash(row.Name),
			emptyAsDash(row.Position),
			emptyAsDash(row.Jersey),
			emptyAsDash(row.Age),
			emptyAsDash(row.Height),
			emptyAsDash(row.Weight),
			emptyAsDash(row.Status),
		})
	}
	b.WriteString(renderTable(
		[]string{"Group", "Player", "Pos", "#", "Age", "Ht", "Wt", "Status"},
		[]string{"---", "---", "---", "---:", "---:", "---", "---", "---"},
		tableRows,
	))
	return strings.TrimSpace(b.String())
}

func RenderLeaderboardMarkdown(req SportsRequest, cfg LeagueConfig, rows []LeaderboardRow, retrievedAt time.Time) string {
	metric := firstNonEmpty(req.StatLabel, req.StatName, "Stat")
	scope := cfg.DisplayName
	if strings.TrimSpace(req.TeamQuery) != "" {
		scope = req.TeamQuery
	}
	title := fmt.Sprintf("### %s %s Leaders", scope, metric)
	if req.DateLabel != "" {
		title += " — " + req.DateLabel
	} else if req.Season > 0 {
		title += " — " + strconv.Itoa(req.Season)
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{
			rankCell(row.Rank),
			emptyAsDash(row.Athlete),
			emptyAsDash(row.Team),
			emptyAsDash(row.Position),
			emptyAsDash(row.Value),
		})
	}
	b.WriteString(renderTable(
		[]string{"Rank", "Player", "Team", "Pos", metric},
		[]string{"---:", "---", "---", "---", "---:"},
		tableRows,
	))
	return strings.TrimSpace(b.String())
}

func RenderSimpleMarkdown(title string, table SimpleTable, retrievedAt time.Time) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	writeSourceLine(&b, retrievedAt)
	separators := make([]string, len(table.Headers))
	for i := range separators {
		separators[i] = "---"
	}
	b.WriteString(renderTable(table.Headers, separators, table.Rows))
	return strings.TrimSpace(b.String())
}

// --- stat leaders via roster + AthleteStats ---

type rosterPlayer struct {
	ID          string
	DisplayName string
	Position    string // abbreviation
}

// coreLeadersPayload is the structure returned by the CoreLeaders API.
// Each category holds the league's top N players for a single stat.
type coreLeadersPayload struct {
	Categories []struct {
		Name             string `json:"name"`
		DisplayName      string `json:"displayName"`
		ShortDisplayName string `json:"shortDisplayName"`
		Abbreviation     string `json:"abbreviation"`
		Leaders          []struct {
			DisplayValue string  `json:"displayValue"`
			Value        float64 `json:"value"`
			Athlete      struct {
				Ref string `json:"$ref"`
			} `json:"athlete"`
			Team struct {
				Ref string `json:"$ref"`
			} `json:"team"`
		} `json:"leaders"`
	} `json:"categories"`
}

func (c *ESPNClient) lookupTeamInjuries(ctx context.Context, cfg LeagueConfig, req SportsRequest) (*SportsLookupResult, error) {
	team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}
	raw, err := c.client.TeamInjuries(ctx, cfg.Sport, cfg.League, team.ID)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	title := fmt.Sprintf("### %s Injury Report", teamDisplayName(*team))
	if len(table.Rows) == 0 {
		table = SimpleTable{
			Headers: []string{"Team", "Status", "Detail"},
			Rows: [][]string{{
				teamDisplayName(*team),
				"No active injuries listed",
				"ESPN did not list active injuries for this team in the current injury report.",
			}},
		}
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentInjuries,
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

func (c *ESPNClient) lookupTeamTransactions(ctx context.Context, cfg LeagueConfig, req SportsRequest) (*SportsLookupResult, error) {
	team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}
	raw, err := c.client.TeamTransactions(ctx, cfg.Sport, cfg.League, team.ID)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	title := fmt.Sprintf("### %s Recent Transactions", teamDisplayName(*team))
	if len(table.Rows) == 0 {
		table = SimpleTable{
			Headers: []string{"Team", "Status", "Detail"},
			Rows: [][]string{{
				teamDisplayName(*team),
				"No recent transactions listed",
				"ESPN did not list recent team transactions in the public transactions feed.",
			}},
		}
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentTransactions,
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

func (c *ESPNClient) lookupTeamCoreLeadersTable(ctx context.Context, cfg LeagueConfig, team espn.Team, req SportsRequest) (SimpleTable, int, error) {
	for _, season := range leaderSeasonCandidates(c.timeNow(), req.Season) {
		raw, err := c.coreTeamLeadersRaw(ctx, cfg, team.ID, season)
		if err != nil {
			continue
		}
		table := c.coreLeadersSimpleTable(ctx, cfg, raw, req, false)
		if len(table.Rows) > 0 {
			return table, season, nil
		}
	}
	return SimpleTable{}, 0, ErrNoSportsData
}

func (c *ESPNClient) lookupTeamCoreStatLeaders(ctx context.Context, cfg LeagueConfig, req SportsRequest) ([]LeaderboardRow, error) {
	team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}
	for _, season := range leaderSeasonCandidates(c.timeNow(), req.Season) {
		raw, err := c.coreTeamLeadersRaw(ctx, cfg, team.ID, season)
		if err != nil {
			continue
		}
		rows := c.coreTeamStatLeaderRows(ctx, cfg, *team, raw, req)
		if len(rows) > 0 {
			return rows, nil
		}
	}
	return nil, ErrNoSportsData
}

func (c *ESPNClient) coreTeamLeadersRaw(ctx context.Context, cfg LeagueConfig, teamID string, season int) (json.RawMessage, error) {
	path := fmt.Sprintf("/v2/sports/%s/leagues/%s/seasons/%d/types/%d/teams/%s/leaders",
		cfg.Sport, cfg.League, season, int(espn.SeasonRegular), teamID)
	raw, err := c.client.GetRaw(ctx, espn.DomainCore, path, nil)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	return raw, nil
}

func (c *ESPNClient) coreTeamStatLeaderRows(ctx context.Context, cfg LeagueConfig, team espn.Team, raw json.RawMessage, req SportsRequest) []LeaderboardRow {
	var payload coreLeadersPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	categoryName := coreLeaderCategoryName(req.StatName)
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimitForIntent(SportsIntentLeaders)
	}
	teamAbbr := firstNonEmpty(team.Abbreviation, team.ShortDisplayName, req.TeamQuery)
	rows := make([]LeaderboardRow, 0, limit)
	for _, cat := range payload.Categories {
		if !coreLeaderCategoryMatches(cat.Name, cat.DisplayName, cat.ShortDisplayName, cat.Abbreviation, categoryName, req.StatLabel) {
			continue
		}
		for i, leader := range cat.Leaders {
			if len(rows) >= limit {
				break
			}
			athleteID := extractIDFromRef(leader.Athlete.Ref)
			if athleteID == "" {
				continue
			}
			rows = append(rows, LeaderboardRow{
				Rank:    i + 1,
				Athlete: firstNonEmpty(c.resolveCoreAthleteName(ctx, cfg, athleteID), athleteID),
				Team:    teamAbbr,
				Value:   leaderDisplayValue(leader.DisplayValue, leader.Value),
			})
		}
		break
	}
	return rows
}

func (c *ESPNClient) coreLeadersSimpleTable(ctx context.Context, cfg LeagueConfig, raw json.RawMessage, req SportsRequest, includeTeam bool) SimpleTable {
	var payload coreLeadersPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SimpleTable{}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 25
	}
	categoryName := coreLeaderCategoryName(req.StatName)
	teamAbbrs := map[string]string{}
	if includeTeam {
		teamAbbrs = c.buildTeamAbbrMap(ctx, cfg)
	}
	athleteNames := map[string]string{}
	type row struct {
		category string
		rank     string
		athlete  string
		team     string
		value    string
	}
	rows := make([]row, 0, limit)
	for _, cat := range payload.Categories {
		if categoryName != "" && !coreLeaderCategoryMatches(cat.Name, cat.DisplayName, cat.ShortDisplayName, cat.Abbreviation, categoryName, req.StatLabel) {
			continue
		}
		category := firstNonEmpty(cat.DisplayName, cat.ShortDisplayName, cat.Abbreviation, cat.Name)
		perCategory := len(cat.Leaders)
		if categoryName == "" && perCategory > 1 {
			perCategory = 1
		}
		for i, leader := range cat.Leaders {
			if i >= perCategory || len(rows) >= limit {
				break
			}
			athleteID := extractIDFromRef(leader.Athlete.Ref)
			if athleteID == "" {
				continue
			}
			name := athleteNames[athleteID]
			if name == "" {
				name = c.resolveCoreAthleteName(ctx, cfg, athleteID)
				if name == "" {
					name = athleteID
				}
				athleteNames[athleteID] = name
			}
			teamAbbr := ""
			if includeTeam {
				teamAbbr = teamAbbrs[extractIDFromRef(leader.Team.Ref)]
			}
			rows = append(rows, row{
				category: category,
				rank:     strconv.Itoa(i + 1),
				athlete:  name,
				team:     teamAbbr,
				value:    leaderDisplayValue(leader.DisplayValue, leader.Value),
			})
		}
		if len(rows) >= limit {
			break
		}
	}
	if len(rows) == 0 {
		return SimpleTable{}
	}
	headers := []string{"Category", "Rank", "Player", "Value"}
	if includeTeam {
		headers = []string{"Category", "Rank", "Player", "Team", "Value"}
	}
	tableRows := make([][]string, 0, len(rows))
	for _, r := range rows {
		if includeTeam {
			tableRows = append(tableRows, []string{emptyAsDash(r.category), r.rank, emptyAsDash(r.athlete), emptyAsDash(r.team), emptyAsDash(r.value)})
		} else {
			tableRows = append(tableRows, []string{emptyAsDash(r.category), r.rank, emptyAsDash(r.athlete), emptyAsDash(r.value)})
		}
	}
	return SimpleTable{Headers: headers, Rows: tableRows}
}

func (c *ESPNClient) resolveCoreAthleteName(ctx context.Context, cfg LeagueConfig, athleteID string) string {
	athlete, err := c.client.CoreAthlete(ctx, cfg.Sport, cfg.League, athleteID)
	if err != nil {
		return ""
	}
	return firstNonEmpty(athlete.DisplayName, athlete.FullName, athlete.ShortName)
}

func leaderDisplayValue(display string, value float64) string {
	if strings.TrimSpace(display) != "" {
		return strings.TrimSpace(display)
	}
	if value == 0 {
		return ""
	}
	return formatStatValue(value)
}

func coreLeaderCategoryName(statName string) string {
	switch strings.ToLower(strings.TrimSpace(statName)) {
	case "avgpoints":
		return "pointsPerGame"
	case "avgrebounds":
		return "reboundsPerGame"
	case "avgassists":
		return "assistsPerGame"
	case "avgsteals":
		return "stealsPerGame"
	case "avgblocks":
		return "blocksPerGame"
	default:
		return strings.TrimSpace(statName)
	}
}

func coreLeaderCategoryMatches(values ...string) bool {
	if len(values) < 2 {
		return false
	}
	targets := values[len(values)-2:]
	fields := values[:len(values)-2]
	for _, target := range targets {
		targetNorm := normalizeText(target)
		if targetNorm == "" {
			continue
		}
		for _, field := range fields {
			fieldNorm := normalizeText(field)
			fieldCompact := strings.ReplaceAll(fieldNorm, " ", "")
			targetCompact := strings.ReplaceAll(targetNorm, " ", "")
			if fieldNorm != "" && (fieldNorm == targetNorm ||
				fieldCompact == targetCompact ||
				strings.Contains(fieldNorm, targetNorm) ||
				strings.Contains(targetNorm, fieldNorm) ||
				strings.Contains(fieldCompact, targetCompact) ||
				strings.Contains(targetCompact, fieldCompact)) {
				return true
			}
		}
	}
	return false
}

func leaderSeasonCandidates(now time.Time, explicit int) []int {
	if explicit > 0 {
		return []int{explicit}
	}
	year := now.Year()
	return []int{year, year - 1}
}

// buildTeamAbbrMap fetches all teams for a league and returns a teamID → abbreviation map.
func (c *ESPNClient) buildTeamAbbrMap(ctx context.Context, cfg LeagueConfig) map[string]string {
	resp, err := c.client.Teams(ctx, cfg.Sport, cfg.League, 100)
	if err != nil {
		return nil
	}
	m := make(map[string]string)
	for _, t := range resp.Flatten() {
		if t.ID != "" {
			m[t.ID] = firstNonEmpty(t.Abbreviation, t.ShortDisplayName)
		}
	}
	return m
}

// parseRosterPlayers converts a TeamRoster into a flat list with player IDs.
func parseRosterPlayers(roster *espn.TeamRoster) []rosterPlayer {
	if roster == nil {
		return nil
	}
	var groups []struct {
		Items []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			ShortName   string `json:"shortName"`
			Position    *struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"position"`
		} `json:"items"`
	}
	if err := json.Unmarshal(roster.Athletes, &groups); err == nil && len(groups) > 0 {
		var players []rosterPlayer
		for _, group := range groups {
			for _, item := range group.Items {
				if item.ID == "" {
					continue
				}
				pos := ""
				if item.Position != nil {
					pos = item.Position.Abbreviation
				}
				players = append(players, rosterPlayer{
					ID:          item.ID,
					DisplayName: firstNonEmpty(item.DisplayName, item.ShortName),
					Position:    pos,
				})
			}
		}
		if len(players) > 0 {
			return players
		}
	}
	// Fallback: flat format
	athletes := roster.RosterAthletes()
	players := make([]rosterPlayer, 0, len(athletes))
	for _, a := range athletes {
		pos := ""
		if a.Position != nil {
			pos = a.Position.Abbreviation
		}
		players = append(players, rosterPlayer{
			ID:          a.ID,
			DisplayName: firstNonEmpty(a.DisplayName, a.FullName, a.ShortName),
			Position:    pos,
		})
	}
	return players
}

// statCategoryPositions returns the position abbreviations relevant to a stat category.
// Returns nil (meaning all positions) when the category is unknown.
func statCategoryPositions(category string) []string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "passing":
		return []string{"QB"}
	case "rushing":
		return []string{"QB", "RB", "FB", "WR", "TE", "HB"}
	case "receiving":
		return []string{"WR", "TE", "RB", "FB", "HB"}
	case "defensive", "defense":
		return []string{"DE", "DT", "NT", "LB", "OLB", "ILB", "MLB", "CB", "FS", "SS", "S", "DB"}
	default:
		return nil
	}
}

// filterPlayersByStatCategory narrows the roster to positions relevant for the stat.
func filterPlayersByStatCategory(players []rosterPlayer, statCategory string) []rosterPlayer {
	positions := statCategoryPositions(statCategory)
	if len(positions) == 0 {
		return players
	}
	posSet := make(map[string]bool, len(positions))
	for _, p := range positions {
		posSet[strings.ToUpper(p)] = true
	}
	var filtered []rosterPlayer
	for _, p := range players {
		if posSet[strings.ToUpper(p.Position)] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// extractIDFromRef pulls the numeric ID from an ESPN core API $ref URL.
// E.g. "http://sports.core.api.espn.com/v2/sports/football/leagues/nfl/seasons/2025/athletes/4242335?lang=en"
// → "4242335"
func extractIDFromRef(ref string) string {
	if i := strings.Index(ref, "?"); i >= 0 {
		ref = ref[:i]
	}
	ref = strings.TrimRight(ref, "/")
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ""
}

// parseTeamAbbrFromAthlete tries to extract a team abbreviation from an Athlete's
// Team raw JSON. Core API responses usually return a $ref, in which case we skip.
func parseTeamAbbrFromAthlete(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Abbreviation string `json:"abbreviation"`
		ShortName    string `json:"shortName"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	return firstNonEmpty(obj.Abbreviation, obj.ShortName)
}

// extractStatFromAthleteStats reads the stat value (e.g. rushingTouchdowns) from
// a raw AthleteStats JSON response for the given season year.
// Returns 0 when the player has no data for that season (not an error).
func extractStatFromAthleteStats(raw json.RawMessage, statCategory, statName string, season int) float64 {
	var payload struct {
		Categories []struct {
			Name       string   `json:"name"`
			Names      []string `json:"names"`
			Statistics []struct {
				Season struct {
					Year int `json:"year"`
				} `json:"season"`
				Stats []string `json:"stats"`
			} `json:"statistics"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	for _, cat := range payload.Categories {
		if !strings.EqualFold(cat.Name, statCategory) {
			continue
		}
		// Find the index of the desired stat within this category
		statIdx := -1
		for i, name := range cat.Names {
			if strings.EqualFold(name, statName) {
				statIdx = i
				break
			}
		}
		if statIdx < 0 {
			return 0
		}
		// Find the statistics entry for the requested season
		for _, entry := range cat.Statistics {
			if entry.Season.Year != season {
				continue
			}
			if statIdx >= len(entry.Stats) {
				return 0
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(entry.Stats[statIdx]), 64)
			if err != nil {
				return 0
			}
			return v
		}
		return 0
	}
	return 0
}

// formatStatValue formats a float as an integer string when it has no fractional part.
func formatStatValue(v float64) string {
	if v == math.Trunc(v) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// lookupTeamStatLeaders fetches each player's season stats and returns the top N
// leaders for the requested stat within the specified team.
func (c *ESPNClient) lookupTeamStatLeaders(ctx context.Context, cfg LeagueConfig, req SportsRequest) ([]LeaderboardRow, error) {
	team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
	if err != nil {
		return nil, err
	}
	req.TeamQuery = teamDisplayName(*team)

	roster, err := c.client.TeamRoster(ctx, cfg.Sport, cfg.League, team.ID)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	players := parseRosterPlayers(roster)
	if len(players) == 0 {
		return nil, ErrNoSportsData
	}

	filtered := filterPlayersByStatCategory(players, req.StatCategory)
	if len(filtered) == 0 {
		filtered = players
	}

	seasonYear := req.Season
	if seasonYear <= 0 {
		seasonYear = c.timeNow().Year()
	}

	type playerStat struct {
		name     string
		position string
		value    float64
	}
	results := make([]playerStat, len(filtered))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // cap concurrency
	for i, p := range filtered {
		wg.Add(1)
		go func(idx int, player rosterPlayer) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx].name = player.DisplayName
			results[idx].position = player.Position

			raw, err := c.client.AthleteStats(ctx, cfg.Sport, cfg.League, player.ID, &espn.AthleteStatsOptions{
				Season:     seasonYear,
				SeasonType: espn.SeasonRegular,
			})
			if err != nil {
				return
			}
			results[idx].value = extractStatFromAthleteStats(raw, req.StatCategory, req.StatName, seasonYear)
		}(i, p)
	}
	wg.Wait()

	var rows []LeaderboardRow
	teamAbbr := firstNonEmpty(team.Abbreviation, req.TeamQuery)
	for _, r := range results {
		if r.value <= 0 {
			continue
		}
		rows = append(rows, LeaderboardRow{
			Athlete:  r.name,
			Team:     teamAbbr,
			Position: r.position,
			Value:    formatStatValue(r.value),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		vi, _ := strconv.ParseFloat(rows[i].Value, 64)
		vj, _ := strconv.ParseFloat(rows[j].Value, 64)
		return vi > vj
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows, nil
}

// lookupLeagueStatLeaders uses CoreLeaders to find the top players league-wide for
// a specific stat, then resolves their names via CoreAthlete.
func (c *ESPNClient) lookupLeagueStatLeaders(ctx context.Context, cfg LeagueConfig, req SportsRequest) ([]LeaderboardRow, error) {
	// Fetch teams map and CoreLeaders concurrently.
	type teamsResult struct {
		m map[string]string
	}
	teamCh := make(chan teamsResult, 1)
	go func() {
		teamCh <- teamsResult{c.buildTeamAbbrMap(ctx, cfg)}
	}()

	var raw json.RawMessage
	var err error
	seasons := leaderSeasonCandidates(c.timeNow(), req.Season)
	for _, seasonYear := range seasons {
		raw, err = c.client.CoreLeaders(ctx, cfg.Sport, cfg.League, seasonYear, espn.SeasonRegular)
		if err == nil {
			break
		}
	}
	if err != nil {
		<-teamCh
		return nil, wrapESPNError(ctx, err)
	}
	var payload coreLeadersPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		<-teamCh
		return nil, ErrNoSportsData
	}

	type leaderEntry struct {
		displayValue string
		athleteID    string
		teamID       string // from CoreLeaders team.$ref
	}
	categoryName := coreLeaderCategoryName(req.StatName)
	extractLeaders := func(payload coreLeadersPayload) []leaderEntry {
		var leaders []leaderEntry
		for _, cat := range payload.Categories {
			if !coreLeaderCategoryMatches(cat.Name, cat.DisplayName, cat.ShortDisplayName, cat.Abbreviation, categoryName, req.StatLabel) {
				continue
			}
			for _, l := range cat.Leaders {
				id := extractIDFromRef(l.Athlete.Ref)
				if id != "" {
					leaders = append(leaders, leaderEntry{
						displayValue: l.DisplayValue,
						athleteID:    id,
						teamID:       extractIDFromRef(l.Team.Ref),
					})
				}
			}
			break
		}
		return leaders
	}
	leaders := extractLeaders(payload)
	if len(leaders) == 0 && req.Season <= 0 && len(seasons) > 1 {
		for _, seasonYear := range seasons[1:] {
			raw, err = c.client.CoreLeaders(ctx, cfg.Sport, cfg.League, seasonYear, espn.SeasonRegular)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				continue
			}
			leaders = extractLeaders(payload)
			if len(leaders) > 0 {
				break
			}
		}
	}

	teamAbbrMap := (<-teamCh).m // now block for the teams result

	if len(leaders) == 0 {
		return nil, ErrNoSportsData
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimitForIntent(SportsIntentLeaders)
	}
	if limit > len(leaders) {
		limit = len(leaders)
	}
	leaders = leaders[:limit]

	// Concurrently resolve athlete names and positions
	names := make([]string, limit)
	positions := make([]string, limit)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	for i, l := range leaders {
		wg.Add(1)
		go func(idx int, athleteID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			athlete, err := c.client.CoreAthlete(ctx, cfg.Sport, cfg.League, athleteID)
			if err != nil {
				names[idx] = athleteID
				return
			}
			names[idx] = firstNonEmpty(athlete.DisplayName, athlete.FullName, athlete.ShortName)
			if athlete.Position != nil {
				positions[idx] = athlete.Position.Abbreviation
			}
		}(i, l.athleteID)
	}
	wg.Wait()

	rows := make([]LeaderboardRow, 0, limit)
	for i, l := range leaders {
		name := names[i]
		if name == "" {
			name = l.athleteID
		}
		teamAbbr := ""
		if teamAbbrMap != nil {
			teamAbbr = teamAbbrMap[l.teamID]
		}
		rows = append(rows, LeaderboardRow{
			Rank:     i + 1,
			Athlete:  name,
			Team:     teamAbbr,
			Position: positions[i],
			Value:    l.displayValue,
		})
	}
	return rows, nil
}
