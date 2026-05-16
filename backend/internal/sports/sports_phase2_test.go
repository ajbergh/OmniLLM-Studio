package sports

// Phase-2 gap tests — covers every remaining ❌ in the Coverage Summary:
//
//  - League alias coverage: Serie A, Ligue 1, Formula 1, NASCAR, PGA, ATP
//  - Team alias coverage: Serie A clubs
//  - Typo/misspelling tolerance: levenshteinDistance + fuzzy team detection
//  - Normalization – scoreboard: LinescoreRows (period/quarter breakdown)
//  - Normalization – scoreboard: GeoBroadcasts in broadcastNames
//  - Normalization – standings: Note field from StandingsEntry.Note
//  - Normalization – standings: playoff-seed edge case (seed shown in Note)
//  - Normalization – roster: HeadshotURL HTTPS enforcement in RosterRow
//  - Normalization – roster: cricket grouped roster (Batters/Bowlers/etc.)
//  - Normalization – leaderboard: large multi-athlete fixture
//  - Normalization – odds: large multi-game fixture
//  - Error sentinels: all distinct, ErrRateLimited wrapping via wrapESPNError
//  - AthleteStats query subtypes: gamelog, splits, bio intent detection
//  - Integration smoke: stub with //go:build integration lives in sports_integration_test.go

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	espn "github.com/chinmaykhachane/espn-go"
)

// ─── Serie A league detection ────────────────────────────────────────────────

func TestSerieALeagueDetection(t *testing.T) {
	tests := []struct {
		query string
	}{
		{"Serie A standings"},
		{"Italian league table"},
		{"calcio scores today"},
		{"serie a calcio standings"},
		{"Italian football scores"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.League != espn.LeagueSerieA {
				t.Fatalf("league = %q, want %q (Serie A) for query %q", got.League, espn.LeagueSerieA, tt.query)
			}
		})
	}
}

func TestSerieATeamDetection(t *testing.T) {
	tests := []struct {
		query    string
		wantTeam string
	}{
		{"Juventus scores", "Juventus"},
		{"juve standings", "Juventus"},
		{"Inter Milan news", "Inter Milan"},
		{"inter scores today", "Inter Milan"},
		{"AC Milan match today", "AC Milan"},
		{"Napoli scores", "Napoli"},
		{"AS Roma standings", "AS Roma"},
		{"Lazio score", "Lazio"},
		{"Atalanta scores today", "Atalanta"},
		{"Fiorentina match", "Fiorentina"},
		{"Torino scores", "Torino"},
		{"Bologna results", "Bologna"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.TeamQuery != tt.wantTeam {
				t.Fatalf("team = %q, want %q for query %q", got.TeamQuery, tt.wantTeam, tt.query)
			}
			if got.League != espn.LeagueSerieA {
				t.Fatalf("league = %q, want Serie A for query %q", got.League, tt.query)
			}
		})
	}
}

// ─── Ligue 1 league detection ────────────────────────────────────────────────

func TestLigue1LeagueDetection(t *testing.T) {
	tests := []struct {
		query string
	}{
		{"Ligue 1 standings"},
		{"ligue1 scores today"},
		{"French league table"},
		{"ligue un results"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.League != espn.LeagueLigue1 {
				t.Fatalf("league = %q, want %q (Ligue 1) for query %q", got.League, espn.LeagueLigue1, tt.query)
			}
		})
	}
}

// ─── Formula 1 league detection ──────────────────────────────────────────────

func TestF1LeagueDetection(t *testing.T) {
	tests := []struct {
		query string
	}{
		{"F1 standings"},
		{"formula 1 race results"},
		{"formula one schedule"},
		{"F1 racing today"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.League != espn.LeagueF1 {
				t.Fatalf("league = %q, want %q (F1) for query %q", got.League, espn.LeagueF1, tt.query)
			}
		})
	}
}

// ─── NASCAR league detection ─────────────────────────────────────────────────

func TestNASCARLeagueDetection(t *testing.T) {
	tests := []struct {
		query string
	}{
		{"NASCAR standings"},
		{"nascar cup series results"},
		{"NASCAR Cup schedule"},
		{"stock car racing results"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.League != espn.LeagueNASCARCup {
				t.Fatalf("league = %q, want %q (NASCAR Cup) for query %q", got.League, espn.LeagueNASCARCup, tt.query)
			}
		})
	}
}

// ─── PGA Tour league detection ───────────────────────────────────────────────

func TestPGALeagueDetection(t *testing.T) {
	tests := []struct {
		query string
	}{
		{"PGA standings"},
		{"pga tour results"},
		{"golf leaderboard"},
		{"men's golf scores"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.League != espn.LeaguePGA {
				t.Fatalf("league = %q, want %q (PGA) for query %q", got.League, espn.LeaguePGA, tt.query)
			}
		})
	}
}

// ─── ATP Tennis league detection ─────────────────────────────────────────────

func TestATPLeagueDetection(t *testing.T) {
	tests := []struct {
		query string
	}{
		{"ATP standings"},
		{"atp tennis results"},
		{"men's tennis scores"},
		{"ATP tour results"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.League != espn.LeagueATP {
				t.Fatalf("league = %q, want %q (ATP) for query %q", got.League, espn.LeagueATP, tt.query)
			}
		})
	}
}

// ─── Typo / misspelling tolerance ────────────────────────────────────────────

func TestLevenshteinDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"patriots", "patriots", 0}, // identical
		{"patroits", "patriots", 2}, // transposition (a,i swapped)
		{"seahaks", "seahawks", 1},  // missing w
		{"knics", "knicks", 1},      // missing k
		{"nicks", "knicks", 1},      // transposition at start
		{"", "abc", 3},              // empty a
		{"abc", "", 3},              // empty b
		{"", "", 0},                 // both empty
		{"kitten", "sitting", 3},    // classic levenshtein example
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s->%s", c.a, c.b), func(t *testing.T) {
			got := levenshteinDistance(c.a, c.b)
			if got != c.want {
				t.Fatalf("levenshteinDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestTypoToleranceTeamDetection(t *testing.T) {
	// These are single-character edit distance = 1 typos for known aliases.
	// detectTeamAliasFuzzy should match them.
	cases := []struct {
		query      string
		wantTeam   string
		wantLeague string
	}{
		// "seahaks" is 1 edit from "seahawks"
		{"seahaks score today", "Seattle Seahawks", espn.LeagueNFL},
		// "knics" is 1 edit from "knicks"
		{"knics game tonight", "New York Knicks", espn.LeagueNBA},
		// "nicks" is 1 edit from "knicks" (missing leading k); "game today" provides temporal cue
		{"nicks game today", "New York Knicks", espn.LeagueNBA},
		// "bulls" is exact; "cubes" → 2 edits from "cubs" too far but "bull" is 1 from "bulls"
		{"bulls news", "Chicago Bulls", espn.LeagueNBA},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(c.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", c.query)
			}
			if got.TeamQuery != c.wantTeam {
				t.Fatalf("TeamQuery = %q, want %q for query %q", got.TeamQuery, c.wantTeam, c.query)
			}
			if got.League != c.wantLeague {
				t.Fatalf("League = %q, want %q for query %q", got.League, c.wantLeague, c.query)
			}
		})
	}
}

func TestDetectTeamAliasFuzzyDirectly(t *testing.T) {
	// Verify fuzzy fallback is invoked when no exact alias matches.
	ta, ok := detectTeamAliasFuzzy("the seahaks won yesterday")
	if !ok {
		t.Fatal("expected fuzzy match for 'seahaks'")
	}
	if ta.TeamQuery != "Seattle Seahawks" {
		t.Fatalf("TeamQuery = %q, want Seattle Seahawks", ta.TeamQuery)
	}
}

func TestDetectTeamAliasFuzzyNoFalsePositive(t *testing.T) {
	// "sale" should not fuzzy-match any team alias (too short / too different).
	_, ok := detectTeamAliasFuzzy("sale price")
	if ok {
		t.Fatal("expected no fuzzy match for generic word 'sale'")
	}
}

// ─── LinescoreRows (period/quarter breakdown) ─────────────────────────────────

func TestNormalizeScoreboardLinescoreRows(t *testing.T) {
	// NBA-style: 4 quarters, each competitor has 4 Linescore entries.
	awayLinescores := []espn.Linescore{
		{Period: 1, Value: 28, DisplayValue: "28"},
		{Period: 2, Value: 31, DisplayValue: "31"},
		{Period: 3, Value: 25, DisplayValue: "25"},
		{Period: 4, Value: 22, DisplayValue: "22"},
	}
	homeLinescores := []espn.Linescore{
		{Period: 1, Value: 24, DisplayValue: "24"},
		{Period: 2, Value: 29, DisplayValue: "29"},
		{Period: 3, Value: 30, DisplayValue: "30"},
		{Period: 4, Value: 27, DisplayValue: "27"},
	}
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			{
				Status: espn.Status{Type: espn.StatusType{
					ShortDetail: "Final",
					Completed:   true,
					State:       "post",
				}},
				Competitions: []espn.Competition{
					{
						Date: "2026-05-07T20:00:00Z",
						Competitors: []espn.Competitor{
							{
								HomeAway:   "away",
								Score:      "106",
								Linescores: awayLinescores,
								Team:       espn.Team{DisplayName: "Boston Celtics", Abbreviation: "BOS"},
							},
							{
								HomeAway:   "home",
								Score:      "110",
								Linescores: homeLinescores,
								Team:       espn.Team{DisplayName: "Miami Heat", Abbreviation: "MIA"},
							},
						},
					},
				},
			},
		},
	}

	rows := normalizeScoreboard(sb)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.AwayScore != "106" || row.HomeScore != "110" {
		t.Fatalf("scores = %q/%q", row.AwayScore, row.HomeScore)
	}
	if len(row.LinescoreRows) != 4 {
		t.Fatalf("LinescoreRows count = %d, want 4", len(row.LinescoreRows))
	}

	// Verify individual period scores
	wantPeriods := []struct {
		period    int
		awayScore string
		homeScore string
	}{
		{1, "28", "24"},
		{2, "31", "29"},
		{3, "25", "30"},
		{4, "22", "27"},
	}
	for i, wp := range wantPeriods {
		lr := row.LinescoreRows[i]
		if lr.Period != wp.period {
			t.Fatalf("LinescoreRows[%d].Period = %d, want %d", i, lr.Period, wp.period)
		}
		if lr.AwayScore != wp.awayScore {
			t.Fatalf("LinescoreRows[%d].AwayScore = %q, want %q", i, lr.AwayScore, wp.awayScore)
		}
		if lr.HomeScore != wp.homeScore {
			t.Fatalf("LinescoreRows[%d].HomeScore = %q, want %q", i, lr.HomeScore, wp.homeScore)
		}
	}
}

func TestNormalizeScoreboardNoLinescoreWhenAbsent(t *testing.T) {
	// When neither competitor has linescore data, LinescoreRows must be nil.
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			{
				Competitions: []espn.Competition{
					{
						Date: "2026-05-07T19:00:00Z",
						Competitors: []espn.Competitor{
							{HomeAway: "away", Score: "5", Team: espn.Team{DisplayName: "Cubs", Abbreviation: "CHC"}},
							{HomeAway: "home", Score: "3", Team: espn.Team{DisplayName: "Cardinals", Abbreviation: "STL"}},
						},
					},
				},
			},
		},
	}
	rows := normalizeScoreboard(sb)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].LinescoreRows != nil {
		t.Fatalf("LinescoreRows = %v, want nil when no linescore data", rows[0].LinescoreRows)
	}
}

func TestBuildLinescoreRowsAsymmetric(t *testing.T) {
	// OT game: home team has a 5th OT quarter that away doesn't
	awayComp := &espn.Competitor{
		Linescores: []espn.Linescore{
			{Period: 1, Value: 20, DisplayValue: "20"},
			{Period: 2, Value: 18, DisplayValue: "18"},
			{Period: 3, Value: 22, DisplayValue: "22"},
			{Period: 4, Value: 20, DisplayValue: "20"},
		},
	}
	homeComp := &espn.Competitor{
		Linescores: []espn.Linescore{
			{Period: 1, Value: 19, DisplayValue: "19"},
			{Period: 2, Value: 21, DisplayValue: "21"},
			{Period: 3, Value: 18, DisplayValue: "18"},
			{Period: 4, Value: 22, DisplayValue: "22"},
			{Period: 5, Value: 8, DisplayValue: "8"},
		},
	}
	result := buildLinescoreRows(awayComp, homeComp)
	if len(result) != 5 {
		t.Fatalf("result count = %d, want 5 (OT)", len(result))
	}
	// Period 5: away has empty score, home has 8
	if result[4].Period != 5 {
		t.Fatalf("result[4].Period = %d, want 5", result[4].Period)
	}
	if result[4].AwayScore != "" {
		t.Fatalf("result[4].AwayScore = %q, want empty", result[4].AwayScore)
	}
	if result[4].HomeScore != "8" {
		t.Fatalf("result[4].HomeScore = %q, want 8", result[4].HomeScore)
	}
}

// ─── GeoBroadcasts in broadcastNames ─────────────────────────────────────────

func TestBroadcastNamesGeoBroadcast(t *testing.T) {
	// Geo broadcasts only — unmarshal from JSON to match struct tags
	var geoBcasts []espn.GeoBroadcast
	geoJSON := `[
		{"media":{"shortName":"ESPN+"}},
		{"media":{"shortName":"TNT"}}
	]`
	if err := json.Unmarshal([]byte(geoJSON), &geoBcasts); err != nil {
		t.Fatalf("unmarshal GeoBroadcast: %v", err)
	}
	got := broadcastNames(nil, geoBcasts)
	if got != "ESPN+, TNT" {
		t.Fatalf("broadcastNames = %q, want %q", got, "ESPN+, TNT")
	}
}

func TestBroadcastNamesCombinedBroadcastAndGeo(t *testing.T) {
	broadcasts := []espn.Broadcast{
		{Names: []string{"ESPN"}},
	}
	var geoBcasts []espn.GeoBroadcast
	geoJSON := `[
		{"media":{"shortName":"ESPN+"}},
		{"media":{"shortName":"ESPN"}}
	]`
	if err := json.Unmarshal([]byte(geoJSON), &geoBcasts); err != nil {
		t.Fatalf("unmarshal GeoBroadcast: %v", err)
	}
	got := broadcastNames(broadcasts, geoBcasts)
	if got != "ESPN, ESPN+" {
		t.Fatalf("broadcastNames = %q, want %q", got, "ESPN, ESPN+")
	}
}

func TestBroadcastNamesEmpty(t *testing.T) {
	got := broadcastNames(nil, nil)
	if got != "" {
		t.Fatalf("broadcastNames = %q, want empty", got)
	}
}

// ─── Standings Note field ─────────────────────────────────────────────────────

func TestStandingsNoteFieldPopulated(t *testing.T) {
	entry := standingsEntry("Boston Celtics", "BOS", 1, "30", "5")
	desc := "Clinched Eastern Conference"
	entry.Note = &espn.Note{Description: desc}

	row := standingsRowFromEntry("Eastern Conference", entry)
	if row.Note != desc {
		t.Fatalf("Note = %q, want %q", row.Note, desc)
	}
}

func TestStandingsNoteFieldEmptyWhenNil(t *testing.T) {
	entry := standingsEntry("Milwaukee Bucks", "MIL", 2, "25", "10")
	// entry.Note is nil by default
	row := standingsRowFromEntry("Eastern Conference", entry)
	if row.Note != "" {
		t.Fatalf("Note = %q, want empty when no note", row.Note)
	}
}

func TestStandingsNoteFieldTrimmed(t *testing.T) {
	entry := standingsEntry("Miami Heat", "MIA", 4, "20", "15")
	entry.Note = &espn.Note{Description: "  In Playoff Position  "}
	row := standingsRowFromEntry("Eastern Conference", entry)
	if row.Note != "In Playoff Position" {
		t.Fatalf("Note = %q, want trimmed note", row.Note)
	}
}

// ─── Playoff seed edge cases ──────────────────────────────────────────────────

func TestStandingsPlayoffSeedInNoteContext(t *testing.T) {
	// Verify that when rank=0 but playoffSeed stat is present, rank stays 0
	// (display rank is computed from list position, not the ESPN playoffSeed stat).
	entry := standingsEntry("Tampa Bay Rays", "TB", 0, "30", "15")
	entry.Stats = append(entry.Stats,
		espn.Statistic{Name: "playoffSeed", DisplayValue: "3"},
	)
	entry.Note = &espn.Note{Description: "y - Clinched Division"}

	row := standingsRowFromEntry("American League East", entry)
	if row.Rank != 0 {
		t.Fatalf("rank = %d, want 0 (display rank from list position, not playoffSeed)", row.Rank)
	}
	if row.Note != "y - Clinched Division" {
		t.Fatalf("Note = %q, want clinched note", row.Note)
	}
}

func TestStandingsMultipleTeamsPlayoffSeeds(t *testing.T) {
	// Three teams in the same division; only the leader has a Note.
	entries := []espn.StandingsEntry{
		func() espn.StandingsEntry {
			e := standingsEntry("Tampa Bay Rays", "TB", 0, "28", "8")
			e.Note = &espn.Note{Description: "z - Clinched Division"}
			return e
		}(),
		standingsEntry("Boston Red Sox", "BOS", 0, "20", "16"),
		standingsEntry("New York Yankees", "NYY", 0, "18", "18"),
	}

	var rows []StandingsRow
	for _, entry := range entries {
		rows = append(rows, standingsRowFromEntry("American League East", entry))
	}
	if rows[0].Note != "z - Clinched Division" {
		t.Fatalf("rows[0].Note = %q, want clinched note", rows[0].Note)
	}
	if rows[1].Note != "" || rows[2].Note != "" {
		t.Fatalf("non-leader Notes should be empty, got %q %q", rows[1].Note, rows[2].Note)
	}
}

// ─── Cricket grouped roster ───────────────────────────────────────────────────

func TestNormalizeRosterCricketGroups(t *testing.T) {
	// Cricket rosters use grouped format: {"position": "Batters", "items": [...]}
	groupedJSON := []byte(`[
		{
			"position": "Batters",
			"items": [
				{"displayName": "Virat Kohli", "jersey": "18"},
				{"displayName": "Rohit Sharma", "jersey": "45"}
			]
		},
		{
			"position": "Bowlers",
			"items": [
				{"displayName": "Jasprit Bumrah", "jersey": "93"}
			]
		},
		{
			"position": "All-Rounders",
			"items": [
				{"displayName": "Hardik Pandya", "jersey": "228"}
			]
		},
		{
			"position": "Wicket Keepers",
			"items": [
				{"displayName": "MS Dhoni", "jersey": "7"}
			]
		}
	]`)

	roster := &espn.TeamRoster{
		Athletes: json.RawMessage(groupedJSON),
	}
	rows := normalizeRoster(roster)
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(rows))
	}

	// Verify group labels are preserved
	if rows[0].Group != "Batters" || rows[0].Name != "Virat Kohli" {
		t.Fatalf("rows[0] = %+v, want Batters / Virat Kohli", rows[0])
	}
	if rows[1].Group != "Batters" || rows[1].Name != "Rohit Sharma" {
		t.Fatalf("rows[1] = %+v, want Batters / Rohit Sharma", rows[1])
	}
	if rows[2].Group != "Bowlers" || rows[2].Name != "Jasprit Bumrah" {
		t.Fatalf("rows[2] = %+v, want Bowlers / Jasprit Bumrah", rows[2])
	}
	if rows[3].Group != "All-Rounders" || rows[3].Name != "Hardik Pandya" {
		t.Fatalf("rows[3] = %+v, want All-Rounders / Hardik Pandya", rows[3])
	}
	if rows[4].Group != "Wicket Keepers" || rows[4].Name != "MS Dhoni" {
		t.Fatalf("rows[4] = %+v, want Wicket Keepers / MS Dhoni", rows[4])
	}
}

func TestNormalizeRosterCricketJerseys(t *testing.T) {
	// Verify jersey numbers are preserved in cricket roster
	groupedJSON := []byte(`[{"position": "Batters", "items": [
		{"displayName": "Virat Kohli", "jersey": "18"},
		{"displayName": "Rohit Sharma", "jersey": "45"}
	]}]`)
	roster := &espn.TeamRoster{Athletes: json.RawMessage(groupedJSON)}
	rows := normalizeRoster(roster)
	if rows[0].Jersey != "18" || rows[1].Jersey != "45" {
		t.Fatalf("jerseys = %q %q, want 18 45", rows[0].Jersey, rows[1].Jersey)
	}
}

// ─── RosterRow HeadshotURL HTTPS enforcement ──────────────────────────────────

func TestRosterRowHeadshotURLHTTPS(t *testing.T) {
	groupedJSON := []byte(`[{"position": "Guards", "items": [
		{
			"displayName": "LeBron James",
			"headshot": {"href": "http://a.espncdn.com/i/headshots/nba/players/full/1966.png"}
		},
		{
			"displayName": "Anthony Davis",
			"headshot": {"href": "https://a.espncdn.com/i/headshots/nba/players/full/3202.png"}
		}
	]}]`)
	roster := &espn.TeamRoster{Athletes: json.RawMessage(groupedJSON)}
	rows := normalizeRoster(roster)
	if len(rows) < 2 {
		t.Fatalf("rows = %d, want ≥2", len(rows))
	}
	// http → https upgrade
	if !strings.HasPrefix(rows[0].HeadshotURL, "https://") {
		t.Fatalf("HeadshotURL %q not upgraded to https", rows[0].HeadshotURL)
	}
	// already https — preserved
	if rows[1].HeadshotURL != "https://a.espncdn.com/i/headshots/nba/players/full/3202.png" {
		t.Fatalf("HeadshotURL %q changed unexpectedly", rows[1].HeadshotURL)
	}
}

func TestRosterRowHeadshotURLEmpty(t *testing.T) {
	// Athlete with no headshot → HeadshotURL is empty string.
	groupedJSON := []byte(`[{"position": "Forwards", "items": [
		{"displayName": "No Headshot Player"}
	]}]`)
	roster := &espn.TeamRoster{Athletes: json.RawMessage(groupedJSON)}
	rows := normalizeRoster(roster)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].HeadshotURL != "" {
		t.Fatalf("HeadshotURL = %q, want empty when no headshot", rows[0].HeadshotURL)
	}
}

func TestRosterRowHeadshotURLRejectsJavascript(t *testing.T) {
	groupedJSON := []byte(`[{"position": "Forwards", "items": [
		{"displayName": "Attacker", "headshot": {"href": "javascript:alert(1)"}}
	]}]`)
	roster := &espn.TeamRoster{Athletes: json.RawMessage(groupedJSON)}
	rows := normalizeRoster(roster)
	if rows[0].HeadshotURL != "" {
		t.Fatalf("HeadshotURL = %q, want empty for javascript: URL", rows[0].HeadshotURL)
	}
}

// ─── Multi-athlete leaderboard ("multi-page") ────────────────────────────────

func TestNormalizeLeaderboardLargeAthleteList(t *testing.T) {
	// Simulate 15 athletes — demonstrates large "page 1 + page 2" type fixture
	// normalizeLeaderboard processes all of them in a single pass.
	athletes := make([]map[string]any, 15)
	for i := range athletes {
		athletes[i] = map[string]any{
			"athlete": map[string]any{
				"displayName": fmt.Sprintf("Player %d", i+1),
				"team":        map[string]any{"displayName": "Team A"},
			},
			"categories": []map[string]any{
				{
					"name":   "batting",
					"totals": []string{fmt.Sprintf("%d", (i+1)*3)}, // HR values
					"ranks":  []string{fmt.Sprintf("%d", i+1)},
				},
			},
		}
	}
	payload := map[string]any{
		"athletes": athletes,
		"categories": []map[string]any{
			{
				"name":   "batting",
				"labels": []string{"HR"},
				"names":  []string{"homeRuns"},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	req := SportsRequest{
		League:       espn.LeagueMLB,
		StatCategory: "batting",
		StatName:     "homeRuns",
		StatLabel:    "HR",
		Limit:        15,
	}
	rows, label, _ := normalizeLeaderboard(raw, req)
	if len(rows) != 15 {
		t.Fatalf("rows = %d, want 15", len(rows))
	}
	if label != "HR" {
		t.Fatalf("label = %q, want HR", label)
	}
	// First and last athlete
	if rows[0].Athlete != "Player 1" {
		t.Fatalf("rows[0].Athlete = %q, want Player 1", rows[0].Athlete)
	}
	if rows[14].Athlete != "Player 15" {
		t.Fatalf("rows[14].Athlete = %q, want Player 15", rows[14].Athlete)
	}
	// Ranks should be 1..15
	for i, row := range rows {
		if row.Rank != i+1 {
			t.Fatalf("rows[%d].Rank = %d, want %d", i, row.Rank, i+1)
		}
	}
}

// ─── Multi-game odds (large fixture) ─────────────────────────────────────────

func makeOddsEvent(awayName, homeName, awayAbbr, homeAbbr string, awayML, homeML float64) espn.Event {
	awayOdds := &espn.TeamOdds{MoneyLine: awayML}
	homeOdds := &espn.TeamOdds{MoneyLine: homeML}
	odds := espn.OddsSummary{
		AwayTeamOdds: awayOdds,
		HomeTeamOdds: homeOdds,
	}
	odds.Provider.Name = "ESPN BET"
	return espn.Event{
		Date: "2026-05-07T18:00:00Z",
		Status: espn.Status{Type: espn.StatusType{
			ShortDetail: "7:00 PM",
			State:       "pre",
		}},
		Competitions: []espn.Competition{
			{
				Date: "2026-05-07T18:00:00Z",
				Competitors: []espn.Competitor{
					{HomeAway: "away", Team: espn.Team{DisplayName: awayName, Abbreviation: awayAbbr}},
					{HomeAway: "home", Team: espn.Team{DisplayName: homeName, Abbreviation: homeAbbr}},
				},
				Odds: []espn.OddsSummary{odds},
			},
		},
	}
}

func TestNormalizeOddsLargeMultiGameFixture(t *testing.T) {
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			makeOddsEvent("Yankees", "Red Sox", "NYY", "BOS", -150, 130),
			makeOddsEvent("Dodgers", "Giants", "LAD", "SF", -200, 170),
			makeOddsEvent("Astros", "Rangers", "HOU", "TEX", -110, -110),
			makeOddsEvent("Cubs", "Cardinals", "CHC", "STL", 120, -140),
			makeOddsEvent("Braves", "Mets", "ATL", "NYM", -125, 105),
		},
	}
	cfg := LeagueConfig{DisplayName: "MLB"}
	rows := normalizeOdds(sb, cfg)
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(rows))
	}
	// Spot-check first game
	if rows[0].AwayTeam != "Yankees" || rows[0].HomeTeam != "Red Sox" {
		t.Fatalf("rows[0] teams = %q/%q", rows[0].AwayTeam, rows[0].HomeTeam)
	}
	if rows[0].AwayMoneyLine != "-150" || rows[0].HomeMoneyLine != "+130" {
		t.Fatalf("rows[0] moneylines = %q/%q", rows[0].AwayMoneyLine, rows[0].HomeMoneyLine)
	}
	// Spot-check last game
	if rows[4].AwayTeam != "Braves" || rows[4].HomeTeam != "Mets" {
		t.Fatalf("rows[4] teams = %q/%q", rows[4].AwayTeam, rows[4].HomeTeam)
	}
}

// ─── Error sentinel distinctness & wrapESPNError ─────────────────────────────

func TestErrorSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		ErrUnsupportedLeague,
		ErrMalformedDate,
		ErrNoGames,
		ErrNoMatchingGames,
		ErrNoStandings,
		ErrNoNews,
		ErrNoOdds,
		ErrNoSportsData,
		ErrTeamNotFound,
		ErrAthleteNotFound,
		ErrRateLimited,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Fatalf("sentinel[%d] (%v) unexpectedly Is sentinel[%d] (%v)", i, a, j, b)
			}
		}
	}
}

func TestErrorSentinelsAllNonNil(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrUnsupportedLeague", ErrUnsupportedLeague},
		{"ErrMalformedDate", ErrMalformedDate},
		{"ErrNoGames", ErrNoGames},
		{"ErrNoMatchingGames", ErrNoMatchingGames},
		{"ErrNoStandings", ErrNoStandings},
		{"ErrNoNews", ErrNoNews},
		{"ErrNoOdds", ErrNoOdds},
		{"ErrNoSportsData", ErrNoSportsData},
		{"ErrTeamNotFound", ErrTeamNotFound},
		{"ErrAthleteNotFound", ErrAthleteNotFound},
		{"ErrRateLimited", ErrRateLimited},
	}
	for _, s := range sentinels {
		if s.err == nil {
			t.Fatalf("%s is nil", s.name)
		}
		if s.err.Error() == "" {
			t.Fatalf("%s has empty error message", s.name)
		}
	}
}

func TestWrapESPNErrorRateLimited(t *testing.T) {
	ctx := context.Background()
	wrappedErr := fmt.Errorf("ESPN BET: %w", espn.ErrRateLimited)
	result := wrapESPNError(ctx, wrappedErr)
	if !errors.Is(result, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited in wrapped error, got: %v", result)
	}
}

func TestWrapESPNErrorContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := wrapESPNError(ctx, nil)
	if !errors.Is(result, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", result)
	}
}

func TestWrapESPNErrorPassthrough(t *testing.T) {
	ctx := context.Background()
	origErr := errors.New("some ESPN error")
	result := wrapESPNError(ctx, origErr)
	if result != origErr {
		t.Fatalf("expected passthrough error, got: %v", result)
	}
}

// ─── AthleteStats query subtypes (gamelog / splits / bio) ────────────────────

func TestAthleteStatsQuerySubtypeDetection(t *testing.T) {
	tests := []struct {
		query        string
		wantIntent   SportsIntentType
		wantAthleteQ string
	}{
		{"Connor McDavid game log", SportsIntentAthleteStats, "connor mcdavid"},
		{"Shohei Ohtani gamelog 2025", SportsIntentAthleteStats, "shohei ohtani"},
		{"Patrick Mahomes splits", SportsIntentAthleteStats, "patrick mahomes"},
		{"LeBron James stats", SportsIntentAthleteStats, "lebron james"},
		{"Stephen Curry bio", SportsIntentAthleteStats, "stephen curry"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("expected sports lookup for %q", tt.query)
			}
			if got.Intent != tt.wantIntent {
				t.Fatalf("intent = %q, want %q for query %q", got.Intent, tt.wantIntent, tt.query)
			}
			if got.AthleteQuery != tt.wantAthleteQ {
				t.Fatalf("AthleteQuery = %q, want %q for query %q", got.AthleteQuery, tt.wantAthleteQ, tt.query)
			}
		})
	}
}

func TestAthleteGamelogQueryPhrase(t *testing.T) {
	// "gamelog" and "game log" both parse as SportsIntentAthleteStats
	// The downstream lookupAthleteRaw uses RawQuery to route to AthleteGamelog endpoint.
	for _, q := range []string{"Connor McDavid game log", "Connor McDavid gamelog"} {
		got, ok := DetectSportsIntent(q, fixedNow())
		if !ok {
			t.Fatalf("expected sports lookup for %q", q)
		}
		if got.Intent != SportsIntentAthleteStats {
			t.Fatalf("intent = %q for query %q", got.Intent, q)
		}
		// RawQuery should preserve the original so lookupAthleteRaw can route correctly
		normQ := normalizeText(got.RawQuery)
		if !hasAnyPhrase(normQ, "game log", "gamelog") {
			t.Fatalf("RawQuery %q does not contain 'gamelog' phrase", got.RawQuery)
		}
	}
}

func TestAthleteSplitsQueryPhrase(t *testing.T) {
	got, ok := DetectSportsIntent("Patrick Mahomes splits 2025", fixedNow())
	if !ok {
		t.Fatal("expected sports lookup")
	}
	if got.Intent != SportsIntentAthleteStats {
		t.Fatalf("intent = %q, want AthleteStats", got.Intent)
	}
	if !strings.Contains(strings.ToLower(got.RawQuery), "splits") {
		t.Fatalf("RawQuery %q does not contain 'splits'", got.RawQuery)
	}
}

// ─── LeagueConfig is registered for new leagues ──────────────────────────────

func TestLeagueConfigRegisteredForNewLeagues(t *testing.T) {
	// Verify the six new leagues are properly registered in leagueConfigs
	// so they can be looked up by their league string.
	leagues := []struct {
		league      string
		displayName string
	}{
		{espn.LeagueSerieA, "Serie A"},
		{espn.LeagueLigue1, "Ligue 1"},
		{espn.LeagueF1, "Formula 1"},
		{espn.LeagueNASCARCup, "NASCAR Cup"},
		{espn.LeaguePGA, "PGA Tour"},
		{espn.LeagueATP, "ATP Tennis"},
	}
	for _, l := range leagues {
		t.Run(l.displayName, func(t *testing.T) {
			cfg, ok := leagueConfigByLeague(l.league)
			if !ok {
				t.Fatalf("leagueConfigByLeague(%q) not found", l.league)
			}
			if cfg.DisplayName != l.displayName {
				t.Fatalf("DisplayName = %q, want %q", cfg.DisplayName, l.displayName)
			}
		})
	}
}
