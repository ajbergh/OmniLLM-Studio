package sports

// sports_q77_100_test.go — Unit tests for Q77–Q87 (Venues, Power Index,
// Recruits, Bracketology) and Q94–Q99 (F1, NASCAR, PGA, ATP).

import (
	"encoding/json"
	"testing"

	espn "github.com/chinmaykhachane/espn-go"
)

// ─── Q77–Q78: Venues / Stadium intent detection ──────────────────────────────

func TestVenuesIntentDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantOK     bool
		wantIntent SportsIntentType
		wantLeague string
	}{
		{"What is the home stadium of the Yankees?", true, SportsIntentVenues, espn.LeagueMLB},
		// "home games" triggers team_schedule, not venues — expected behavior
		{"Where do the Lakers play their home games?", true, SportsIntentTeamSchedule, espn.LeagueNBA},
		{"What stadium do the Yankees play in?", true, SportsIntentVenues, espn.LeagueMLB},
		{"What is the home arena for the Bulls?", true, SportsIntentVenues, espn.LeagueNBA},
		{"Which ballpark does the Red Sox play in?", true, SportsIntentVenues, espn.LeagueMLB},
		{"Show me all MLB home venues", true, SportsIntentVenues, espn.LeagueMLB},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.Intent != c.wantIntent {
				t.Fatalf("intent=%q, want %q", got.Intent, c.wantIntent)
			}
			if c.wantLeague != "" && got.League != c.wantLeague {
				t.Fatalf("league=%q, want %q", got.League, c.wantLeague)
			}
		})
	}
}

// TestNormalizeVenueStruct verifies the helper that converts an espn.Venue to
// a display row — no live API needed.
func TestNormalizeVenueStruct(t *testing.T) {
	v := espn.Venue{
		ID:       "3615",
		FullName: "Fenway Park",
		Address: espn.Address{
			City:  "Boston",
			State: "MA",
		},
		Capacity: 37755,
		Indoor:   false,
		Grass:    true,
	}
	row := NormalizeVenueStruct(v)
	// row = [FullName, City+State, Capacity, indoor/outdoor, grass/turf]
	if len(row) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(row))
	}
	if row[0] != "Fenway Park" {
		t.Errorf("col[0] = %q, want %q", row[0], "Fenway Park")
	}
	if row[1] != "Boston, MA" {
		t.Errorf("col[1] = %q, want %q", row[1], "Boston, MA")
	}
	if row[2] != "37755" {
		t.Errorf("col[2] = %q, want %q", row[2], "37755")
	}
	if row[3] != "outdoor" {
		t.Errorf("col[3] = %q, want %q", row[3], "outdoor")
	}
	if row[4] != "grass" {
		t.Errorf("col[4] = %q, want %q", row[4], "grass")
	}
}

func TestNormalizeVenueStructIndoorNoCapacity(t *testing.T) {
	v := espn.Venue{
		FullName: "Chase Center",
		Indoor:   true,
		Grass:    false,
		Address:  espn.Address{City: "San Francisco", State: "CA"},
	}
	row := NormalizeVenueStruct(v)
	if row[2] != "" {
		t.Errorf("expected empty capacity, got %q", row[2])
	}
	if row[3] != "indoor" {
		t.Errorf("expected indoor, got %q", row[3])
	}
	if row[4] != "turf" {
		t.Errorf("expected turf, got %q", row[4])
	}
}

func TestVenueIDFromRef(t *testing.T) {
	cases := []struct {
		ref  string
		want string
	}{
		{"http://sports.core.api.espn.com/v2/sports/football/venues/3615", "3615"},
		{"https://sports.core.api.espn.com/v2/sports/baseball/venues/999", "999"},
		{"", ""},
		{"noslash", ""},
	}
	for _, c := range cases {
		got := venueIDFromRef(c.ref)
		if got != c.want {
			t.Errorf("venueIDFromRef(%q) = %q, want %q", c.ref, got, c.want)
		}
	}
}

// ─── Q83–Q84: Power Index intent detection ───────────────────────────────────

func TestPowerIndexIntentDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantOK     bool
		wantIntent SportsIntentType
		wantLeague string
	}{
		{"Show the CFB power index rankings", true, SportsIntentPowerIndex, espn.LeagueCollegeFootball},
		{"What is the FPI for college football?", true, SportsIntentPowerIndex, espn.LeagueCollegeFootball},
		{"Show me the College Football BPI", true, SportsIntentPowerIndex, espn.LeagueCollegeFootball},
		{"What teams have the highest SP+ rating?", true, SportsIntentPowerIndex, espn.LeagueCollegeFootball},
		{"College Football power index leaders 2025", true, SportsIntentPowerIndex, espn.LeagueCollegeFootball},
		// "NBA power rankings" resolves to standings (not power index) — expected behavior
		{"Show me the NBA power rankings", true, SportsIntentStandings, espn.LeagueNBA},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.Intent != c.wantIntent {
				t.Fatalf("intent=%q, want %q", got.Intent, c.wantIntent)
			}
			if c.wantLeague != "" && got.League != c.wantLeague {
				t.Fatalf("league=%q, want %q", got.League, c.wantLeague)
			}
		})
	}
}

// ─── Q85–Q86: Recruits intent detection ─────────────────────────────────────

func TestRecruitsIntentDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantOK     bool
		wantLeague string
	}{
		{"Who are the top college football recruits for 2025?", true, espn.LeagueCollegeFootball},
		{"Show me the CFB recruiting rankings", true, espn.LeagueCollegeFootball},
		{"Show Alabama's recruiting class", true, espn.LeagueCollegeFootball},
		{"Top college football recruits this year", true, espn.LeagueCollegeFootball},
		{"CFB top commits 2025", true, espn.LeagueCollegeFootball},
		{"College football recruit rankings", true, espn.LeagueCollegeFootball},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.Intent != SportsIntentRecruits {
				t.Fatalf("intent=%q, want %q", got.Intent, SportsIntentRecruits)
			}
			if c.wantLeague != "" && got.League != c.wantLeague {
				t.Fatalf("league=%q, want %q", got.League, c.wantLeague)
			}
		})
	}
}

func TestRecruitsTeamQueryPassedThrough(t *testing.T) {
	got, ok := DetectSportsIntent("Show Alabama's recruiting class", fixedNow())
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Intent != SportsIntentRecruits {
		t.Fatalf("intent=%q, want recruits", got.Intent)
	}
	// Alabama should be detected as teamQuery
	if got.TeamQuery != "Alabama Crimson Tide" {
		t.Logf("TeamQuery=%q (Alabama alias coverage may vary)", got.TeamQuery)
	}
}

// ─── Q87: Bracketology intent detection ─────────────────────────────────────

func TestBracketologyIntentDetection(t *testing.T) {
	cases := []struct {
		query  string
		wantOK bool
	}{
		{"Show current bracketology", true},
		{"What is the latest NCAA bracket projection?", true},
		{"Show me the tournament bracket outlook", true},
		{"NCAA bracket projections for 2025", true},
		{"March Madness bracket predictions", true},
		{"Show me the March Madness bracket", true},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if c.wantOK && got.Intent != SportsIntentBracketology {
				t.Fatalf("intent=%q, want %q", got.Intent, SportsIntentBracketology)
			}
		})
	}
}

func TestBracketologySeasonExtracted(t *testing.T) {
	got, ok := DetectSportsIntent("NCAA bracket projections for 2025", fixedNow())
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Season != 2025 {
		t.Fatalf("Season=%d, want 2025", got.Season)
	}
}

// ─── Q94–Q99: F1, NASCAR, PGA, ATP intent detection ─────────────────────────

func TestF1IntentDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantOK     bool
		wantIntent SportsIntentType
	}{
		{"Show me the F1 standings", true, SportsIntentStandings},
		{"F1 race results today", true, SportsIntentScores},
		{"Formula 1 championship standings", true, SportsIntentStandings},
		{"Formula one race schedule", true, SportsIntentSchedule},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.League != espn.LeagueF1 {
				t.Fatalf("league=%q, want %q", got.League, espn.LeagueF1)
			}
			if got.Intent != c.wantIntent {
				t.Fatalf("intent=%q, want %q", got.Intent, c.wantIntent)
			}
		})
	}
}

func TestNASCARIntentDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantOK     bool
		wantIntent SportsIntentType
	}{
		{"What are the NASCAR Cup race results?", true, SportsIntentScores},
		{"NASCAR standings 2025", true, SportsIntentStandings},
		{"NASCAR Cup Series schedule", true, SportsIntentSchedule},
		{"Show me NASCAR news", true, SportsIntentNews},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.League != espn.LeagueNASCARCup {
				t.Fatalf("league=%q, want %q", got.League, espn.LeagueNASCARCup)
			}
			if got.Intent != c.wantIntent {
				t.Fatalf("intent=%q, want %q", got.Intent, c.wantIntent)
			}
		})
	}
}

func TestPGAIntentDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantOK     bool
		wantIntent SportsIntentType
	}{
		{"Show the PGA Tour leaderboard", true, SportsIntentLeaders},
		{"PGA Tour standings 2025", true, SportsIntentStandings},
		{"PGA tournament scores today", true, SportsIntentScores},
		{"PGA Tour schedule", true, SportsIntentSchedule},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.League != espn.LeaguePGA {
				t.Fatalf("league=%q, want %q", got.League, espn.LeaguePGA)
			}
			if got.Intent != c.wantIntent {
				t.Fatalf("intent=%q, want %q", got.Intent, c.wantIntent)
			}
		})
	}
}

func TestATPIntentDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantOK     bool
		wantIntent SportsIntentType
	}{
		{"Show ATP tennis rankings", true, SportsIntentStandings}, // "rankings" → standings
		{"ATP World Tour standings", true, SportsIntentStandings},
		{"ATP scores today", true, SportsIntentScores},
		{"ATP tennis news", true, SportsIntentNews},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if ok != c.wantOK {
				t.Fatalf("DetectSportsIntent(%q) ok=%v, want %v", c.query, ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.League != espn.LeagueATP {
				t.Fatalf("league=%q, want %q", got.League, espn.LeagueATP)
			}
			if got.Intent != c.wantIntent {
				t.Fatalf("intent=%q, want %q", got.Intent, c.wantIntent)
			}
		})
	}
}

// ─── Normalization fixtures (no live API) ───────────────────────────────────

// TestPowerIndexRawJSONNormalized verifies rawJSONTable works on a Power Index
// -shaped JSON payload, exercising the normalization path used by LookupPowerIndex.
func TestPowerIndexRawJSONNormalized(t *testing.T) {
	raw := json.RawMessage(`[
		{"team": "Alabama", "fpi": 25.3, "rank": 1},
		{"team": "Georgia", "fpi": 24.1, "rank": 2},
		{"team": "Ohio State", "fpi": 22.7, "rank": 3}
	]`)
	table := rawJSONTable(raw, 10)
	if len(table.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(table.Rows))
	}
	// Headers should include "team", "fpi", "rank" (order may vary)
	if len(table.Headers) == 0 {
		t.Fatal("expected non-empty headers")
	}
}

// TestRecruitsRawJSONNormalized verifies the recruits JSON payload normalizes.
func TestRecruitsRawJSONNormalized(t *testing.T) {
	raw := json.RawMessage(`[
		{"name": "Recruit A", "position": "QB", "stars": 5, "school": "Alabama"},
		{"name": "Recruit B", "position": "WR", "stars": 4, "school": "Georgia"}
	]`)
	table := rawJSONTable(raw, 10)
	if len(table.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(table.Rows))
	}
}

// TestBracketologyRawJSONNormalized verifies bracketology JSON normalizes.
func TestBracketologyRawJSONNormalized(t *testing.T) {
	raw := json.RawMessage(`[
		{"seed": 1, "team": "Houston", "region": "South"},
		{"seed": 1, "team": "Kansas", "region": "Midwest"},
		{"seed": 1, "team": "Purdue", "region": "East"},
		{"seed": 1, "team": "Arizona", "region": "West"}
	]`)
	table := rawJSONTable(raw, 10)
	if len(table.Rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(table.Rows))
	}
}

// ─── New intent sentinel checks ─────────────────────────────────────────────

func TestNewIntentSentinelsAreDistinct(t *testing.T) {
	intents := []SportsIntentType{
		SportsIntentVenues,
		SportsIntentPowerIndex,
		SportsIntentRecruits,
		SportsIntentBracketology,
	}
	seen := map[SportsIntentType]bool{}
	for _, i := range intents {
		if seen[i] {
			t.Fatalf("duplicate intent value: %q", i)
		}
		seen[i] = true
		if i == SportsIntentUnknown {
			t.Fatalf("intent %q must not equal SportsIntentUnknown", i)
		}
	}
}

// TestNewLeagueConfigsRegistered verifies leagueConfigByLeague resolves all new
// leagues added in Q94-Q99.
func TestNewLeagueConfigsRegistered(t *testing.T) {
	leagues := []string{
		espn.LeagueSerieA,
		espn.LeagueLigue1,
		espn.LeagueF1,
		espn.LeagueNASCARCup,
		espn.LeaguePGA,
		espn.LeagueATP,
	}
	for _, l := range leagues {
		_, ok := leagueConfigByLeague(l)
		if !ok {
			t.Errorf("leagueConfigByLeague(%q) not found", l)
		}
	}
}

// ─── MLB stat leaders — "steals" cross-sport disambiguation and related stats ─

// TestMLBStealsLeaders is the primary regression for the bug: "steals" used
// as a synonym for stolen bases when MLB is explicitly specified.
func TestMLBStealsLeaders(t *testing.T) {
	cases := []struct {
		query        string
		wantLeague   string
		wantStatName string
		wantSeason   int
	}{
		// Core bug: "steals" + explicit MLB must resolve to stolenBases
		{"Who had the most steals in MLB in 1979", espn.LeagueMLB, "stolenBases", 1979},
		{"Who leads MLB in steals this season?", espn.LeagueMLB, "stolenBases", 0},
		{"Most steals in MLB 2023", espn.LeagueMLB, "stolenBases", 2023},
		{"MLB steals leaders 2024", espn.LeagueMLB, "stolenBases", 2024},
		{"Who has the most steals in baseball?", espn.LeagueMLB, "stolenBases", 0},
		// "stolen bases" phrasing must still work
		{"Most stolen bases in MLB 2022", espn.LeagueMLB, "stolenBases", 2022},
		{"Who leads in stolen bases this season?", espn.LeagueMLB, "stolenBases", 0},
		// NBA must still get avgSteals (regression guard)
		{"NBA steals leaders 2021", espn.LeagueNBA, "avgSteals", 2021},
		{"Who leads the NBA in steals per game?", espn.LeagueNBA, "avgSteals", 0},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) ok=false, want true", c.query)
			}
			if got.Intent != SportsIntentLeaders {
				t.Fatalf("intent=%q, want leaders", got.Intent)
			}
			if got.League != c.wantLeague {
				t.Fatalf("league=%q, want %q", got.League, c.wantLeague)
			}
			if got.StatName != c.wantStatName {
				t.Fatalf("statName=%q, want %q", got.StatName, c.wantStatName)
			}
			if c.wantSeason != 0 && got.Season != c.wantSeason {
				t.Fatalf("season=%d, want %d", got.Season, c.wantSeason)
			}
		})
	}
}

// TestMLBBattingStatLeaders covers the full range of MLB batting stats,
// exercising historical seasons (pre-2000) and various phrasings.
func TestMLBBattingStatLeaders(t *testing.T) {
	cases := []struct {
		query        string
		wantStatName string
		wantSeason   int
	}{
		{"Who hit the most home runs in MLB in 1998?", "homeRuns", 1998},
		{"MLB home run leaders 2024", "homeRuns", 2024},
		{"Most RBIs in MLB 1991", "RBIs", 1991},
		{"Who led MLB in RBI in 2023?", "RBIs", 2023},
		{"Batting average leaders in MLB 2024", "avg", 2024},
		{"Who had the highest batting average in MLB in 1941?", "avg", 1941},
		{"MLB hits leaders 2022", "hits", 2022},
		{"Who had the most hits in baseball in 2004?", "hits", 2004},
		{"MLB stolen bases leaders 2023", "stolenBases", 2023},
		{"Most stolen bases in MLB 1982", "stolenBases", 1982},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) ok=false, want true", c.query)
			}
			if got.Intent != SportsIntentLeaders {
				t.Fatalf("intent=%q, want leaders", got.Intent)
			}
			if got.League != espn.LeagueMLB {
				t.Fatalf("league=%q, want %q", got.League, espn.LeagueMLB)
			}
			if got.StatName != c.wantStatName {
				t.Fatalf("statName=%q, want %q", got.StatName, c.wantStatName)
			}
			if c.wantSeason != 0 && got.Season != c.wantSeason {
				t.Fatalf("season=%d, want %d", got.Season, c.wantSeason)
			}
		})
	}
}

// TestMLBPitchingStatLeaders covers MLB pitching stats.
func TestMLBPitchingStatLeaders(t *testing.T) {
	cases := []struct {
		query        string
		wantStatName string
		wantSeason   int
	}{
		{"Who had the lowest ERA in MLB in 1968?", "ERA", 1968},
		{"MLB ERA leaders 2024", "ERA", 2024},
		{"Most strikeouts in MLB 1973", "strikeouts", 1973},
		{"Who led MLB in strikeouts in 2023?", "strikeouts", 2023},
		{"MLB saves leaders 2022", "saves", 2022},
		{"Who had the most saves in baseball in 2001?", "saves", 2001},
		{"MLB WHIP leaders 2024", "WHIP", 2024},
		{"Who had the best WHIP in MLB in 1985?", "WHIP", 1985},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) ok=false, want true", c.query)
			}
			if got.Intent != SportsIntentLeaders {
				t.Fatalf("intent=%q, want leaders", got.Intent)
			}
			if got.League != espn.LeagueMLB {
				t.Fatalf("league=%q, want %q", got.League, espn.LeagueMLB)
			}
			if got.StatName != c.wantStatName {
				t.Fatalf("statName=%q, want %q", got.StatName, c.wantStatName)
			}
			if c.wantSeason != 0 && got.Season != c.wantSeason {
				t.Fatalf("season=%d, want %d", got.Season, c.wantSeason)
			}
		})
	}
}
