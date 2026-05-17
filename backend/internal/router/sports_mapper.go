package router

import (
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sports"
	espn "github.com/chinmaykhachane/espn-go"
)

func SportsRequestFromDecision(raw string, decision RouterDecision, now time.Time) (*sports.SportsRequest, error) {
	if decision.Route != RouteSportsLookup || decision.Sports == nil {
		return nil, ErrMissingSportsParam
	}
	params := decision.Sports
	intent, ok := sportsIntent(params.Intent)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported sports intent %q", ErrUnsupportedRoute, params.Intent)
	}
	league, sport, ok := sportsLeague(params.League, params.Sport)
	if !ok && requiresLeague(intent) {
		return nil, fmt.Errorf("%w: unsupported sports league %q", ErrUnsupportedRoute, params.League)
	}
	req := &sports.SportsRequest{
		RawQuery:           raw,
		Intent:             intent,
		League:             league,
		Sport:              sport,
		TeamQuery:          strings.TrimSpace(params.TeamQuery),
		AthleteQuery:       strings.TrimSpace(params.AthleteQuery),
		SecondAthleteQuery: strings.TrimSpace(params.SecondAthleteQuery),
		GameDetailSubtype:  strings.TrimSpace(params.GameDetailSubtype),
		DateLabel:          strings.TrimSpace(params.DateLabel),
		RenderMode:         sports.DefaultSportsRenderMode,
	}
	if req.GameDetailSubtype == "" && wantsPitchingMatchups(raw) && (intent == sports.SportsIntentSchedule || intent == sports.SportsIntentScores) {
		req.GameDetailSubtype = "pitching_matchups"
	}
	if params.Season != nil {
		req.Season = *params.Season
	}
	if params.Limit != nil {
		req.Limit = clamp(*params.Limit, 1, 50)
	}
	if req.Limit == 0 {
		req.Limit = 25
	}
	if metric := strings.TrimSpace(params.Metric); metric != "" {
		applyMetric(req, metric)
	}
	if strings.TrimSpace(params.Date) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(params.Date))
		if err != nil {
			return nil, sports.ErrMalformedDate
		}
		req.Date = &parsed
		if req.DateLabel == "" {
			req.DateLabel = parsed.Format("2006-01-02")
		}
	}
	if req.DateLabel == "" && req.Date != nil {
		req.DateLabel = req.Date.Format("2006-01-02")
	}
	if req.Date == nil {
		switch strings.ToLower(req.DateLabel) {
		case "today":
			d := dateOnly(now)
			req.Date = &d
		case "tomorrow":
			d := dateOnly(now.AddDate(0, 0, 1))
			req.Date = &d
		case "yesterday":
			d := dateOnly(now.AddDate(0, 0, -1))
			req.Date = &d
		}
	}
	return req, nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func sportsIntent(intent string) (sports.SportsIntentType, bool) {
	key := strings.ToLower(strings.TrimSpace(intent))
	key = strings.ReplaceAll(key, "-", "_")
	known := map[string]sports.SportsIntentType{
		"scores": sports.SportsIntentScores, "score": sports.SportsIntentScores, "schedule": sports.SportsIntentSchedule,
		"standings": sports.SportsIntentStandings, "news": sports.SportsIntentNews, "roster": sports.SportsIntentRoster,
		"injuries": sports.SportsIntentInjuries, "transactions": sports.SportsIntentTransactions, "team_record": sports.SportsIntentTeamRecord,
		"team_schedule": sports.SportsIntentTeamSchedule, "leaders": sports.SportsIntentLeaders, "leader": sports.SportsIntentLeaders,
		"athlete_stats": sports.SportsIntentAthleteStats, "player_stats": sports.SportsIntentAthleteStats,
		"athlete_news": sports.SportsIntentAthleteNews, "rankings": sports.SportsIntentRankings, "league_stats": sports.SportsIntentLeagueStats,
		"odds": sports.SportsIntentOdds, "teams": sports.SportsIntentTeams, "team_history": sports.SportsIntentTeamHistory,
		"seasons": sports.SportsIntentSeasons, "calendar": sports.SportsIntentCalendar, "scoreboard_header": sports.SportsIntentScoreboardHeader,
		"search": sports.SportsIntentSearch, "qbr": sports.SportsIntentQBR, "athlete_comparison": sports.SportsIntentAthleteComparison,
		"athlete_awards": sports.SportsIntentAthleteAwards, "athlete_seasons": sports.SportsIntentAthleteSeasons,
		"athlete_records": sports.SportsIntentAthleteRecords, "athlete_injuries": sports.SportsIntentAthleteInjuries,
		"hot_zones": sports.SportsIntentHotZones, "game_detail": sports.SportsIntentGameDetail, "champions": sports.SportsIntentChampions,
		"draft": sports.SportsIntentDraft, "coaches": sports.SportsIntentCoaches, "venues": sports.SportsIntentVenues,
		"power_index": sports.SportsIntentPowerIndex, "recruits": sports.SportsIntentRecruits, "bracketology": sports.SportsIntentBracketology,
		"tournaments": sports.SportsIntentTournaments, "fantasy": sports.SportsIntentFantasy,
	}
	intentType, ok := known[key]
	return intentType, ok
}

func sportsLeague(league, sport string) (string, string, bool) {
	key := strings.ToLower(strings.TrimSpace(league))
	key = strings.ReplaceAll(key, " ", "")
	key = strings.ReplaceAll(key, "-", "")
	known := map[string][2]string{
		"mlb": {espn.LeagueMLB, espn.SportBaseball}, "baseball": {espn.LeagueMLB, espn.SportBaseball},
		"nfl": {espn.LeagueNFL, espn.SportFootball}, "nba": {espn.LeagueNBA, espn.SportBasketball}, "wnba": {espn.LeagueWNBA, espn.SportBasketball},
		"nhl": {espn.LeagueNHL, espn.SportHockey}, "ncaaf": {espn.LeagueCollegeFootball, espn.SportFootball}, "cfb": {espn.LeagueCollegeFootball, espn.SportFootball},
		"ncaamb": {espn.LeagueMensCollegeBball, espn.SportBasketball}, "menscollegebasketball": {espn.LeagueMensCollegeBball, espn.SportBasketball},
		"ncaawb": {espn.LeagueWomensCollegeBall, espn.SportBasketball}, "womenscollegebasketball": {espn.LeagueWomensCollegeBall, espn.SportBasketball},
		"epl": {espn.LeagueEPL, espn.SportSoccer}, "premierleague": {espn.LeagueEPL, espn.SportSoccer}, "mls": {espn.LeagueMLS, espn.SportSoccer},
		"ucl": {espn.LeagueChampionsLg, espn.SportSoccer}, "championsleague": {espn.LeagueChampionsLg, espn.SportSoccer},
		"laliga": {espn.LeagueLaLiga, espn.SportSoccer}, "bundesliga": {espn.LeagueBundesliga, espn.SportSoccer},
		"seriea": {espn.LeagueSerieA, espn.SportSoccer}, "ligue1": {espn.LeagueLigue1, espn.SportSoccer}, "ipl": {sports.LeagueIPL, espn.SportCricket},
		"f1": {espn.LeagueF1, espn.SportRacing}, "formula1": {espn.LeagueF1, espn.SportRacing}, "nascar": {espn.LeagueNASCARCup, espn.SportRacing},
		"pga": {espn.LeaguePGA, espn.SportGolf}, "atp": {espn.LeagueATP, espn.SportTennis},
	}
	if pair, ok := known[key]; ok {
		return pair[0], pair[1], true
	}
	if strings.TrimSpace(league) != "" && strings.TrimSpace(sport) != "" {
		return strings.TrimSpace(league), strings.TrimSpace(sport), true
	}
	return "", strings.TrimSpace(sport), false
}

func requiresLeague(intent sports.SportsIntentType) bool {
	return intent != sports.SportsIntentNews && intent != sports.SportsIntentOdds
}

func applyMetric(req *sports.SportsRequest, metric string) {
	key := strings.ToLower(strings.TrimSpace(metric))
	key = strings.ReplaceAll(key, "-", " ")
	metrics := map[string]struct{ category, name, label, sort string }{
		"home runs": {"batting", "homeRuns", "HR", "batting.homeRuns:desc"}, "hr": {"batting", "homeRuns", "HR", "batting.homeRuns:desc"},
		"rbi": {"batting", "RBIs", "RBI", "batting.RBIs:desc"}, "rbis": {"batting", "RBIs", "RBI", "batting.RBIs:desc"},
		"goals": {"scoring", "goals", "G", "scoring.goals:desc"}, "assists": {"scoring", "assists", "A", "scoring.assists:desc"},
		"points": {"scoring", "points", "PTS", "scoring.points:desc"}, "saves": {"pitching", "saves", "SV", "pitching.saves:desc"},
		"passing yards":   {"passing", "passingYards", "YDS", "passing.passingYards:desc"},
		"rushing yards":   {"rushing", "rushingYards", "YDS", "rushing.rushingYards:desc"},
		"receiving yards": {"receiving", "receivingYards", "YDS", "receiving.receivingYards:desc"},
		"rebounds":        {"general", "avgRebounds", "REB", "general.avgRebounds:desc"},
		"blocks":          {"defensive", "avgBlocks", "BLK", "defensive.avgBlocks:desc"},
		"steals":          {"defensive", "avgSteals", "STL", "defensive.avgSteals:desc"},
	}
	if m, ok := metrics[key]; ok {
		req.StatCategory = m.category
		req.StatName = m.name
		req.StatLabel = m.label
		req.StatSort = m.sort
	}
}

func clamp(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func wantsPitchingMatchups(raw string) bool {
	norm := strings.ToLower(strings.TrimSpace(raw))
	norm = strings.ReplaceAll(norm, "'", "")
	return strings.Contains(norm, "pitching matchup") ||
		strings.Contains(norm, "pitching matchups") ||
		strings.Contains(norm, "probable pitcher") ||
		strings.Contains(norm, "probable pitchers") ||
		strings.Contains(norm, "probable starter") ||
		strings.Contains(norm, "probable starters") ||
		strings.Contains(norm, "starting pitcher") ||
		strings.Contains(norm, "starting pitchers")
}
