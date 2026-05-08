package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	raw, err := c.client.StatisticsByAthlete(ctx, cfg.Sport, cfg.League, &espn.StatisticsByAthleteOptions{
		Season:     req.Season,
		SeasonType: espn.SeasonRegular,
		Category:   req.StatCategory,
		Sort:       req.StatSort,
		Limit:      limit,
		Page:       1,
	})
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	rows, label, seasonLabel := normalizeLeaderboard(raw, req)
	if len(rows) == 0 {
		return nil, ErrNoSportsData
	}
	if req.StatLabel == "" {
		req.StatLabel = label
	}
	if req.DateLabel == "" {
		req.DateLabel = seasonLabel
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
			return nil, wrapESPNError(ctx, err)
		}
		rows := normalizeNewsFeed(feed)
		if len(rows) == 0 {
			return nil, ErrNoNews
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
		return nil, err
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentAthleteStats,
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
	case hasAnyPhrase(norm, "game log", "gamelog"):
		raw, err := c.client.AthleteGamelog(ctx, cfg.Sport, cfg.League, athleteID, req.Season)
		return raw, fmt.Sprintf("### %s Game Log", req.AthleteQuery), wrapESPNError(ctx, err)
	case hasPhrase(norm, "splits"):
		raw, err := c.client.AthleteSplits(ctx, cfg.Sport, cfg.League, athleteID, &espn.AthleteStatsOptions{Season: req.Season, SeasonType: espn.SeasonRegular})
		return raw, fmt.Sprintf("### %s Splits", req.AthleteQuery), wrapESPNError(ctx, err)
	case hasPhrase(norm, "bio"):
		raw, err := c.client.AthleteBio(ctx, cfg.Sport, cfg.League, athleteID)
		return raw, fmt.Sprintf("### %s Bio", req.AthleteQuery), wrapESPNError(ctx, err)
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
	return RosterRow{
		Group:    group,
		Name:     firstNonEmpty(athlete.DisplayName, athlete.FullName, athlete.ShortName),
		Position: position,
		Jersey:   athlete.Jersey,
		Age:      intString(athlete.Age),
		Height:   athlete.DisplayHeight,
		Weight:   athlete.DisplayWeight,
		Status:   status,
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
	opts := &espn.SearchOptions{Type: "player", Limit: 5}
	if cfg, ok := leagueConfigForRequest(req); ok {
		opts.Sport = cfg.League
	}
	raw, err := c.client.Search(ctx, query, opts)
	if err != nil {
		return SearchEntity{}, wrapESPNError(ctx, err)
	}
	entities := normalizeSearchEntities(raw, "player")
	if len(entities) == 0 {
		return SearchEntity{}, fmt.Errorf("%w: %s", ErrAthleteNotFound, query)
	}
	return entities[0], nil
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
		if wantType != "" && !strings.EqualFold(result.Type, wantType) {
			continue
		}
		for _, content := range result.Contents {
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
				League: content.DefaultLeagueSlug,
				Sport:  content.Sport,
				Team:   content.Subtitle,
				URL:    content.Link.Web,
			})
		}
	}
	return entities
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
	title := fmt.Sprintf("### %s %s Leaders", cfg.DisplayName, metric)
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
