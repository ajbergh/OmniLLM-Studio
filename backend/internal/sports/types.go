package sports

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const SourceESPN = "ESPN public API via espn-go"

var (
	ErrUnsupportedLeague = errors.New("unsupported sports league")
	ErrMalformedDate     = errors.New("malformed sports date")
	ErrNoGames           = errors.New("espn returned no games")
	ErrNoMatchingGames   = errors.New("espn returned no games matching team")
	ErrNoStandings       = errors.New("espn returned no standings")
	ErrNoNews            = errors.New("espn returned no sports news")
	ErrNoSportsData      = errors.New("espn returned no sports data")
	ErrTeamNotFound      = errors.New("espn team not found")
	ErrAthleteNotFound   = errors.New("espn athlete not found")
	ErrRateLimited       = errors.New("espn rate limited sports lookup")
)

type SportsIntentType string

const (
	SportsIntentUnknown      SportsIntentType = "unknown"
	SportsIntentScores       SportsIntentType = "scores"
	SportsIntentSchedule     SportsIntentType = "schedule"
	SportsIntentStandings    SportsIntentType = "standings"
	SportsIntentNews         SportsIntentType = "news"
	SportsIntentRoster       SportsIntentType = "roster"
	SportsIntentInjuries     SportsIntentType = "injuries"
	SportsIntentTransactions SportsIntentType = "transactions"
	SportsIntentTeamRecord   SportsIntentType = "team_record"
	SportsIntentTeamSchedule SportsIntentType = "team_schedule"
	SportsIntentLeaders      SportsIntentType = "leaders"
	SportsIntentAthleteStats SportsIntentType = "athlete_stats"
	SportsIntentAthleteNews  SportsIntentType = "athlete_news"
	SportsIntentRankings     SportsIntentType = "rankings"
	SportsIntentLeagueStats  SportsIntentType = "league_stats"
)

type SportsRequest struct {
	RawQuery     string
	Intent       SportsIntentType
	League       string
	Sport        string
	TeamQuery    string
	AthleteQuery string
	StatCategory string
	StatName     string
	StatLabel    string
	StatSort     string
	Date         *time.Time
	DateLabel    string
	Season       int
	Limit        int
}

type LeagueConfig struct {
	DisplayName string
	Sport       string
	League      string
	Aliases     []string
}

type SportsLookupResult struct {
	Intent      SportsIntentType
	League      string
	LeagueName  string
	Sport       string
	DateLabel   string
	Markdown    string
	Source      string
	RetrievedAt time.Time
}

type GameRow struct {
	Date       string
	Time       string
	Status     string
	AwayTeam   string
	AwayAbbr   string
	AwayScore  string
	HomeTeam   string
	HomeAbbr   string
	HomeScore  string
	Venue      string
	Broadcasts string
}

type StandingsRow struct {
	Group            string
	Rank             int
	Team             string
	Abbr             string
	Wins             string
	Losses           string
	Ties             string
	Draws            string
	Pct              string
	GamesBack        string
	Streak           string
	LastTen          string
	Points           string
	GamesPlayed      string
	GoalDifferential string
	Note             string
}

type NewsRow struct {
	Published   string
	Headline    string
	Description string
	Byline      string
	URL         string
}

type RosterRow struct {
	Group    string
	Name     string
	Position string
	Jersey   string
	Age      string
	Height   string
	Weight   string
	Status   string
}

type LeaderboardRow struct {
	Rank     int
	Athlete  string
	Team     string
	Position string
	Value    string
}

type SearchEntity struct {
	ID     string
	Name   string
	League string
	Sport  string
	Team   string
	URL    string
}

func UserFacingError(req SportsRequest, err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrUnsupportedLeague) {
		return "I can retrieve ESPN-backed scores, schedules, standings, news, rosters, injuries, transactions, team records, rankings, player stats, league stats, and league leaders for supported leagues."
	}
	if errors.Is(err, ErrMalformedDate) {
		return "I could not understand that sports date. Please use today, tomorrow, yesterday, or YYYY-MM-DD."
	}
	if errors.Is(err, ErrNoGames) || errors.Is(err, ErrNoMatchingGames) {
		if req.League != "" {
			return fmt.Sprintf("I found the league, but ESPN did not return games for that date.")
		}
		return "ESPN did not return games for that date."
	}
	if errors.Is(err, ErrNoStandings) {
		name := req.League
		if cfg, ok := leagueConfigForRequest(req); ok {
			name = cfg.DisplayName
		}
		if name == "" {
			name = "sports"
		}
		return fmt.Sprintf("I could not retrieve %s standings from ESPN right now.", name)
	}
	if errors.Is(err, ErrNoNews) {
		name := req.League
		if cfg, ok := leagueConfigForRequest(req); ok {
			name = cfg.DisplayName
		}
		if req.TeamQuery != "" {
			name = req.TeamQuery
		}
		if name == "" {
			name = "sports"
		}
		return fmt.Sprintf("I could not retrieve %s news from ESPN right now.", name)
	}
	if errors.Is(err, ErrNoSportsData) {
		name := req.League
		if cfg, ok := leagueConfigForRequest(req); ok {
			name = cfg.DisplayName
		}
		if req.TeamQuery != "" {
			name = req.TeamQuery
		}
		if req.AthleteQuery != "" {
			name = req.AthleteQuery
		}
		if name == "" {
			name = "sports"
		}
		return fmt.Sprintf("I could not retrieve %s data from ESPN right now.", name)
	}
	if errors.Is(err, ErrTeamNotFound) {
		return fmt.Sprintf("I found the league, but ESPN did not find a team matching %q.", req.TeamQuery)
	}
	if errors.Is(err, ErrAthleteNotFound) {
		return fmt.Sprintf("ESPN did not find a player matching %q.", req.AthleteQuery)
	}
	if errors.Is(err, ErrRateLimited) {
		return "ESPN is rate limiting sports lookups right now. Please try again shortly."
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "The sports lookup was cancelled before ESPN returned data."
	}

	name := req.League
	if cfg, ok := leagueConfigForRequest(req); ok {
		name = cfg.DisplayName
	}
	if name == "" {
		name = "sports"
	}
	if req.Intent == SportsIntentStandings {
		return fmt.Sprintf("I could not retrieve %s standings from ESPN right now.", name)
	}
	if req.Intent == SportsIntentNews {
		return fmt.Sprintf("I could not retrieve %s news from ESPN right now.", name)
	}
	return fmt.Sprintf("I could not retrieve %s sports data from ESPN right now.", name)
}
