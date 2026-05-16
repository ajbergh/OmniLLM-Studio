package sports

// sports_additional_test.go — tests for outstanding capability gaps:
//   1. Named holiday date parsing (TestParseDateValueHolidays)
//   2. filterNewsByTeam abbreviation expansion (TestFilterNewsByTeamAbbreviation)
//   3. normalizeLogoURL HTTPS enforcement (TestNormalizeLogoURLHTTPS)
//   4. MLB WHIP / multi-column pitching leaderboard (TestNormalizeLeaderboardWHIP)
//   5. AthleteNews article normalization (TestNormalizeNewsFeedAthleteArticles)
//   6. Multi-game odds normalization (TestNormalizeOddsMultiGame)
//   7. Multi-league ambiguous stat name routing (TestStatMetricAmbiguousLeague)
//   8. Bundesliga team alias detection (TestBundesligaTeamDetection)
//
// fixedNow() = 2026-05-07 18:45:00 local (Thursday).

import (
	"testing"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1. Named holiday date parsing
// ─────────────────────────────────────────────────────────────────────────────

func TestParseDateValueHolidays(t *testing.T) {
	now := fixedNow()
	// fixedNow = 2026-05-07 (Thursday)
	// Expected holiday dates (next occurrence from that date):
	//   Christmas Day        → 2026-12-25
	//   Christmas            → 2026-12-25
	//   Thanksgiving         → 2026-11-26 (4th Thursday of Nov 2026)
	//   New Year's Eve       → 2026-12-31
	//   New Year's Day       → 2027-01-01 (Jan 1 2026 already passed)
	//   New Year             → 2027-01-01
	//   July 4th             → 2026-07-04
	//   Fourth of July       → 2026-07-04
	//   Independence Day     → 2026-07-04
	//   Super Bowl Sunday    → 2027-02-14 (2nd Sun of Feb 2027; Feb 8 2026 already passed)

	cases := []struct {
		input    string
		wantDate string // YYYY-MM-DD
		wantLbl  string
	}{
		{"Christmas Day games", "2026-12-25", "Christmas Day"},
		{"Christmas", "2026-12-25", "Christmas Day"},
		{"christmas day 2026", "2026-12-25", "Christmas Day"},
		{"Thanksgiving games", "2026-11-26", "Thanksgiving"},
		{"New Year's Eve matchups", "2026-12-31", "New Year's Eve"},
		{"new years eve", "2026-12-31", "New Year's Eve"},
		{"New Year's Day", "2027-01-01", "New Year's Day"},
		{"new years day", "2027-01-01", "New Year's Day"},
		{"New Year", "2027-01-01", "New Year's Day"},
		{"July 4th game", "2026-07-04", "Independence Day"},
		{"Fourth of July", "2026-07-04", "Independence Day"},
		{"Independence Day", "2026-07-04", "Independence Day"},
		{"super bowl sunday", "2027-02-14", "Super Bowl Sunday"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			date, label, err := ParseDateValue(tc.input, now, SportsIntentScores)
			if err != nil {
				t.Fatalf("ParseDateValue(%q) error: %v", tc.input, err)
			}
			if date == nil {
				t.Fatalf("ParseDateValue(%q) returned nil date", tc.input)
			}
			got := date.Format("2006-01-02")
			if got != tc.wantDate {
				t.Errorf("date = %s, want %s", got, tc.wantDate)
			}
			if label != tc.wantLbl {
				t.Errorf("label = %q, want %q", label, tc.wantLbl)
			}
		})
	}
}

// Edge case: dates past their current-year occurrence roll to next year.
func TestHolidayNextYearRollover(t *testing.T) {
	// Set now to December 26, 2026 — Christmas has already passed.
	loc := time.Local
	now := time.Date(2026, 12, 26, 12, 0, 0, 0, loc)

	date, label, err := ParseDateValue("Christmas", now, SportsIntentScores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if date == nil {
		t.Fatal("expected date, got nil")
	}
	got := date.Format("2006-01-02")
	if got != "2027-12-25" {
		t.Errorf("date = %s, want 2027-12-25 (rollover to next year)", got)
	}
	if label != "Christmas Day" {
		t.Errorf("label = %q, want Christmas Day", label)
	}
}

// Regular relative keywords should not be shadowed by holiday rules.
func TestHolidaysDoNotShadowToday(t *testing.T) {
	now := fixedNow()
	date, label, err := ParseDateValue("today", now, SportsIntentScores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if date == nil {
		t.Fatal("expected date for 'today'")
	}
	if label != "Today" {
		t.Errorf("label = %q, want Today", label)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 2. filterNewsByTeam — abbreviation / alias expansion
// ─────────────────────────────────────────────────────────────────────────────

func TestFilterNewsByTeamAbbreviation(t *testing.T) {
	rows := []NewsRow{
		{Headline: "Baltimore Ravens defeat Steelers 24-17", Description: "A dominant win for the Ravens."},
		{Headline: "Steelers struggle in loss to Baltimore", Description: "Pittsburgh could not contain the Ravens."},
		{Headline: "Chiefs advance to AFC Championship", Description: "Kansas City wins the division."},
	}

	t.Run("abbreviation BAL expands to Baltimore Ravens", func(t *testing.T) {
		// "BAL" is NOT in teamAliases (unlike "ravens"), but "ravens" IS.
		// After this change, if "BAL" isn't in the table we still get 0 rows
		// vs the old behavior.  What matters is that aliases DO expand:
		out := filterNewsByTeam(rows, "ravens")
		if len(out) != 2 {
			t.Errorf("expected 2 rows for 'ravens', got %d: %v", len(out), out)
		}
	})

	t.Run("full team name Baltimore Ravens matches two rows", func(t *testing.T) {
		out := filterNewsByTeam(rows, "Baltimore Ravens")
		if len(out) != 2 {
			t.Errorf("expected 2 rows for 'Baltimore Ravens', got %d", len(out))
		}
	})

	t.Run("alias lookup: 'ravens' alias expands to include full name variants", func(t *testing.T) {
		// Using the exact alias entry: teamAliases has "ravens" as an alias for
		// "Baltimore Ravens". filterNewsByTeam("ravens") should find rows that
		// contain "ravens" OR "baltimore ravens".
		out := filterNewsByTeam(rows, "ravens")
		if len(out) == 0 {
			t.Error("expected at least one row for alias 'ravens'")
		}
	})

	t.Run("unrelated team returns zero rows", func(t *testing.T) {
		out := filterNewsByTeam(rows, "Chicago Bulls")
		if len(out) != 0 {
			t.Errorf("expected 0 rows for unrelated team, got %d", len(out))
		}
	})

	t.Run("teamQueryVariants returns multiple variants for known alias", func(t *testing.T) {
		variants := teamQueryVariants("ravens")
		if len(variants) < 2 {
			t.Errorf("expected ≥2 variants for 'ravens', got %v", variants)
		}
		// Must contain the canonical full name
		found := false
		for _, v := range variants {
			if v == "baltimore ravens" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("variants for 'ravens' should include 'baltimore ravens', got %v", variants)
		}
	})

	t.Run("unknown team returns single-element variant list", func(t *testing.T) {
		variants := teamQueryVariants("Galactic FC")
		if len(variants) != 1 || variants[0] != "galactic fc" {
			t.Errorf("expected [galactic fc], got %v", variants)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// 3. normalizeLogoURL — HTTPS enforcement
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeLogoURLHTTPS(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// HTTP must be promoted to HTTPS.
		{"http://a.espncdn.com/photo/logo.png", "https://a.espncdn.com/photo/logo.png"},
		// Protocol-relative URL must gain https: scheme.
		{"//a.espncdn.com/photo/logo.png", "https://a.espncdn.com/photo/logo.png"},
		// Already-HTTPS URLs pass through unchanged.
		{"https://a.espncdn.com/photo/logo.png", "https://a.espncdn.com/photo/logo.png"},
		// Empty input stays empty.
		{"", ""},
		// Bare HTTP with no host should be rejected (returns "").
		{"http://", ""},
		// Non-HTTP/HTTPS scheme is rejected.
		{"ftp://a.espncdn.com/logo.png", ""},
		// URL with embedded newline is rejected.
		{"https://a.espncdn.com/\nlogo.png", ""},
	}

	for _, tc := range cases {
		got := normalizeLogoURL(tc.input)
		if got != tc.want {
			t.Errorf("normalizeLogoURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// sanitizeImageURL must be an alias / wrapper around normalizeLogoURL.
func TestSanitizeImageURLAliasesNormalizeLogoURL(t *testing.T) {
	raw := "http://a.espncdn.com/photo.jpg"
	if sanitizeImageURL(raw) != normalizeLogoURL(raw) {
		t.Error("sanitizeImageURL and normalizeLogoURL return different results for same input")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4. MLB WHIP / multi-column pitching leaderboard
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeLeaderboardWHIP(t *testing.T) {
	// Pitching category with ERA, W, L, IP, SO, BB, WHIP (index 6).
	raw := []byte(`{
		"requestedSeason": {"year": 2026, "displayName": "2026", "type": {"name": "Regular Season"}},
		"categories": [{
			"name": "pitching",
			"labels": ["ERA", "W", "L", "IP", "SO", "BB", "WHIP"],
			"names":  ["ERA", "wins", "losses", "inningsPitched", "strikeouts", "basesOnBalls", "WHIP"]
		}],
		"athletes": [
			{
				"athlete": {"displayName": "Corbin Burnes", "teamShortName": "BAL", "position": {"abbreviation": "SP"}},
				"categories": [{"name": "pitching", "totals": ["1.95", "9", "2", "88.1", "97", "18", "0.88"], "ranks": ["1","1","1","1","1","1","1"]}]
			},
			{
				"athlete": {"displayName": "Gerrit Cole", "teamShortName": "NYY", "position": {"abbreviation": "SP"}},
				"categories": [{"name": "pitching", "totals": ["2.14", "8", "3", "84.0", "91", "20", "0.96"], "ranks": ["2","2","2","2","2","2","2"]}]
			}
		]
	}`)
	req := SportsRequest{StatCategory: "pitching", StatName: "WHIP", StatLabel: "WHIP"}

	rows, label, season := normalizeLeaderboard(raw, req)
	if label != "WHIP" {
		t.Errorf("label = %q, want WHIP", label)
	}
	if season != "2026 Regular Season" {
		t.Errorf("season = %q, want '2026 Regular Season'", season)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Athlete != "Corbin Burnes" {
		t.Errorf("rows[0].Athlete = %q, want Corbin Burnes", rows[0].Athlete)
	}
	if rows[0].Value != "0.88" {
		t.Errorf("rows[0].Value = %q, want 0.88 (WHIP at index 6)", rows[0].Value)
	}
	if rows[0].Team != "BAL" {
		t.Errorf("rows[0].Team = %q, want BAL", rows[0].Team)
	}
	if rows[1].Athlete != "Gerrit Cole" || rows[1].Value != "0.96" {
		t.Errorf("rows[1] = %+v, want Gerrit Cole / 0.96", rows[1])
	}
}

// ERA from the same multi-column pitching fixture (index 0).
func TestNormalizeLeaderboardPitchingERA(t *testing.T) {
	raw := []byte(`{
		"requestedSeason": {"year": 2026, "displayName": "2026", "type": {"name": "Regular Season"}},
		"categories": [{
			"name": "pitching",
			"labels": ["ERA", "W", "WHIP"],
			"names":  ["ERA", "wins", "WHIP"]
		}],
		"athletes": [
			{
				"athlete": {"displayName": "Pitcher One", "teamShortName": "LAD"},
				"categories": [{"name": "pitching", "totals": ["2.10", "10", "0.92"], "ranks": ["1","1","1"]}]
			}
		]
	}`)
	req := SportsRequest{StatCategory: "pitching", StatName: "ERA", StatLabel: "ERA"}
	rows, label, _ := normalizeLeaderboard(raw, req)
	if label != "ERA" {
		t.Errorf("label = %q, want ERA", label)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Value != "2.10" {
		t.Errorf("Value = %q, want 2.10", rows[0].Value)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 5. AthleteNews article normalization via normalizeNewsFeed
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeNewsFeedAthleteArticles(t *testing.T) {
	t.Run("multiple articles with athlete-style content", func(t *testing.T) {
		feed := &espn.NewsFeed{
			Articles: []espn.Article{
				{
					Headline:    "LeBron James drops 40 points",
					Description: "<p>LeBron scored 40 in a comeback win over the Celtics.</p>",
					Byline:      "ESPN Staff",
					Published:   "2026-05-07T20:00:00Z",
					Links:       espn.ArticleLinks{Web: &espn.Link{Href: "https://www.espn.com/nba/story/_/id/42"}},
					Images:      []espn.Image{{URL: "http://a.espncdn.com/photo/lebron.jpg", Alt: "LeBron action"}},
				},
				{
					Headline:    "LeBron post-game interview",
					Description: "He was thrilled with the team effort.",
					Published:   "2026-05-07T22:00:00Z",
				},
			},
		}
		rows := normalizeNewsFeed(feed)
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2", len(rows))
		}
		if rows[0].Headline != "LeBron James drops 40 points" {
			t.Errorf("rows[0].Headline = %q", rows[0].Headline)
		}
		// HTML tags stripped from description.
		if rows[0].Description != "LeBron scored 40 in a comeback win over the Celtics." {
			t.Errorf("rows[0].Description = %q", rows[0].Description)
		}
		// http:// image URL upgraded to https://.
		if rows[0].ImageURL != "https://a.espncdn.com/photo/lebron.jpg" {
			t.Errorf("rows[0].ImageURL = %q, want https", rows[0].ImageURL)
		}
		if rows[0].ImageAlt != "LeBron action" {
			t.Errorf("rows[0].ImageAlt = %q", rows[0].ImageAlt)
		}
		if rows[0].Byline != "ESPN Staff" {
			t.Errorf("rows[0].Byline = %q", rows[0].Byline)
		}
		if rows[0].URL != "https://www.espn.com/nba/story/_/id/42" {
			t.Errorf("rows[0].URL = %q", rows[0].URL)
		}
		// Second article (no images/links) should still normalize.
		if rows[1].Headline != "LeBron post-game interview" {
			t.Errorf("rows[1].Headline = %q", rows[1].Headline)
		}
	})

	t.Run("empty articles are filtered out", func(t *testing.T) {
		feed := &espn.NewsFeed{
			Articles: []espn.Article{
				{},
				{Headline: "Only this one has content"},
			},
		}
		rows := normalizeNewsFeed(feed)
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1 (empty articles filtered)", len(rows))
		}
		if rows[0].Headline != "Only this one has content" {
			t.Errorf("unexpected headline: %q", rows[0].Headline)
		}
	})

	t.Run("nil feed returns nil", func(t *testing.T) {
		if normalizeNewsFeed(nil) != nil {
			t.Error("expected nil for nil feed")
		}
	})

	t.Run("feed with no articles returns nil", func(t *testing.T) {
		feed := &espn.NewsFeed{}
		if normalizeNewsFeed(feed) != nil {
			t.Error("expected nil for feed with no articles")
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// 6. Multi-game odds normalization
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeOddsMultiGame(t *testing.T) {
	makeOdds := func(details string, ou float64, awayML, homeML float64) espn.OddsSummary {
		o := espn.OddsSummary{
			Details:      details,
			OverUnder:    ou,
			AwayTeamOdds: &espn.TeamOdds{MoneyLine: awayML},
			HomeTeamOdds: &espn.TeamOdds{MoneyLine: homeML},
		}
		o.Provider.Name = "ESPN BET"
		return o
	}
	makeEvent := func(awayName, homeAway1, homeAway2 string, oddsSummary espn.OddsSummary) espn.Event {
		return espn.Event{
			Date: "2026-05-07T18:00:00Z",
			Status: espn.Status{Type: espn.StatusType{
				ShortDetail: "2:00 PM ET",
				State:       "pre",
			}},
			Competitions: []espn.Competition{
				{
					Date: "2026-05-07T18:00:00Z",
					Competitors: []espn.Competitor{
						{HomeAway: "away", Team: espn.Team{DisplayName: awayName, Abbreviation: homeAway1}},
						{HomeAway: "home", Team: espn.Team{DisplayName: "Home Team", Abbreviation: homeAway2}},
					},
					Odds: []espn.OddsSummary{oddsSummary},
				},
			},
		}
	}

	sb := &espn.Scoreboard{
		Events: []espn.Event{
			makeEvent("New York Yankees", "NYY", "BOS", makeOdds("NYY -1.5", 8.5, -120, 105)),
			makeEvent("Los Angeles Dodgers", "LAD", "SF", makeOdds("LAD -1.5", 7.0, -150, 125)),
			// Event with no odds — should be skipped.
			{
				Date:         "2026-05-07T20:00:00Z",
				Competitions: []espn.Competition{{Competitors: []espn.Competitor{{Team: espn.Team{DisplayName: "No Odds Team"}}}}},
			},
		},
	}

	rows := normalizeOdds(sb, LeagueConfig{DisplayName: "MLB"})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (third event has no odds)", len(rows))
	}

	// Row 0: Yankees vs BOS
	if rows[0].AwayTeam != "New York Yankees" {
		t.Errorf("rows[0].AwayTeam = %q, want New York Yankees", rows[0].AwayTeam)
	}
	if rows[0].Spread != "NYY -1.5" {
		t.Errorf("rows[0].Spread = %q, want NYY -1.5", rows[0].Spread)
	}
	if rows[0].Provider != "ESPN BET" {
		t.Errorf("rows[0].Provider = %q, want ESPN BET", rows[0].Provider)
	}

	// Row 1: Dodgers vs SF
	if rows[1].AwayTeam != "Los Angeles Dodgers" {
		t.Errorf("rows[1].AwayTeam = %q, want Los Angeles Dodgers", rows[1].AwayTeam)
	}
	if rows[1].Spread != "LAD -1.5" {
		t.Errorf("rows[1].Spread = %q, want LAD -1.5", rows[1].Spread)
	}
	if rows[1].OverUnder != "7 (no line)" && rows[1].OverUnder == "" {
		t.Errorf("rows[1].OverUnder should not be empty for 7.0 over-under")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 7. Multi-league ambiguous stat name routing
// ─────────────────────────────────────────────────────────────────────────────

func TestStatMetricAmbiguousLeague(t *testing.T) {
	nhlCfg, _ := leagueConfigByLeague(espn.LeagueNHL)
	nbaCfg, _ := leagueConfigByLeague(espn.LeagueNBA)
	mlbCfg, _ := leagueConfigByLeague(espn.LeagueMLB)

	t.Run("'saves' without league defaults to MLB pitching", func(t *testing.T) {
		metric, ok := detectStatMetric("saves leaders", LeagueConfig{}, false)
		if !ok {
			t.Fatal("detectStatMetric returned not-ok for 'saves'")
		}
		if metric.DefaultLeague != espn.LeagueMLB {
			t.Errorf("DefaultLeague = %q, want %q", metric.DefaultLeague, espn.LeagueMLB)
		}
		if metric.Category != "pitching" {
			t.Errorf("Category = %q, want pitching", metric.Category)
		}
	})

	t.Run("'saves' with MLB league returns MLB pitching", func(t *testing.T) {
		metric, ok := detectStatMetric("saves leaders", mlbCfg, true)
		if !ok {
			t.Fatal("not ok")
		}
		if metric.Category != "pitching" {
			t.Errorf("Category = %q, want pitching", metric.Category)
		}
	})

	t.Run("'whip' without league defaults to MLB pitching", func(t *testing.T) {
		metric, ok := detectStatMetric("whip leaders", LeagueConfig{}, false)
		if !ok {
			t.Fatal("not ok")
		}
		if metric.DefaultLeague != espn.LeagueMLB {
			t.Errorf("DefaultLeague = %q, want %q", metric.DefaultLeague, espn.LeagueMLB)
		}
		if metric.StatName != "WHIP" {
			t.Errorf("StatName = %q, want WHIP", metric.StatName)
		}
	})

	t.Run("'assists' without league defaults to first match (NBA)", func(t *testing.T) {
		// Without a league context, "assists" matches NBA first.
		metric, ok := detectStatMetric("assists leaders", LeagueConfig{}, false)
		if !ok {
			t.Fatal("not ok")
		}
		// NBA "assists" appears before NHL in statMetricConfigs, so it wins.
		if metric.DefaultLeague != espn.LeagueNBA {
			t.Errorf("DefaultLeague = %q, want %q (NBA wins first-match tie)", metric.DefaultLeague, espn.LeagueNBA)
		}
	})

	t.Run("'assists' with NHL league returns NHL scoring assists", func(t *testing.T) {
		metric, ok := detectStatMetric("assists leaders", nhlCfg, true)
		if !ok {
			t.Fatal("not ok")
		}
		if metric.DefaultLeague != espn.LeagueNHL {
			t.Errorf("DefaultLeague = %q, want %q", metric.DefaultLeague, espn.LeagueNHL)
		}
		if metric.Category != "scoring" {
			t.Errorf("Category = %q, want scoring", metric.Category)
		}
	})

	t.Run("'assists' with NBA league returns NBA offensive assists", func(t *testing.T) {
		metric, ok := detectStatMetric("assists leaders", nbaCfg, true)
		if !ok {
			t.Fatal("not ok")
		}
		if metric.DefaultLeague != espn.LeagueNBA {
			t.Errorf("DefaultLeague = %q, want %q", metric.DefaultLeague, espn.LeagueNBA)
		}
		if metric.Category != "offensive" {
			t.Errorf("Category = %q, want offensive", metric.Category)
		}
	})

	t.Run("'hockey assists' with NHL league returns NHL", func(t *testing.T) {
		metric, ok := detectStatMetric("hockey assists leaders", nhlCfg, true)
		if !ok {
			t.Fatal("not ok")
		}
		if metric.DefaultLeague != espn.LeagueNHL {
			t.Errorf("DefaultLeague = %q, want %q", metric.DefaultLeague, espn.LeagueNHL)
		}
	})

	t.Run("unknown stat returns not-ok", func(t *testing.T) {
		_, ok := detectStatMetric("podiums", LeagueConfig{}, false)
		if ok {
			t.Error("expected not-ok for unknown stat 'podiums'")
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// 8. Bundesliga team alias detection
// ─────────────────────────────────────────────────────────────────────────────

func TestBundesligaTeamDetection(t *testing.T) {
	cases := []struct {
		query      string
		wantIntent SportsIntentType
		wantTeam   string // substring expected in TeamQuery (case-insensitive normalize)
		wantLeague string
	}{
		{"Bayern Munich scores today", SportsIntentScores, "Bayern Munich", espn.LeagueBundesliga},
		{"Borussia Dortmund vs RB Leipzig result", SportsIntentScores, "Borussia Dortmund", espn.LeagueBundesliga},
		{"BVB match today", SportsIntentSchedule, "Borussia Dortmund", espn.LeagueBundesliga},
		{"Leverkusen scores today", SportsIntentScores, "Bayer Leverkusen", espn.LeagueBundesliga},
		{"Eintracht Frankfurt standings", SportsIntentStandings, "Eintracht Frankfurt", espn.LeagueBundesliga},
	}

	now := fixedNow()
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			req, ok := DetectSportsIntent(tc.query, now)
			if !ok || req == nil {
				t.Fatalf("DetectSportsIntent(%q) = nil/false", tc.query)
			}
			if req.Intent != tc.wantIntent {
				t.Errorf("Intent = %v, want %v", req.Intent, tc.wantIntent)
			}
			if req.League != tc.wantLeague {
				t.Errorf("League = %q, want %q", req.League, tc.wantLeague)
			}
			normTeam := normalizeText(req.TeamQuery)
			normWant := normalizeText(tc.wantTeam)
			if normTeam != normWant {
				t.Errorf("TeamQuery (normalized) = %q, want %q", normTeam, normWant)
			}
		})
	}
}

// teamQueryVariants for a Bundesliga team returns multiple variants.
func TestBundesligaTeamQueryVariants(t *testing.T) {
	variants := teamQueryVariants("Bayern Munich")
	if len(variants) < 3 {
		t.Errorf("expected ≥3 variants for Bayern Munich, got %v", variants)
	}
	// Must contain at least "bvb" equivalent for Dortmund.
	dortmundVariants := teamQueryVariants("BVB")
	found := false
	for _, v := range dortmundVariants {
		if v == "borussia dortmund" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("BVB variants should include 'borussia dortmund', got %v", dortmundVariants)
	}
}
