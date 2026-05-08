package sports

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	espn "github.com/chinmaykhachane/espn-go"
)

var exactDatePattern = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
var slashDatePattern = regexp.MustCompile(`\b\d{1,2}/\d{1,2}/\d{4}\b`)
var seasonPattern = regexp.MustCompile(`\b(19|20)\d{2}\b`)
var topLimitPattern = regexp.MustCompile(`\btop\s+(\d{1,3})\b`)

var leagueConfigs = []LeagueConfig{
	{
		DisplayName: "MLB",
		Sport:       espn.SportBaseball,
		League:      espn.LeagueMLB,
		Aliases:     []string{"mlb", "baseball", "major league baseball"},
	},
	{
		DisplayName: "NFL",
		Sport:       espn.SportFootball,
		League:      espn.LeagueNFL,
		Aliases:     []string{"nfl", "football", "pro football"},
	},
	{
		DisplayName: "NBA",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueNBA,
		Aliases:     []string{"nba", "basketball"},
	},
	{
		DisplayName: "WNBA",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueWNBA,
		Aliases:     []string{"wnba"},
	},
	{
		DisplayName: "NHL",
		Sport:       espn.SportHockey,
		League:      espn.LeagueNHL,
		Aliases:     []string{"nhl", "hockey"},
	},
	{
		DisplayName: "College Football",
		Sport:       espn.SportFootball,
		League:      espn.LeagueCollegeFootball,
		Aliases:     []string{"college football", "ncaaf", "cfb"},
	},
	{
		DisplayName: "Men's College Basketball",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueMensCollegeBball,
		Aliases:     []string{"men's college basketball", "mens college basketball", "ncaamb", "college basketball"},
	},
	{
		DisplayName: "Women's College Basketball",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueWomensCollegeBall,
		Aliases:     []string{"women's college basketball", "womens college basketball", "ncaawb"},
	},
	{
		DisplayName: "Premier League",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueEPL,
		Aliases:     []string{"premier league", "epl", "english premier league"},
	},
	{
		DisplayName: "MLS",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueMLS,
		Aliases:     []string{"mls", "major league soccer"},
	},
}

type teamAlias struct {
	League    string
	TeamQuery string
	Aliases   []string
}

var teamAliases = []teamAlias{
	{League: espn.LeagueMLB, TeamQuery: "New York Yankees", Aliases: []string{"yankees", "new york yankees", "nyy"}},
	{League: espn.LeagueMLB, TeamQuery: "Chicago Cubs", Aliases: []string{"cubs", "chicago cubs", "chc"}},
	{League: espn.LeagueMLB, TeamQuery: "Boston Red Sox", Aliases: []string{"red sox", "boston red sox"}},
	{League: espn.LeagueMLB, TeamQuery: "Los Angeles Dodgers", Aliases: []string{"dodgers", "los angeles dodgers", "la dodgers", "lad"}},
	{League: espn.LeagueMLB, TeamQuery: "New York Mets", Aliases: []string{"mets", "new york mets", "nym"}},
	{League: espn.LeagueMLB, TeamQuery: "Philadelphia Phillies", Aliases: []string{"phillies", "philadelphia phillies"}},
	{League: espn.LeagueMLB, TeamQuery: "Atlanta Braves", Aliases: []string{"braves", "atlanta braves"}},
	{League: espn.LeagueMLB, TeamQuery: "St. Louis Cardinals", Aliases: []string{"st louis cardinals", "st. louis cardinals"}},
	{League: espn.LeagueMLB, TeamQuery: "San Francisco Giants", Aliases: []string{"san francisco giants", "sf giants", "sfg"}},
	{League: espn.LeagueMLB, TeamQuery: "San Diego Padres", Aliases: []string{"padres", "san diego padres"}},
	{League: espn.LeagueNFL, TeamQuery: "Kansas City Chiefs", Aliases: []string{"chiefs", "kansas city chiefs", "kc chiefs"}},
	{League: espn.LeagueNFL, TeamQuery: "Dallas Cowboys", Aliases: []string{"cowboys", "dallas cowboys"}},
	{League: espn.LeagueNFL, TeamQuery: "Green Bay Packers", Aliases: []string{"packers", "green bay packers"}},
	{League: espn.LeagueNFL, TeamQuery: "San Francisco 49ers", Aliases: []string{"49ers", "niners", "san francisco 49ers", "sf 49ers"}},
	{League: espn.LeagueNFL, TeamQuery: "New England Patriots", Aliases: []string{"patriots", "new england patriots"}},
	{League: espn.LeagueNFL, TeamQuery: "Philadelphia Eagles", Aliases: []string{"eagles", "philadelphia eagles"}},
	{League: espn.LeagueNFL, TeamQuery: "Buffalo Bills", Aliases: []string{"bills", "buffalo bills"}},
	{League: espn.LeagueNFL, TeamQuery: "Chicago Bears", Aliases: []string{"bears", "chicago bears"}},
	{League: espn.LeagueNFL, TeamQuery: "Detroit Lions", Aliases: []string{"lions", "detroit lions"}},
	{League: espn.LeagueNBA, TeamQuery: "Los Angeles Lakers", Aliases: []string{"lakers", "los angeles lakers", "la lakers"}},
	{League: espn.LeagueNBA, TeamQuery: "Boston Celtics", Aliases: []string{"celtics", "boston celtics"}},
	{League: espn.LeagueNBA, TeamQuery: "Golden State Warriors", Aliases: []string{"warriors", "golden state warriors", "gsw"}},
	{League: espn.LeagueNBA, TeamQuery: "New York Knicks", Aliases: []string{"knicks", "new york knicks"}},
	{League: espn.LeagueNBA, TeamQuery: "Chicago Bulls", Aliases: []string{"bulls", "chicago bulls"}},
	{League: espn.LeagueNBA, TeamQuery: "Miami Heat", Aliases: []string{"heat", "miami heat"}},
	{League: espn.LeagueNBA, TeamQuery: "Denver Nuggets", Aliases: []string{"nuggets", "denver nuggets"}},
	{League: espn.LeagueNBA, TeamQuery: "Dallas Mavericks", Aliases: []string{"mavericks", "mavs", "dallas mavericks"}},
	{League: espn.LeagueNBA, TeamQuery: "Phoenix Suns", Aliases: []string{"suns", "phoenix suns"}},
	{League: espn.LeagueNHL, TeamQuery: "Boston Bruins", Aliases: []string{"bruins", "boston bruins"}},
	{League: espn.LeagueNHL, TeamQuery: "Toronto Maple Leafs", Aliases: []string{"maple leafs", "leafs", "toronto maple leafs"}},
	{League: espn.LeagueNHL, TeamQuery: "New York Rangers", Aliases: []string{"new york rangers", "ny rangers"}},
	{League: espn.LeagueNHL, TeamQuery: "Chicago Blackhawks", Aliases: []string{"blackhawks", "chicago blackhawks"}},
	{League: espn.LeagueNHL, TeamQuery: "Colorado Avalanche", Aliases: []string{"avalanche", "avs", "colorado avalanche"}},
	{League: espn.LeagueNHL, TeamQuery: "Tampa Bay Lightning", Aliases: []string{"lightning", "tampa bay lightning"}},
	{League: espn.LeagueNHL, TeamQuery: "Florida Panthers", Aliases: []string{"florida panthers"}},
	{League: espn.LeagueNHL, TeamQuery: "Vegas Golden Knights", Aliases: []string{"golden knights", "vegas golden knights", "vgk"}},
}

type statMetricConfig struct {
	Aliases       []string
	DefaultLeague string
	Category      string
	StatName      string
	Label         string
	Sort          string
	DisplayName   string
	Ascending     bool
}

var statMetricConfigs = []statMetricConfig{
	{Aliases: []string{"hr", "home run", "home runs", "homer", "homers"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "homeRuns", Label: "HR", Sort: "batting.homeRuns:desc", DisplayName: "Home Runs"},
	{Aliases: []string{"rbi", "rbis", "runs batted in"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "RBIs", Label: "RBI", Sort: "batting.RBIs:desc", DisplayName: "RBI"},
	{Aliases: []string{"batting average", "avg"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "avg", Label: "AVG", Sort: "batting.avg:desc", DisplayName: "Batting Average"},
	{Aliases: []string{"hits"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "hits", Label: "H", Sort: "batting.hits:desc", DisplayName: "Hits"},
	{Aliases: []string{"stolen bases"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "stolenBases", Label: "SB", Sort: "batting.stolenBases:desc", DisplayName: "Stolen Bases"},
	{Aliases: []string{"era", "earned run average"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "ERA", Label: "ERA", Sort: "pitching.ERA:asc", DisplayName: "ERA", Ascending: true},
	{Aliases: []string{"strikeout", "strikeouts", "ks"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "strikeouts", Label: "K", Sort: "pitching.strikeouts:desc", DisplayName: "Strikeouts"},
	{Aliases: []string{"saves"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "saves", Label: "SV", Sort: "pitching.saves:desc", DisplayName: "Saves"},
	{Aliases: []string{"whip"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "WHIP", Label: "WHIP", Sort: "pitching.WHIP:asc", DisplayName: "WHIP", Ascending: true},
	{Aliases: []string{"passing yards"}, DefaultLeague: espn.LeagueNFL, Category: "passing", StatName: "passingYards", Label: "YDS", Sort: "passing.passingYards:desc", DisplayName: "Passing Yards"},
	{Aliases: []string{"passing touchdowns", "passing tds"}, DefaultLeague: espn.LeagueNFL, Category: "passing", StatName: "passingTouchdowns", Label: "TD", Sort: "passing.passingTouchdowns:desc", DisplayName: "Passing Touchdowns"},
	{Aliases: []string{"rushing yards"}, DefaultLeague: espn.LeagueNFL, Category: "rushing", StatName: "rushingYards", Label: "YDS", Sort: "rushing.rushingYards:desc", DisplayName: "Rushing Yards"},
	{Aliases: []string{"receiving yards"}, DefaultLeague: espn.LeagueNFL, Category: "receiving", StatName: "receivingYards", Label: "YDS", Sort: "receiving.receivingYards:desc", DisplayName: "Receiving Yards"},
	{Aliases: []string{"points per game", "ppg", "points"}, DefaultLeague: espn.LeagueNBA, Category: "offensive", StatName: "avgPoints", Label: "PTS", Sort: "offensive.avgPoints:desc", DisplayName: "Points Per Game"},
	{Aliases: []string{"rebounds", "rebounds per game", "rpg"}, DefaultLeague: espn.LeagueNBA, Category: "general", StatName: "avgRebounds", Label: "REB", Sort: "general.avgRebounds:desc", DisplayName: "Rebounds Per Game"},
	{Aliases: []string{"assists", "assists per game", "apg"}, DefaultLeague: espn.LeagueNBA, Category: "offensive", StatName: "avgAssists", Label: "AST", Sort: "offensive.avgAssists:desc", DisplayName: "Assists Per Game"},
	{Aliases: []string{"steals", "steals per game"}, DefaultLeague: espn.LeagueNBA, Category: "defensive", StatName: "avgSteals", Label: "STL", Sort: "defensive.avgSteals:desc", DisplayName: "Steals Per Game"},
	{Aliases: []string{"blocks", "blocks per game"}, DefaultLeague: espn.LeagueNBA, Category: "defensive", StatName: "avgBlocks", Label: "BLK", Sort: "defensive.avgBlocks:desc", DisplayName: "Blocks Per Game"},
	{Aliases: []string{"goals"}, DefaultLeague: espn.LeagueNHL, Category: "scoring", StatName: "goals", Label: "G", Sort: "scoring.goals:desc", DisplayName: "Goals"},
	{Aliases: []string{"hockey assists", "assists"}, DefaultLeague: espn.LeagueNHL, Category: "scoring", StatName: "assists", Label: "A", Sort: "scoring.assists:desc", DisplayName: "Assists"},
	{Aliases: []string{"hockey points", "points"}, DefaultLeague: espn.LeagueNHL, Category: "scoring", StatName: "points", Label: "PTS", Sort: "scoring.points:desc", DisplayName: "Points"},
}

func DetectSportsIntent(query string, now time.Time) (*SportsRequest, bool) {
	raw := strings.TrimSpace(query)
	if raw == "" {
		return nil, false
	}

	norm := normalizeText(raw)
	if isNonLookupQuery(norm) {
		return nil, false
	}

	cfg, hasLeague := detectLeague(norm)
	teamQuery := ""
	if team, ok := detectTeamAlias(norm); ok {
		teamQuery = team.TeamQuery
		if !hasLeague {
			if teamCfg, ok := leagueConfigByLeague(team.League); ok {
				cfg = teamCfg
				hasLeague = true
			}
		}
	}

	intent := detectIntent(norm)
	season := parseSeasonFromQuery(raw)
	limit := parseLimitFromQuery(norm, defaultLimitForIntent(intent))
	if intent == SportsIntentSchedule && teamQuery != "" && !hasTemporalPhrase(norm) {
		intent = SportsIntentTeamSchedule
	}
	if intent == SportsIntentTeamRecord && teamQuery == "" {
		return nil, false
	}
	if metric, ok := detectStatMetric(norm, cfg, hasLeague); ok && isLeaderQuery(norm) {
		intent = SportsIntentLeaders
		if !hasLeague && metric.DefaultLeague != "" {
			if metricCfg, ok := leagueConfigByLeague(metric.DefaultLeague); ok {
				cfg = metricCfg
				hasLeague = true
			}
		}
		if limit == defaultLimitForIntent(SportsIntentUnknown) || limit == 0 {
			limit = 50
		}
		return sportsRequestWithStat(raw, cfg, hasLeague, teamQuery, "", intent, metric, season, limit), hasLeague
	}
	if intent == SportsIntentAthleteStats || intent == SportsIntentAthleteNews {
		athleteQuery := extractAthleteQuery(raw, norm, cfg, hasLeague, intent)
		if intent == SportsIntentAthleteStats && hasLeague &&
			(athleteQuery == "" || isTeamStatsQuery(norm, athleteQuery, teamQuery)) {
			date, dateLabel, _ := parseDateFromQuery(raw, norm, now, SportsIntentLeagueStats)
			return &SportsRequest{
				RawQuery:  raw,
				Intent:    SportsIntentLeagueStats,
				League:    cfg.League,
				Sport:     cfg.Sport,
				TeamQuery: teamQuery,
				Date:      date,
				DateLabel: dateLabel,
				Season:    season,
				Limit:     limit,
			}, true
		}
		if athleteQuery == "" {
			return nil, false
		}
		req := sportsRequestWithStat(raw, cfg, hasLeague, teamQuery, athleteQuery, intent, statMetricConfig{}, season, limit)
		return req, true
	}
	if intent == SportsIntentUnknown {
		if !hasLeague {
			return nil, false
		}
		if hasTemporalPhrase(norm) {
			intent = SportsIntentScores
			if hasAnyPhrase(norm, "playing", "who plays", "games", "matchup", "matchups") {
				intent = SportsIntentSchedule
			}
		} else {
			return nil, false
		}
	}

	if !hasLeague && intent != SportsIntentNews && intent != SportsIntentOdds {
		return nil, false
	}
	if !hasLeague && intent == SportsIntentNews && !hasBroadSportsNewsPhrase(norm) {
		return nil, false
	}
	if !hasLeague && intent == SportsIntentOdds && !hasBroadSportsOddsPhrase(norm) {
		return nil, false
	}

	date, dateLabel, _ := parseDateFromQuery(raw, norm, now, intent)
	return &SportsRequest{
		RawQuery:  raw,
		Intent:    intent,
		League:    cfg.League,
		Sport:     cfg.Sport,
		TeamQuery: teamQuery,
		Date:      date,
		DateLabel: dateLabel,
		Season:    season,
		Limit:     limit,
	}, true
}

func ParseDateValue(value string, now time.Time, intent SportsIntentType) (*time.Time, string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, "", nil
	}
	norm := normalizeText(raw)
	date, label, ok := parseDateFromQuery(raw, norm, now, intent)
	if ok {
		return date, label, nil
	}

	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		return datePtr(t), t.Format("Jan 2, 2006"), nil
	}
	if t, err := time.ParseInLocation("1/2/2006", raw, loc); err == nil {
		return datePtr(t), t.Format("Jan 2, 2006"), nil
	}
	return nil, "", ErrMalformedDate
}

func ValidateDateInQuery(query string, now time.Time) error {
	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	if match := exactDatePattern.FindString(query); match != "" {
		if _, err := time.ParseInLocation("2006-01-02", match, loc); err != nil {
			return ErrMalformedDate
		}
	}
	if match := slashDatePattern.FindString(query); match != "" {
		if _, err := time.ParseInLocation("1/2/2006", match, loc); err != nil {
			return ErrMalformedDate
		}
	}
	return nil
}

func normalizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func hasPhrase(norm, phrase string) bool {
	needle := normalizeText(phrase)
	if needle == "" {
		return false
	}
	return strings.Contains(" "+norm+" ", " "+needle+" ")
}

func hasAnyPhrase(norm string, phrases ...string) bool {
	for _, phrase := range phrases {
		if hasPhrase(norm, phrase) {
			return true
		}
	}
	return false
}

func isNonLookupQuery(norm string) bool {
	if hasAnyPhrase(norm,
		"write a story", "short story", "make a sports logo", "sports logo",
		"write a sports news article", "write a news article", "draft a sports news article",
	) {
		return true
	}
	if hasAnyPhrase(norm, "explain betting odds", "explain how betting odds work", "how betting odds work", "how do betting odds work") {
		return true
	}
	if hasPhrase(norm, "explain how") && hasPhrase(norm, "standings work") {
		return true
	}
	if hasPhrase(norm, "how standings work") || hasPhrase(norm, "how mlb standings work") {
		return true
	}
	return false
}

func detectLeague(norm string) (LeagueConfig, bool) {
	var best LeagueConfig
	bestLen := 0
	for _, cfg := range leagueConfigs {
		for _, alias := range cfg.Aliases {
			if hasPhrase(norm, alias) {
				if l := len(normalizeText(alias)); l > bestLen {
					best = cfg
					bestLen = l
				}
			}
		}
	}
	if bestLen > 0 {
		return best, true
	}
	return LeagueConfig{}, false
}

func detectTeamAlias(norm string) (teamAlias, bool) {
	for _, team := range teamAliases {
		for _, alias := range team.Aliases {
			if hasPhrase(norm, alias) {
				return team, true
			}
		}
	}
	return teamAlias{}, false
}

func detectIntent(norm string) SportsIntentType {
	if hasAnyPhrase(norm, "roster", "depth chart", "who is on", "who plays for") {
		return SportsIntentRoster
	}
	if hasAnyPhrase(norm, "injury", "injuries", "injured", "injury report") {
		return SportsIntentInjuries
	}
	if hasAnyPhrase(norm, "transaction", "transactions", "traded", "signed", "waived") {
		return SportsIntentTransactions
	}
	if hasAnyPhrase(norm, "team record", "record") && !hasAnyPhrase(norm, "record holder", "records", "all time") {
		return SportsIntentTeamRecord
	}
	if hasAnyPhrase(norm, "team schedule", "full schedule", "season schedule") {
		return SportsIntentTeamSchedule
	}
	if hasAnyPhrase(norm, "player news", "athlete news") {
		return SportsIntentAthleteNews
	}
	if hasOddsIntent(norm) {
		return SportsIntentOdds
	}
	if isLeaderQuery(norm) {
		return SportsIntentLeaders
	}
	if hasPlayerStatMetric(norm) {
		return SportsIntentAthleteStats
	}
	if hasAnyPhrase(norm, "player stats", "player statistics", "athlete stats", "athlete statistics", "stats", "statistics", "game log", "gamelog", "splits", "bio") &&
		!hasAnyPhrase(norm, "team stats", "team statistics", "league stats", "league statistics") {
		return SportsIntentAthleteStats
	}
	if hasAnyPhrase(norm, "team stats", "team statistics", "league stats", "league statistics") {
		return SportsIntentLeagueStats
	}
	if hasAnyPhrase(norm, "top 25", "poll", "polls", "ap poll", "coaches poll") ||
		(hasPhrase(norm, "rankings") && hasAnyPhrase(norm,
			"college football", "ncaaf", "cfb", "college basketball", "ncaamb", "ncaawb",
		)) {
		return SportsIntentRankings
	}
	if hasAnyPhrase(norm,
		"standings", "conference standings", "division", "rank", "rankings",
		"wild card", "wildcard",
	) || hasStandingsTableIntent(norm) {
		return SportsIntentStandings
	}
	if hasAnyPhrase(norm,
		"score", "scores", "final", "result", "results", "how did", "who won", "live score",
	) {
		return SportsIntentScores
	}
	if hasAnyPhrase(norm,
		"schedule", "games", "playing", "who plays", "matchup", "matchups",
		"on today", "tonight", "tomorrow",
	) {
		return SportsIntentSchedule
	}
	if hasAnyPhrase(norm,
		"news", "headlines", "latest", "latest on", "what is new", "what s new",
	) {
		return SportsIntentNews
	}
	return SportsIntentUnknown
}

func hasTemporalPhrase(norm string) bool {
	return hasAnyPhrase(norm, "today", "tonight", "current", "yesterday", "tomorrow", "this week") ||
		exactDatePattern.MatchString(norm) ||
		slashDatePattern.MatchString(norm)
}

func hasStandingsTableIntent(norm string) bool {
	if !hasPhrase(norm, "table") {
		return false
	}
	return hasAnyPhrase(norm,
		"league table",
		"premier league table",
		"english premier league table",
		"epl table",
		"soccer table",
	) || hasAnyPhrase(norm,
		"premier league",
		"english premier league",
		"epl",
	)
}

func hasBroadSportsNewsPhrase(norm string) bool {
	return hasPhrase(norm, "sports") &&
		hasAnyPhrase(norm,
			"news", "headlines", "sports news", "sports headlines",
			"latest in sports", "what is new in sports", "what s new in sports",
		)
}

func hasBroadSportsOddsPhrase(norm string) bool {
	return hasAnyPhrase(norm,
		"sports betting odds", "sports odds", "current betting odds",
		"latest betting odds", "today betting odds", "today s betting odds", "todays betting odds",
		"current betting lines", "latest betting lines", "sports betting lines",
	)
}

func hasOddsIntent(norm string) bool {
	return hasAnyPhrase(norm,
		"odds", "betting odds", "sportsbook odds", "betting lines", "game lines",
		"spread", "spreads", "point spread", "moneyline", "money line",
		"over under", "overunder", "over odds", "under odds",
		"who is favored", "who s favored", "who is the favorite", "point total",
	)
}

func defaultLimitForIntent(intent SportsIntentType) int {
	if intent == SportsIntentNews {
		return 10
	}
	if intent == SportsIntentLeaders {
		return 50
	}
	if intent == SportsIntentOdds {
		return 50
	}
	return 100
}

func isUnsupportedSportsStatQuery(norm string) bool {
	return false
}

func isTeamStatsQuery(norm, athleteQuery, teamQuery string) bool {
	if teamQuery == "" {
		return false
	}
	if hasAnyPhrase(norm, "team stats", "team statistics") {
		return true
	}
	team, ok := detectTeamAlias(normalizeText(athleteQuery))
	return ok && strings.EqualFold(team.TeamQuery, teamQuery)
}

func hasPlayerStatMetric(norm string) bool {
	return hasAnyPhrase(norm,
		"hr", "home run", "home runs", "homer", "homers",
		"rbi", "rbis", "runs batted in", "batting average", "avg", "hits", "stolen bases",
		"era", "earned run average", "strikeout", "strikeouts", "saves", "whip",
		"passing yards", "rushing yards", "receiving yards", "touchdown", "touchdowns", "td", "tds",
		"points per game", "ppg", "rebounds", "assists", "steals", "blocks",
		"goals", "goal scorers", "clean sheets", "saves percentage",
	)
}

func isLeaderQuery(norm string) bool {
	return hasAnyPhrase(norm,
		"leader", "leaders", "leaderboard", "stat leaders", "league leaders",
		"top", "top players", "most", "who leads", "league leader",
	)
}

func detectStatMetric(norm string, cfg LeagueConfig, hasLeague bool) (statMetricConfig, bool) {
	var fallback statMetricConfig
	for _, metric := range statMetricConfigs {
		for _, alias := range metric.Aliases {
			if !hasPhrase(norm, alias) {
				continue
			}
			if hasLeague && metric.DefaultLeague == cfg.League {
				return metric, true
			}
			if hasLeague {
				continue
			}
			if fallback.DisplayName == "" {
				fallback = metric
			}
		}
	}
	if fallback.DisplayName != "" {
		return fallback, true
	}
	return statMetricConfig{}, false
}

func sportsRequestWithStat(raw string, cfg LeagueConfig, hasLeague bool, teamQuery, athleteQuery string, intent SportsIntentType, metric statMetricConfig, season, limit int) *SportsRequest {
	req := &SportsRequest{
		RawQuery:     raw,
		Intent:       intent,
		TeamQuery:    teamQuery,
		AthleteQuery: athleteQuery,
		StatCategory: metric.Category,
		StatName:     metric.StatName,
		StatLabel:    metric.Label,
		StatSort:     metric.Sort,
		Season:       season,
		Limit:        limit,
	}
	if hasLeague {
		req.League = cfg.League
		req.Sport = cfg.Sport
	}
	return req
}

func parseSeasonFromQuery(raw string) int {
	for _, match := range seasonPattern.FindAllString(raw, -1) {
		if len(match) == 4 {
			if strings.Contains(raw, match+"-") || strings.Contains(raw, match+"/") {
				continue
			}
			var season int
			for _, r := range match {
				season = season*10 + int(r-'0')
			}
			return season
		}
	}
	return 0
}

func parseLimitFromQuery(norm string, fallback int) int {
	if fallback <= 0 {
		fallback = defaultLimitForIntent(SportsIntentUnknown)
	}
	match := topLimitPattern.FindStringSubmatch(norm)
	if len(match) == 2 {
		n := 0
		for _, r := range match[1] {
			n = n*10 + int(r-'0')
		}
		if n > 0 {
			if n > 100 {
				return 100
			}
			return n
		}
	}
	return fallback
}

func extractAthleteQuery(raw, norm string, cfg LeagueConfig, hasLeague bool, intent SportsIntentType) string {
	cleaned := normalizeText(raw)
	for _, phrase := range []string{
		"show me", "what are", "what is", "whats", "what s", "give me", "print out", "table",
		"player stats", "player statistics", "athlete stats", "athlete statistics",
		"stats", "statistics", "stat", "game log", "gamelog", "splits", "bio", "news",
		"for", "in", "during", "season", "regular season",
	} {
		cleaned = removePhrase(cleaned, phrase)
	}
	if hasLeague {
		for _, alias := range cfg.Aliases {
			cleaned = removePhrase(cleaned, alias)
		}
		cleaned = removePhrase(cleaned, cfg.League)
	}
	for _, metric := range statMetricConfigs {
		for _, alias := range metric.Aliases {
			cleaned = removePhrase(cleaned, alias)
		}
	}
	cleaned = seasonPattern.ReplaceAllString(cleaned, " ")
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" && intent == SportsIntentAthleteNews {
		return strings.TrimSpace(raw)
	}
	return cleaned
}

func removePhrase(norm, phrase string) string {
	needle := normalizeText(phrase)
	if needle == "" {
		return norm
	}
	return strings.TrimSpace(strings.ReplaceAll(" "+norm+" ", " "+needle+" ", " "))
}

func parseDateFromQuery(raw, norm string, now time.Time, intent SportsIntentType) (*time.Time, string, bool) {
	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	today := dateOnly(now.In(loc))

	if hasPhrase(norm, "yesterday") {
		t := today.AddDate(0, 0, -1)
		return &t, "Yesterday", true
	}
	if hasPhrase(norm, "tomorrow") {
		t := today.AddDate(0, 0, 1)
		return &t, "Tomorrow", true
	}
	if hasPhrase(norm, "today") {
		return &today, "Today", true
	}
	if hasPhrase(norm, "tonight") {
		return &today, "Tonight", true
	}
	if hasPhrase(norm, "current") {
		if intent == SportsIntentStandings {
			return nil, "Current", true
		}
		return &today, "Today", true
	}

	if match := exactDatePattern.FindString(raw); match != "" {
		if t, err := time.ParseInLocation("2006-01-02", match, loc); err == nil {
			return &t, t.Format("Jan 2, 2006"), true
		}
	}
	if match := slashDatePattern.FindString(raw); match != "" {
		if t, err := time.ParseInLocation("1/2/2006", match, loc); err == nil {
			return &t, t.Format("Jan 2, 2006"), true
		}
	}
	return nil, "", false
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func datePtr(t time.Time) *time.Time {
	return &t
}

func leagueConfigForRequest(req SportsRequest) (LeagueConfig, bool) {
	if req.League != "" {
		if cfg, ok := leagueConfigByLeague(req.League); ok {
			return cfg, true
		}
		if cfg, ok := leagueConfigByAlias(req.League); ok {
			return cfg, true
		}
	}
	if req.Sport != "" {
		for _, cfg := range leagueConfigs {
			if cfg.Sport == req.Sport && (req.League == "" || cfg.League == req.League) {
				return cfg, true
			}
		}
	}
	return LeagueConfig{}, false
}

func leagueConfigByLeague(league string) (LeagueConfig, bool) {
	for _, cfg := range leagueConfigs {
		if strings.EqualFold(cfg.League, strings.TrimSpace(league)) {
			return cfg, true
		}
	}
	return LeagueConfig{}, false
}

func leagueConfigByAlias(alias string) (LeagueConfig, bool) {
	norm := normalizeText(alias)
	for _, cfg := range leagueConfigs {
		if hasPhrase(norm, cfg.League) {
			return cfg, true
		}
		for _, a := range cfg.Aliases {
			if normalizeText(a) == norm {
				return cfg, true
			}
		}
	}
	return LeagueConfig{}, false
}
