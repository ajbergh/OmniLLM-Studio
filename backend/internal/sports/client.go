package sports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

type ESPNClient struct {
	client *espn.Client
	now    func() time.Time
}

func NewESPNClient() *ESPNClient {
	return &ESPNClient{
		client: espn.New(
			espn.WithUserAgent("OmniLLM-Studio/1.0"),
			espn.WithTimeout(10*time.Second),
			espn.WithMaxRetries(3),
			espn.WithBackoff(250*time.Millisecond),
		),
		now: time.Now,
	}
}

func (c *ESPNClient) Lookup(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	result, err := c.lookup(ctx, req)
	if err != nil && isGracefulLookupError(err) {
		return c.emptyLookupResult(req, err), nil
	}
	return result, err
}

func (c *ESPNClient) lookup(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	switch req.Intent {
	case SportsIntentStandings:
		return c.LookupStandings(ctx, req)
	case SportsIntentScores, SportsIntentSchedule:
		return c.LookupScores(ctx, req)
	case SportsIntentNews:
		return c.LookupNews(ctx, req)
	case SportsIntentOdds:
		return c.LookupOdds(ctx, req)
	case SportsIntentRoster:
		return c.LookupRoster(ctx, req)
	case SportsIntentInjuries, SportsIntentTransactions, SportsIntentTeamSchedule, SportsIntentRankings, SportsIntentLeagueStats, SportsIntentCalendar:
		return c.LookupGeneric(ctx, req)
	case SportsIntentTeamRecord:
		return c.LookupTeamRecord(ctx, req)
	case SportsIntentTeams:
		return c.LookupTeams(ctx, req)
	case SportsIntentTeamHistory:
		return c.LookupTeamHistory(ctx, req)
	case SportsIntentSeasons:
		return c.LookupSeasons(ctx, req)
	case SportsIntentLeaders:
		return c.LookupLeaders(ctx, req)
	case SportsIntentAthleteStats, SportsIntentAthleteNews, SportsIntentAthleteAwards, SportsIntentAthleteSeasons, SportsIntentAthleteRecords, SportsIntentAthleteInjuries:
		return c.LookupAthlete(ctx, req)
	// Extended capabilities (Q10, Q46, Q52, Q53, Q58, Q62, Q63, Q68–Q76)
	case SportsIntentScoreboardHeader:
		return c.LookupScoreboardHeader(ctx, req)
	case SportsIntentSearch:
		return c.LookupSearch(ctx, req)
	case SportsIntentQBR:
		return c.LookupQBR(ctx, req)
	case SportsIntentAthleteComparison:
		return c.LookupAthleteComparison(ctx, req)
	case SportsIntentHotZones:
		return c.LookupHotZones(ctx, req)
	case SportsIntentGameDetail:
		return c.LookupGameDetail(ctx, req)
	case SportsIntentChampions:
		return c.LookupChampions(ctx, req)
	case SportsIntentDraft:
		return c.LookupDraft(ctx, req)
	case SportsIntentCoaches:
		return c.LookupCoaches(ctx, req)
	// Q77–Q87
	case SportsIntentVenues:
		return c.LookupVenues(ctx, req)
	case SportsIntentPowerIndex:
		return c.LookupPowerIndex(ctx, req)
	case SportsIntentRecruits:
		return c.LookupRecruits(ctx, req)
	case SportsIntentBracketology:
		return c.LookupBracketology(ctx, req)
	case SportsIntentTournaments:
		return c.LookupTournaments(ctx, req)
	case SportsIntentFantasy:
		return c.LookupFantasy(ctx, req)
	default:
		return nil, fmt.Errorf("%w: unknown sports intent", ErrUnsupportedLeague)
	}
}

func isGracefulLookupError(err error) bool {
	return errors.Is(err, ErrNoGames) ||
		errors.Is(err, ErrNoMatchingGames) ||
		errors.Is(err, ErrNoStandings) ||
		errors.Is(err, ErrNoNews) ||
		errors.Is(err, ErrNoOdds) ||
		errors.Is(err, ErrNoSportsData) ||
		errors.Is(err, ErrTeamNotFound) ||
		errors.Is(err, ErrAthleteNotFound)
}

func (c *ESPNClient) emptyLookupResult(req SportsRequest, err error) *SportsLookupResult {
	retrievedAt := c.timeNow()
	cfg, hasCfg := leagueConfigForRequest(req)
	leagueName := "Sports"
	league := req.League
	sport := req.Sport
	leagueLogoURL := ""
	if hasCfg {
		leagueName = cfg.DisplayName
		league = cfg.League
		sport = cfg.Sport
		leagueLogoURL = leagueIdentityForConfig(cfg).LogoURL
	}
	message := UserFacingError(req, err)
	if strings.TrimSpace(message) == "" {
		message = "ESPN returned no rows for this lookup right now."
	}
	table := SimpleTable{
		Headers: []string{"Status", "Detail"},
		Rows: [][]string{{
			emptyLookupStatus(err),
			message,
		}},
	}
	return &SportsLookupResult{
		Intent:        req.Intent,
		League:        league,
		LeagueName:    leagueName,
		LeagueLogoURL: leagueLogoURL,
		Sport:         sport,
		DateLabel:     req.DateLabel,
		Markdown:      RenderSimpleMarkdown(emptyLookupTitle(req, leagueName, err), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}
}

func emptyLookupTitle(req SportsRequest, leagueName string, err error) string {
	subject := strings.TrimSpace(firstNonEmpty(req.TeamQuery, req.AthleteQuery, leagueName))
	if subject == "" {
		subject = "ESPN"
	}
	action := strings.ReplaceAll(string(req.Intent), "_", " ")
	if action == "" || action == string(SportsIntentUnknown) {
		action = "lookup"
	}
	switch {
	case errors.Is(err, ErrNoGames), errors.Is(err, ErrNoMatchingGames):
		return fmt.Sprintf("### %s %s — No Scheduled Events Listed", subject, titleCaseWords(action))
	case errors.Is(err, ErrNoNews):
		return fmt.Sprintf("### %s News — No Articles Listed", subject)
	case errors.Is(err, ErrNoOdds):
		return fmt.Sprintf("### %s Odds — No Betting Lines Listed", subject)
	default:
		return fmt.Sprintf("### %s %s — No ESPN Rows Returned", subject, titleCaseWords(action))
	}
}

func emptyLookupStatus(err error) string {
	switch {
	case errors.Is(err, ErrNoGames), errors.Is(err, ErrNoMatchingGames):
		return "No scheduled events"
	case errors.Is(err, ErrNoNews):
		return "No articles listed"
	case errors.Is(err, ErrNoOdds):
		return "No betting lines listed"
	case errors.Is(err, ErrTeamNotFound):
		return "Team not found"
	case errors.Is(err, ErrAthleteNotFound):
		return "Athlete not found"
	default:
		return "No data returned"
	}
}

func titleCaseWords(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func (c *ESPNClient) LookupScores(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	req.League = cfg.League
	req.Sport = cfg.Sport
	if req.Intent == SportsIntentUnknown {
		req.Intent = SportsIntentScores
	}

	opts := &espn.ScoreboardOptions{}
	if req.Date != nil {
		opts.SetDate(*req.Date)
	}
	if req.Limit > 0 {
		opts.Limit = req.Limit
	} else {
		opts.Limit = 100
	}

	sb, err := c.client.Scoreboard(ctx, req.Sport, req.League, opts)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}

	leagueLogoURL := logoURLFromScoreboard(sb, cfg)
	req.LeagueLogoURL = leagueLogoURL
	rows := normalizeScoreboard(sb)
	if len(rows) == 0 {
		if cfg.Sport == espn.SportRacing {
			if result, resultErr := c.lookupRacingScoreboard(ctx, cfg, req, sb); resultErr == nil {
				return result, nil
			}
		}
		return nil, ErrNoGames
	}
	if req.TeamQuery != "" {
		rows = filterGameRowsByTeam(rows, req.TeamQuery)
		if len(rows) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrNoMatchingGames, req.TeamQuery)
		}
	}

	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        req.Intent,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueLogoURL,
		Sport:         cfg.Sport,
		DateLabel:     req.DateLabel,
		Markdown:      RenderGamesMarkdown(req, cfg, rows, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) lookupRacingScoreboard(ctx context.Context, cfg LeagueConfig, req SportsRequest, sb *espn.Scoreboard) (*SportsLookupResult, error) {
	norm := normalizeText(req.RawQuery)
	if hasAnyPhrase(norm, "most recent race", "latest race", "recent race", "who won") {
		if result, err := c.lookupRecentRacingResult(ctx, cfg, req, sb); err == nil {
			return result, nil
		}
	}
	table := racingScheduleTable(sb, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoGames
	}
	title := fmt.Sprintf("### %s Race Schedule", cfg.DisplayName)
	if label := strings.TrimSpace(req.DateLabel); label != "" {
		title += " — " + label
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        req.Intent,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		DateLabel:     req.DateLabel,
		Markdown:      RenderSimpleMarkdown(title, table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) lookupRecentRacingResult(ctx context.Context, cfg LeagueConfig, req SportsRequest, sb *espn.Scoreboard) (*SportsLookupResult, error) {
	eventID, eventName := recentRacingEventFromCalendar(sb, c.timeNow())
	if eventID == "" {
		return nil, ErrNoGames
	}
	raw, err := c.client.SummaryRaw(ctx, cfg.Sport, cfg.League, eventID)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentScores,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         cfg.Sport,
		Markdown:      RenderSimpleMarkdown(fmt.Sprintf("### %s Result", firstNonEmpty(eventName, cfg.DisplayName+" Race")), table, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func racingScheduleTable(sb *espn.Scoreboard, limit int) SimpleTable {
	if sb == nil {
		return SimpleTable{}
	}
	if limit <= 0 {
		limit = 25
	}
	rows := make([][]string, 0, limit)
	for _, event := range sb.Events {
		for _, comp := range event.Competitions {
			if len(rows) >= limit {
				break
			}
			session := racingCompetitionType(comp.Type)
			if session == "" {
				session = firstNonEmpty(event.ShortName, event.Name)
			}
			rows = append(rows, []string{
				emptyAsDash(firstNonEmpty(event.Name, event.ShortName)),
				emptyAsDash(session),
				emptyAsDash(formatGameDate(firstNonEmpty(comp.Date, event.Date))),
				emptyAsDash(formatGameTime(firstNonEmpty(comp.Date, event.Date))),
				emptyAsDash(statusText(chooseStatus(event.Status, comp.Status), firstNonEmpty(comp.Date, event.Date))),
				emptyAsDash(broadcastNames(comp.Broadcasts, comp.GeoBroadcasts)),
			})
		}
	}
	if len(rows) == 0 {
		rows = racingCalendarRows(sb, limit)
	}
	return SimpleTable{Headers: []string{"Race", "Session", "Date", "Time", "Status", "Broadcast"}, Rows: rows}
}

func racingCalendarRows(sb *espn.Scoreboard, limit int) [][]string {
	var rows [][]string
	for _, league := range sb.Leagues {
		for _, item := range league.Calendar {
			if len(rows) >= limit {
				return rows
			}
			cal, ok := racingCalendarItem(item)
			if !ok {
				continue
			}
			rows = append(rows, []string{
				emptyAsDash(cal.Label),
				"Race weekend",
				emptyAsDash(formatGameDate(cal.StartDate)),
				emptyAsDash(formatGameTime(cal.StartDate)),
				emptyAsDash(dateRangeLabel(cal.StartDate, cal.EndDate)),
				"",
			})
		}
	}
	return rows
}

func racingCompetitionType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value struct {
		Text         string `json:"text"`
		Name         string `json:"name"`
		Abbreviation string `json:"abbreviation"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return firstNonEmpty(value.Abbreviation, value.Text, value.Name)
}

type racingCalItem struct {
	Label     string
	StartDate string
	EndDate   string
	EventID   string
}

func racingCalendarItem(item any) (racingCalItem, bool) {
	raw, err := json.Marshal(item)
	if err != nil {
		return racingCalItem{}, false
	}
	var cal struct {
		Label     string `json:"label"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		Event     struct {
			Ref string `json:"$ref"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &cal); err != nil {
		return racingCalItem{}, false
	}
	return racingCalItem{
		Label:     cal.Label,
		StartDate: cal.StartDate,
		EndDate:   cal.EndDate,
		EventID:   extractIDFromRef(cal.Event.Ref),
	}, strings.TrimSpace(cal.Label) != ""
}

func recentRacingEventFromCalendar(sb *espn.Scoreboard, now time.Time) (string, string) {
	var best racingCalItem
	var bestEnd time.Time
	for _, league := range sb.Leagues {
		for _, item := range league.Calendar {
			cal, ok := racingCalendarItem(item)
			if !ok || cal.EventID == "" {
				continue
			}
			end, ok := parseESPNTime(cal.EndDate)
			if !ok || end.After(now) {
				continue
			}
			if best.EventID == "" || end.After(bestEnd) {
				best = cal
				bestEnd = end
			}
		}
	}
	return best.EventID, best.Label
}

func dateRangeLabel(start, end string) string {
	startTime, startOK := parseESPNTime(start)
	endTime, endOK := parseESPNTime(end)
	if !startOK || !endOK {
		return ""
	}
	if startTime.Year() == endTime.Year() && startTime.Month() == endTime.Month() && startTime.Day() == endTime.Day() {
		return startTime.Local().Format("Jan 2")
	}
	return startTime.Local().Format("Jan 2") + "-" + endTime.Local().Format("Jan 2")
}

func (c *ESPNClient) LookupStandings(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}
	req.League = cfg.League
	req.Sport = cfg.Sport
	req.Intent = SportsIntentStandings

	st, err := c.client.Standings(ctx, req.Sport, req.League, req.Season)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	rows := normalizeStandings(st)
	if len(rows) == 0 {
		return nil, ErrNoStandings
	}

	retrievedAt := c.timeNow()
	leagueLogoURL := leagueIdentityForConfig(cfg).LogoURL
	req.LeagueLogoURL = leagueLogoURL
	return &SportsLookupResult{
		Intent:        SportsIntentStandings,
		League:        cfg.League,
		LeagueName:    cfg.DisplayName,
		LeagueLogoURL: leagueLogoURL,
		Sport:         cfg.Sport,
		DateLabel:     req.DateLabel,
		Markdown:      RenderStandingsMarkdown(req, cfg, rows, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) LookupNews(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimitForIntent(SportsIntentNews)
	}
	req.Intent = SportsIntentNews

	var cfg LeagueConfig
	hasCfg := false
	if req.League != "" || req.Sport != "" {
		var ok bool
		cfg, ok = leagueConfigForRequest(req)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
		}
		hasCfg = true
		req.League = cfg.League
		req.Sport = cfg.Sport
	}

	var rows []NewsRow
	if req.TeamQuery != "" {
		if !hasCfg {
			return nil, fmt.Errorf("%w: missing league for team news", ErrUnsupportedLeague)
		}
		team, err := c.resolveTeam(ctx, cfg, req.TeamQuery)
		if err != nil {
			return nil, err
		}
		req.TeamQuery = teamDisplayName(*team)
		// Try the ESPN team-specific news endpoint first.
		if teamFeed, teamErr := c.client.TeamNews(ctx, cfg.Sport, cfg.League, team.ID, limit); teamErr == nil {
			rows = normalizeNewsFeed(teamFeed)
		}
		// Some ESPN team news endpoints return an empty object even when the
		// league news endpoint can filter by team ID.
		if len(rows) == 0 {
			if teamRows, teamErr := c.lookupTeamNewsByLeagueParam(ctx, cfg, team.ID, limit); teamErr == nil {
				rows = teamRows
			}
		}
		// Fall back to league news filtered by team name when the team endpoint
		// returns nothing (ESPN's team news API is often inactive for some leagues).
		if len(rows) == 0 {
			fetchLimit := limit * 5
			if fetchLimit < 50 {
				fetchLimit = 50
			}
			leagueFeed, leagueErr := c.client.News(ctx, cfg.Sport, cfg.League, fetchLimit)
			if leagueErr != nil {
				return nil, wrapESPNError(ctx, leagueErr)
			}
			allRows := normalizeNewsFeed(leagueFeed)
			rows = filterNewsByTeam(allRows, req.TeamQuery)
			if len(rows) > limit {
				rows = rows[:limit]
			}
		}
	} else if hasCfg {
		feed, err := c.client.News(ctx, cfg.Sport, cfg.League, limit)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		rows = normalizeNewsFeed(feed)
	} else {
		var err error
		rows, err = c.lookupBroadNewsRows(ctx, limit)
		if err != nil {
			return nil, err
		}
	}

	if len(rows) == 0 {
		return nil, ErrNoNews
	}

	leagueName := "Sports"
	league := ""
	sport := ""
	if hasCfg {
		leagueName = cfg.DisplayName
		league = cfg.League
		sport = cfg.Sport
	}

	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:        SportsIntentNews,
		League:        league,
		LeagueName:    leagueName,
		LeagueLogoURL: leagueIdentityForConfig(cfg).LogoURL,
		Sport:         sport,
		DateLabel:     req.DateLabel,
		Markdown:      RenderNewsMarkdown(req, leagueName, rows, retrievedAt),
		Source:        SourceESPN,
		RetrievedAt:   retrievedAt,
		RenderMode:    renderMode(req),
	}, nil
}

func (c *ESPNClient) lookupBroadNewsRows(ctx context.Context, limit int) ([]NewsRow, error) {
	params := espn.Params{}
	if limit > 0 {
		params["limit"] = limit
	}
	raw, err := c.client.GetRaw(ctx, espn.DomainNow, "/v1/sports/news", params)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	rows, err := normalizeNowNewsPayload(raw)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *ESPNClient) lookupTeamNewsByLeagueParam(ctx context.Context, cfg LeagueConfig, teamID string, limit int) ([]NewsRow, error) {
	if strings.TrimSpace(teamID) == "" {
		return nil, ErrTeamNotFound
	}
	path := fmt.Sprintf("/apis/site/v2/sports/%s/%s/news", cfg.Sport, cfg.League)
	params := espn.Params{"team": teamID}
	if limit > 0 {
		params["limit"] = limit
	}
	raw, err := c.client.GetRaw(ctx, espn.DomainSite, path, params)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	var feed espn.NewsFeed
	if err := json.Unmarshal(raw, &feed); err != nil {
		return nil, fmt.Errorf("decode espn team-filtered news: %w", err)
	}
	return normalizeNewsFeed(&feed), nil
}

func (c *ESPNClient) timeNow() time.Time {
	if c != nil && c.now != nil {
		return c.now()
	}
	return time.Now()
}

func MaybeHandleSportsQuery(ctx context.Context, query string, now time.Time) (*SportsLookupResult, bool, error) {
	req, ok := DetectSportsIntent(query, now)
	if !ok {
		return nil, false, nil
	}
	if err := ValidateDateInQuery(query, now); err != nil {
		return nil, true, err
	}
	result, err := NewESPNClient().Lookup(ctx, *req)
	return result, true, err
}

func wrapESPNError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err == nil {
		return nil
	}
	if errors.Is(err, espn.ErrNotFound) {
		return fmt.Errorf("%w: %v", ErrNoSportsData, err)
	}
	if errors.Is(err, espn.ErrRateLimited) {
		return fmt.Errorf("%w: %v", ErrRateLimited, err)
	}
	// ESPN returns HTTP 400/404 for unsupported queries (e.g. historical seasons
	// that predate the API's coverage, or parameters the endpoint doesn't accept).
	// Map these to ErrNoSportsData so ErrorForRequest can render a helpful message.
	if s := err.Error(); strings.Contains(s, "status 400") || strings.Contains(s, "status 404") {
		return fmt.Errorf("%w: %v", ErrNoSportsData, err)
	}
	return err
}

func normalizeScoreboard(sb *espn.Scoreboard) []GameRow {
	if sb == nil || len(sb.Events) == 0 {
		return nil
	}

	rows := make([]GameRow, 0, len(sb.Events))
	for _, ev := range sb.Events {
		var comp espn.Competition
		if len(ev.Competitions) > 0 {
			comp = ev.Competitions[0]
		}

		status := chooseStatus(ev.Status, comp.Status)
		eventDate := firstNonEmpty(comp.Date, ev.Date)
		away, home := splitCompetitors(comp.Competitors)
		if away == nil && home == nil {
			continue
		}

		row := GameRow{
			Date:       formatGameDate(eventDate),
			Time:       formatGameTime(eventDate),
			Status:     statusText(status, eventDate),
			StatusType: statusKind(status),
			Venue:      venueName(comp.Venue),
			Broadcasts: broadcastNames(comp.Broadcasts, comp.GeoBroadcasts),
		}
		if away != nil {
			row.Away = extractTeamIdentityFromCompetitor(*away)
			row.AwayTeam = row.Away.DisplayName
			row.AwayAbbr = row.Away.Abbreviation
			row.AwayScore = strings.TrimSpace(away.Score)
		}
		if home != nil {
			row.Home = extractTeamIdentityFromCompetitor(*home)
			row.HomeTeam = row.Home.DisplayName
			row.HomeAbbr = row.Home.Abbreviation
			row.HomeScore = strings.TrimSpace(home.Score)
		}
		row.LinescoreRows = buildLinescoreRows(away, home)
		rows = append(rows, row)
	}
	return rows
}

// buildLinescoreRows extracts period-by-period scores from two competitors.
// Returns nil if neither competitor has linescore data.
func buildLinescoreRows(away, home *espn.Competitor) []LinescoreRow {
	if away == nil || home == nil {
		return nil
	}
	maxPeriods := len(away.Linescores)
	if len(home.Linescores) > maxPeriods {
		maxPeriods = len(home.Linescores)
	}
	if maxPeriods == 0 {
		return nil
	}
	rows := make([]LinescoreRow, maxPeriods)
	for i := 0; i < maxPeriods; i++ {
		period := i + 1
		awayScore := ""
		homeScore := ""
		if i < len(away.Linescores) {
			period = away.Linescores[i].Period
			if period == 0 {
				period = i + 1
			}
			awayScore = away.Linescores[i].DisplayValue
			if awayScore == "" {
				awayScore = fmt.Sprintf("%.0f", away.Linescores[i].Value)
			}
		}
		if i < len(home.Linescores) {
			if home.Linescores[i].Period != 0 {
				period = home.Linescores[i].Period
			}
			homeScore = home.Linescores[i].DisplayValue
			if homeScore == "" {
				homeScore = fmt.Sprintf("%.0f", home.Linescores[i].Value)
			}
		}
		rows[i] = LinescoreRow{Period: period, AwayScore: awayScore, HomeScore: homeScore}
	}
	return rows
}

func normalizeStandings(st *espn.Standings) []StandingsRow {
	if st == nil {
		return nil
	}
	var rows []StandingsRow
	root := espn.StandingsGroup{
		Name:      firstNonEmpty(st.DisplayName, st.Name, st.Abbreviation),
		Standings: st.Standings,
		Children:  st.Children,
	}
	collectStandingsRows("", root, &rows)
	return rows
}

func normalizeNewsFeed(feed *espn.NewsFeed) []NewsRow {
	if feed == nil || len(feed.Articles) == 0 {
		return nil
	}
	rows := make([]NewsRow, 0, len(feed.Articles))
	for _, article := range feed.Articles {
		row := NewsRow{
			Published:   formatNewsPublished(article.Published),
			Headline:    strings.TrimSpace(article.Headline),
			Description: compactNewsDescription(article.Description),
			Byline:      strings.TrimSpace(article.Byline),
			URL:         articleURL(article.Links),
			ImageURL:    firstNewsImageURL(article.Images),
			ImageAlt:    firstNewsImageAlt(article.Images, article.Headline),
		}
		if row.Headline == "" && row.Description == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

// teamQueryVariants returns all normalized name variants for a given team name
// by consulting the package-level teamAliases table. If no alias entry is
// found, the result is a single-element slice with the normalized input.
func teamQueryVariants(teamName string) []string {
	normName := normalizeText(teamName)
	for _, ta := range teamAliases {
		if normalizeText(ta.TeamQuery) == normName {
			return collectNormTeamAliases(ta)
		}
		for _, a := range ta.Aliases {
			if normalizeText(a) == normName {
				return collectNormTeamAliases(ta)
			}
		}
	}
	return []string{normName}
}

func collectNormTeamAliases(ta teamAlias) []string {
	seen := make(map[string]struct{})
	var result []string
	add := func(s string) {
		n := normalizeText(s)
		if n != "" {
			if _, dup := seen[n]; !dup {
				seen[n] = struct{}{}
				result = append(result, n)
			}
		}
	}
	add(ta.TeamQuery)
	for _, a := range ta.Aliases {
		add(a)
	}
	return result
}

// filterNewsByTeam returns rows whose headline or description mentions the team.
// Abbreviations (e.g. "BAL") are expanded via teamAliases so that a query for
// "BAL" also matches headlines containing "Baltimore Ravens".
// Used as a fallback when the ESPN team-specific news endpoint returns nothing.
func filterNewsByTeam(rows []NewsRow, teamName string) []NewsRow {
	if teamName == "" || len(rows) == 0 {
		return rows
	}
	variants := teamQueryVariants(teamName)
	var filtered []NewsRow
	for _, row := range rows {
		normH := normalizeText(row.Headline)
		normD := normalizeText(row.Description)
		for _, v := range variants {
			if strings.Contains(normH, v) || strings.Contains(normD, v) {
				filtered = append(filtered, row)
				break
			}
		}
	}
	return filtered
}

func normalizeNowFeed(feed *espn.NowFeed) []NewsRow {
	if feed == nil || len(feed.Feed) == 0 {
		return nil
	}
	items := make([]nowNewsItem, 0, len(feed.Feed))
	for _, item := range feed.Feed {
		items = append(items, nowNewsItem{
			Description: item.Description,
			Published:   item.Published,
			Headline:    item.Headline,
			Links:       mustMarshalJSON(item.Links),
			Images:      mustMarshalJSON(item.Images),
		})
	}
	return normalizeNowNewsItems(items)
}

func normalizeNowNewsPayload(raw []byte) ([]NewsRow, error) {
	var payload nowNewsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode espn now news: %w", err)
	}
	items := payload.Feed
	if len(items) == 0 {
		items = payload.Headlines
	}
	return normalizeNowNewsItems(items), nil
}

type nowNewsPayload struct {
	Feed      []nowNewsItem `json:"feed"`
	Headlines []nowNewsItem `json:"headlines"`
}

type nowNewsItem struct {
	Description string          `json:"description"`
	Published   string          `json:"published"`
	Headline    string          `json:"headline"`
	Title       string          `json:"title"`
	LinkText    string          `json:"linkText"`
	Links       json.RawMessage `json:"links"`
	Images      json.RawMessage `json:"images"`
}

func normalizeNowNewsItems(items []nowNewsItem) []NewsRow {
	if len(items) == 0 {
		return nil
	}
	rows := make([]NewsRow, 0, len(items))
	for _, item := range items {
		headline := firstNonEmpty(item.Headline, item.Title, item.LinkText)
		images := nowNewsImages(item.Images)
		row := NewsRow{
			Published:   formatNewsPublished(item.Published),
			Headline:    strings.TrimSpace(headline),
			Description: compactNewsDescription(item.Description),
			URL:         nowNewsURL(item.Links),
			ImageURL:    firstNewsImageURL(images),
			ImageAlt:    firstNewsImageAlt(images, headline),
		}
		if row.Headline == "" && row.Description == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func nowNewsURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var links espn.ArticleLinks
	if err := json.Unmarshal(raw, &links); err == nil {
		if url := articleURL(links); url != "" {
			return url
		}
	}
	var list []espn.Link
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, link := range list {
			if href := strings.TrimSpace(link.Href); href != "" {
				return href
			}
		}
	}
	var link espn.Link
	if err := json.Unmarshal(raw, &link); err == nil {
		return strings.TrimSpace(link.Href)
	}
	var href struct {
		Href string `json:"href"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(raw, &href); err == nil {
		return firstNonEmpty(href.Href, href.URL)
	}
	return ""
}

func nowNewsImages(raw json.RawMessage) []espn.Image {
	if len(raw) == 0 {
		return nil
	}
	var images []espn.Image
	if err := json.Unmarshal(raw, &images); err == nil {
		return images
	}
	var image espn.Image
	if err := json.Unmarshal(raw, &image); err == nil {
		return []espn.Image{image}
	}
	return nil
}

func mustMarshalJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func collectStandingsRows(groupName string, group espn.StandingsGroup, rows *[]StandingsRow) {
	currentGroup := combinedGroupName(groupName, standingsGroupName(group))
	if group.Standings != nil {
		if currentGroup == "" {
			currentGroup = group.Standings.Name
		}
		for _, entry := range group.Standings.Entries {
			*rows = append(*rows, standingsRowFromEntry(currentGroup, entry))
		}
	}
	for _, child := range group.Children {
		collectStandingsRows(currentGroup, child, rows)
	}
}

func standingsRowFromEntry(group string, entry espn.StandingsEntry) StandingsRow {
	rank := statIntAny(entry, []string{"rank"})
	team := extractTeamIdentityFromStandingEntry(entry)
	row := StandingsRow{
		Group:            group,
		Rank:             rank,
		TeamIdentity:     team,
		Team:             team.DisplayName,
		Abbr:             team.Abbreviation,
		Wins:             statDisplayAny(entry, []string{"wins", "matchesWon", "matches won", "W"}),
		Losses:           statDisplayAny(entry, []string{"losses", "matchesLost", "matches lost", "L"}),
		Ties:             statDisplayAny(entry, []string{"ties", "tied", "matchesTied", "matches tied", "otLosses", "overtimeLosses", "OT", "OTL", "T"}),
		Draws:            statDisplayAny(entry, []string{"draws", "ties", "D"}),
		NoResult:         statDisplayAny(entry, []string{"noResult", "noResults", "no result", "no results", "N/R", "NR"}),
		Pct:              statDisplayAny(entry, []string{"winPercent", "winPercentage", "percentage", "pct"}),
		GamesBack:        statDisplayAny(entry, []string{"gamesBack", "gamesBehind", "GB"}),
		Streak:           statDisplayAny(entry, []string{"streak", "STREAK"}),
		LastTen:          statDisplayAny(entry, []string{"lastTenGames", "lastTen", "lastTenRecord", "L10"}),
		Points:           statDisplayAny(entry, []string{"points", "matchPoints", "match points", "PTS", "PT"}),
		GamesPlayed:      statDisplayAny(entry, []string{"gamesPlayed", "matchesPlayed", "matches", "played", "GP", "M"}),
		GoalDifferential: statDisplayAny(entry, []string{"goalDifferential", "goalDifference", "pointDifferential", "GD"}),
		NetRunRate:       statDisplayAny(entry, []string{"netRunRate", "net run rate", "netrr", "net rr", "NRR"}),
		For:              statDisplayAny(entry, []string{"for", "runsFor", "runs for", "FOR"}),
		Against:          statDisplayAny(entry, []string{"against", "runsAgainst", "runs against"}),
	}
	row.GoalDiff = row.GoalDifferential
	if entry.Note != nil {
		row.Note = strings.TrimSpace(entry.Note.Description)
	}
	return row
}

func statDisplayAny(entry espn.StandingsEntry, names []string) string {
	for _, stat := range entry.Stats {
		if statMatchesAny(stat, names) {
			if strings.TrimSpace(stat.DisplayValue) != "" {
				return strings.TrimSpace(stat.DisplayValue)
			}
			return formatFloatStat(stat.Value)
		}
	}
	return ""
}

func statIntAny(entry espn.StandingsEntry, names []string) int {
	value := statDisplayAny(entry, names)
	if value == "" {
		return 0
	}
	value = strings.TrimSpace(strings.TrimSuffix(value, ".0"))
	n, err := strconv.Atoi(value)
	if err == nil {
		return n
	}
	return 0
}

func statMatchesAny(stat espn.Statistic, names []string) bool {
	fields := []string{stat.Name, stat.DisplayName, stat.ShortDisplayName, stat.Abbreviation}
	for _, field := range fields {
		normField := normalizeText(field)
		for _, name := range names {
			if normField == normalizeText(name) {
				return true
			}
		}
	}
	return false
}

func splitCompetitors(competitors []espn.Competitor) (*espn.Competitor, *espn.Competitor) {
	var away, home *espn.Competitor
	for i := range competitors {
		switch strings.ToLower(competitors[i].HomeAway) {
		case "away":
			away = &competitors[i]
		case "home":
			home = &competitors[i]
		}
	}
	if away == nil && len(competitors) > 0 {
		away = &competitors[0]
	}
	if home == nil && len(competitors) > 1 {
		home = &competitors[1]
	}
	return away, home
}

func chooseStatus(eventStatus, competitionStatus espn.Status) espn.Status {
	if competitionStatus.Type.ShortDetail != "" ||
		competitionStatus.Type.Detail != "" ||
		competitionStatus.Type.Description != "" ||
		competitionStatus.Type.State != "" {
		return competitionStatus
	}
	return eventStatus
}

func statusText(status espn.Status, eventDate string) string {
	st := status.Type
	for _, candidate := range []string{st.ShortDetail, st.Detail, st.Description, st.State, st.Name} {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}
	if t := formatGameTime(eventDate); t != "" {
		return t
	}
	return ""
}

func statusKind(status espn.Status) string {
	st := status.Type
	text := normalizeText(firstNonEmpty(st.State, st.Name, st.Description, st.Detail, st.ShortDetail))
	switch {
	case strings.Contains(text, "postponed"), strings.Contains(text, "canceled"), strings.Contains(text, "cancelled"), strings.Contains(text, "suspended"):
		return "postponed"
	case st.Completed, text == "post", strings.Contains(text, "final"):
		return "final"
	case text == "in", strings.Contains(text, "progress"), strings.Contains(text, "halftime"):
		return "live"
	case text == "pre", strings.Contains(text, "scheduled"), strings.Contains(text, "preview"):
		return "scheduled"
	default:
		return strings.TrimSpace(st.State)
	}
}

func teamDisplayName(team espn.Team) string {
	return firstNonEmpty(team.DisplayName, team.ShortDisplayName, team.Name, team.Location, team.Abbreviation)
}

func (c *ESPNClient) resolveTeam(ctx context.Context, cfg LeagueConfig, teamQuery string) (*espn.Team, error) {
	resp, err := c.client.Teams(ctx, cfg.Sport, cfg.League, 100)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	query := normalizeText(teamQuery)
	for _, team := range resp.Flatten() {
		if espnTeamMatchesQuery(team, query) {
			t := team
			return &t, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrTeamNotFound, teamQuery)
}

func espnTeamMatchesQuery(team espn.Team, query string) bool {
	if query == "" {
		return false
	}
	fields := []string{team.ID, team.DisplayName, team.ShortDisplayName, team.Name, team.Nickname, team.Abbreviation, team.Slug}
	for _, field := range fields {
		norm := normalizeText(field)
		if norm == "" {
			continue
		}
		if norm == query || strings.Contains(norm, query) {
			return true
		}
	}
	if location := normalizeText(team.Location); location != "" && location == query {
		return true
	}
	return false
}

func venueName(venue *espn.Venue) string {
	if venue == nil {
		return ""
	}
	return strings.TrimSpace(venue.FullName)
}

func broadcastNames(broadcasts []espn.Broadcast, geo []espn.GeoBroadcast) string {
	seen := map[string]bool{}
	var names []string
	for _, b := range broadcasts {
		for _, name := range b.Names {
			name = strings.TrimSpace(name)
			if name != "" && !seen[strings.ToLower(name)] {
				seen[strings.ToLower(name)] = true
				names = append(names, name)
			}
		}
	}
	for _, g := range geo {
		name := strings.TrimSpace(g.Media.ShortName)
		if name != "" && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func filterGameRowsByTeam(rows []GameRow, teamQuery string) []GameRow {
	query := normalizeText(teamQuery)
	if query == "" {
		return rows
	}
	filtered := make([]GameRow, 0, len(rows))
	for _, row := range rows {
		if rowMatchesTeam(row, query) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func rowMatchesTeam(row GameRow, query string) bool {
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

func standingsGroupName(group espn.StandingsGroup) string {
	return firstNonEmpty(group.Name, group.Abbreviation)
}

func combinedGroupName(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)
	switch {
	case parent == "":
		return child
	case child == "":
		return parent
	case strings.Contains(strings.ToLower(child), strings.ToLower(parent)):
		return child
	case isDirectionalDivision(child):
		return parent + " " + child
	default:
		return child
	}
}

func isDirectionalDivision(name string) bool {
	n := normalizeText(name)
	switch n {
	case "east", "central", "west", "north", "south", "division":
		return true
	default:
		return strings.HasSuffix(n, " division")
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func formatGameDate(value string) string {
	t, ok := parseESPNTime(value)
	if !ok {
		return ""
	}
	return t.Local().Format("Jan 2")
}

func formatGameTime(value string) string {
	t, ok := parseESPNTime(value)
	if !ok {
		return ""
	}
	return t.Local().Format("3:04 PM")
}

func parseESPNTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04Z0700",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05.000Z0700",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func formatNewsPublished(value string) string {
	if t, ok := parseESPNTime(value); ok {
		return t.Local().Format("Jan 2, 3:04 PM")
	}
	return strings.TrimSpace(value)
}

func compactNewsDescription(value string) string {
	value = html.UnescapeString(stripHTMLTags(value))
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if len(value) <= 220 {
		return value
	}
	return strings.TrimSpace(value[:217]) + "..."
}

func stripHTMLTags(value string) string {
	var b strings.Builder
	inTag := false
	for _, r := range value {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func articleURL(links espn.ArticleLinks) string {
	for _, link := range []*espn.Link{links.Web, links.Mobile, links.App, links.API} {
		if link != nil && strings.TrimSpace(link.Href) != "" {
			return strings.TrimSpace(link.Href)
		}
	}
	return ""
}

func firstNewsImageURL(images []espn.Image) string {
	for _, image := range images {
		if url := sanitizeImageURL(image.URL); url != "" {
			return url
		}
	}
	return ""
}

func firstNewsImageAlt(images []espn.Image, fallback string) string {
	for _, image := range images {
		if alt := strings.TrimSpace(firstNonEmpty(image.Alt, image.Caption, image.Name)); alt != "" {
			return alt
		}
	}
	return strings.TrimSpace(fallback)
}

func formatFloatStat(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	if v == math.Trunc(v) {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if strings.HasPrefix(s, "0.") {
		s = strings.TrimPrefix(s, "0")
	}
	if strings.HasPrefix(s, "-0.") {
		s = "-" + strings.TrimPrefix(s, "-0")
	}
	return s
}
