package sports

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const SourceESPN = "ESPN public API via espn-go"

// LeagueIPL is ESPN's cricket series identifier for the Indian Premier League.
// espn-go exposes SportCricket, but does not currently define league constants
// for cricket series pages such as /cricket/standings/series/8048/ipl.
const LeagueIPL = "8048"

var (
	ErrUnsupportedLeague = errors.New("unsupported sports league")
	ErrMalformedDate     = errors.New("malformed sports date")
	ErrNoGames           = errors.New("espn returned no games")
	ErrNoMatchingGames   = errors.New("espn returned no games matching team")
	ErrNoStandings       = errors.New("espn returned no standings")
	ErrNoNews            = errors.New("espn returned no sports news")
	ErrNoOdds            = errors.New("espn returned no betting odds")
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
	SportsIntentOdds         SportsIntentType = "odds"
	SportsIntentTeams        SportsIntentType = "teams"
	SportsIntentTeamHistory  SportsIntentType = "team_history"
	SportsIntentSeasons      SportsIntentType = "seasons"
	SportsIntentCalendar     SportsIntentType = "calendar"

	// Extended capabilities (Q10, Q46, Q52, Q53, Q58, Q62, Q63, Q68–Q76)
	SportsIntentScoreboardHeader  SportsIntentType = "scoreboard_header"
	SportsIntentSearch            SportsIntentType = "search"
	SportsIntentQBR               SportsIntentType = "qbr"
	SportsIntentAthleteComparison SportsIntentType = "athlete_comparison"
	SportsIntentAthleteAwards     SportsIntentType = "athlete_awards"
	SportsIntentAthleteSeasons    SportsIntentType = "athlete_seasons"
	SportsIntentAthleteRecords    SportsIntentType = "athlete_records"
	SportsIntentAthleteInjuries   SportsIntentType = "athlete_injuries"
	SportsIntentHotZones          SportsIntentType = "hot_zones"
	SportsIntentGameDetail        SportsIntentType = "game_detail"
	SportsIntentChampions         SportsIntentType = "champions"
	SportsIntentDraft             SportsIntentType = "draft"
	SportsIntentCoaches           SportsIntentType = "coaches"

	// Q77–Q87 and Q94–Q99
	SportsIntentVenues       SportsIntentType = "venues"       // stadium / arena lookup
	SportsIntentPowerIndex   SportsIntentType = "power_index"  // FPI / BPI / SP+ power index
	SportsIntentRecruits     SportsIntentType = "recruits"     // CFB recruit rankings
	SportsIntentBracketology SportsIntentType = "bracketology" // NCAA bracket projections
	SportsIntentTournaments  SportsIntentType = "tournaments"  // golf / tennis tournament lists
	SportsIntentFantasy      SportsIntentType = "fantasy"      // ESPN fantasy league/player info
)

type SportsRenderMode string

const (
	SportsRenderPlainMarkdown    SportsRenderMode = "plain_markdown"
	SportsRenderEnhancedMarkdown SportsRenderMode = "enhanced_markdown"
	SportsRenderHTMLMarkdown     SportsRenderMode = "html_markdown"
)

const DefaultSportsRenderMode = SportsRenderEnhancedMarkdown

type SportsRequest struct {
	RawQuery           string
	Intent             SportsIntentType
	League             string
	Sport              string
	TeamQuery          string
	AthleteQuery       string
	SecondAthleteQuery string // for athlete comparison (SportsIntentAthleteComparison)
	GameDetailSubtype  string // "officials", "predictor", "probabilities", "gamepackage"
	StatCategory       string
	StatName           string
	StatLabel          string
	StatSort           string
	Date               *time.Time
	DateLabel          string
	Season             int
	Limit              int
	RenderMode         SportsRenderMode
	LeagueLogoURL      string
}

type LeagueConfig struct {
	DisplayName string
	Sport       string
	League      string
	Aliases     []string
}

type SportsLookupResult struct {
	Intent        SportsIntentType
	League        string
	LeagueName    string
	LeagueLogoURL string
	Sport         string
	DateLabel     string
	Markdown      string
	Source        string
	RetrievedAt   time.Time
	RenderMode    SportsRenderMode
}

type TeamIdentity struct {
	DisplayName    string `json:"display_name"`
	ShortName      string `json:"short_name,omitempty"`
	Abbreviation   string `json:"abbreviation,omitempty"`
	Location       string `json:"location,omitempty"`
	LogoURL        string `json:"logo_url,omitempty"`
	DarkLogoURL    string `json:"dark_logo_url,omitempty"`
	PrimaryColor   string `json:"primary_color,omitempty"`
	AlternateColor string `json:"alternate_color,omitempty"`
}

type LeagueIdentity struct {
	League       string
	DisplayName  string
	LogoURL      string
	Abbreviation string
}

type GameRow struct {
	Date          string
	Time          string
	Status        string
	StatusType    string
	Away          TeamIdentity
	AwayTeam      string
	AwayAbbr      string
	AwayScore     string
	Home          TeamIdentity
	HomeTeam      string
	HomeAbbr      string
	HomeScore     string
	Venue         string
	Broadcasts    string
	LinescoreRows []LinescoreRow // period/quarter breakdown; nil when not in-progress or final
}

// LinescoreRow holds the score for a single period or quarter.
type LinescoreRow struct {
	Period    int    // 1-based period/quarter number
	AwayScore string // away team's score for this period
	HomeScore string // home team's score for this period
}

type StandingsRow struct {
	Group            string
	Rank             int
	TeamIdentity     TeamIdentity
	Team             string
	Abbr             string
	Wins             string
	Losses           string
	Ties             string
	Draws            string
	NoResult         string
	Pct              string
	GamesBack        string
	Streak           string
	LastTen          string
	Points           string
	GamesPlayed      string
	GoalDifferential string
	GoalDiff         string
	NetRunRate       string
	For              string
	Against          string
	Note             string
}

type NewsRow struct {
	Published   string
	Headline    string
	Description string
	Byline      string
	URL         string
	ImageURL    string
	ImageAlt    string
}

type OddsRow struct {
	LeagueName    string
	Date          string
	Time          string
	Status        string
	StatusType    string
	Away          TeamIdentity
	AwayTeam      string
	AwayAbbr      string
	AwayMoneyLine string
	Home          TeamIdentity
	HomeTeam      string
	HomeAbbr      string
	HomeMoneyLine string
	Spread        string
	OverUnder     string
	Provider      string
}

type RosterRow struct {
	Group       string
	Name        string
	Position    string
	Jersey      string
	Age         string
	Height      string
	Weight      string
	Status      string
	HeadshotURL string // player headshot image URL (HTTPS-enforced)
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
	Type   string
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
		return "I can retrieve ESPN-backed scores, schedules, standings, news, betting odds, rosters, injuries, transactions, team records, rankings, player stats, league stats, and league leaders for supported leagues."
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
	if errors.Is(err, ErrNoOdds) {
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
		return fmt.Sprintf("ESPN did not return betting odds for %s right now.", name)
	}
	if errors.Is(err, ErrNoSportsData) {
		// For leader/stats queries with a truly historical season (pre-2002),
		// explain that ESPN's API doesn't cover data that far back.
		if (req.Intent == SportsIntentLeaders || req.Intent == SportsIntentAthleteStats) && req.Season > 0 && req.Season < 2002 {
			leagueName := req.League
			if cfg, ok := leagueConfigForRequest(req); ok {
				leagueName = cfg.DisplayName
			}
			if leagueName == "" {
				leagueName = "that league"
			}
			return fmt.Sprintf("ESPN's statistics API does not have %s data for the %d season. Historical player statistics are only available for recent seasons (approximately 2002–present).", leagueName, req.Season)
		}
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
	if req.Intent == SportsIntentOdds {
		return fmt.Sprintf("I could not retrieve %s betting odds from ESPN right now.", name)
	}
	return fmt.Sprintf("I could not retrieve %s sports data from ESPN right now.", name)
}
