package sports

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectSportsIntentExtended: intent detection for queries from the ESPN-go
// test plan, including fixes applied this session (schedule "game" phrase,
// "this weekend" temporal phrase, Unknown+temporal "play" upgrade, and
// athlete-news fallback for no-league news queries).
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectSportsIntentExtended(t *testing.T) {
	tests := []struct {
		query         string
		wantIntent    SportsIntentType
		wantLeague    string
		wantTeam      string
		wantDateLabel string
		wantSeason    int
		wantLimit     int // 0 = don't check
	}{
		// ── Schedule: league-wide with temporal date labels ──────────────────
		{
			query:         "What is the NBA schedule tonight?",
			wantIntent:    SportsIntentSchedule,
			wantLeague:    espn.LeagueNBA,
			wantDateLabel: "Tonight",
		},
		{
			query:         "Are there any WNBA games tomorrow?",
			wantIntent:    SportsIntentSchedule,
			wantLeague:    espn.LeagueWNBA,
			wantDateLabel: "Tomorrow",
		},
		{
			// Fix 1: "this weekend" now recognised as a temporal phrase, so
			// the query is not rejected as Unknown.
			query:      "What college football games are scheduled for this weekend?",
			wantIntent: SportsIntentSchedule,
			wantLeague: espn.LeagueCollegeFootball,
		},
		// ── Fix 2: "game" (singular) → SportsIntentSchedule → TeamSchedule ──
		{
			query:      "When is the next Chicago Bears game?",
			wantIntent: SportsIntentTeamSchedule,
			wantLeague: espn.LeagueNFL,
			wantTeam:   "Chicago Bears",
		},
		// ── Fix 3: Unknown intent + temporal + "play" keyword → Schedule ─────
		{
			// "play" triggers the Unknown-intent temporal upgrade to Schedule.
			// The TeamSchedule conversion does NOT happen here because that
			// conversion runs before the Unknown branch.
			query:      "What time do the Packers play this week?",
			wantIntent: SportsIntentSchedule,
			wantLeague: espn.LeagueNFL,
			wantTeam:   "Green Bay Packers",
		},
		// ── Standings ────────────────────────────────────────────────────────
		{
			query:      "Where do the Detroit Lions sit in the NFC North standings?",
			wantIntent: SportsIntentStandings,
			wantLeague: espn.LeagueNFL,
			wantTeam:   "Detroit Lions",
		},
		{
			query:         "What are the current Premier League standings?",
			wantIntent:    SportsIntentStandings,
			wantLeague:    espn.LeagueEPL,
			wantDateLabel: "Current",
		},
		// ── Roster ───────────────────────────────────────────────────────────
		{
			query:      "Show me the LA Kings roster.",
			wantIntent: SportsIntentRoster,
			wantLeague: espn.LeagueNHL,
			wantTeam:   "Los Angeles Kings",
		},
		// ── Injuries ─────────────────────────────────────────────────────────
		{
			// Note: "who is on" matches the Roster phrase; use the explicit
			// "injury report" phrasing to get SportsIntentInjuries.
			query:      "Los Angeles Lakers injury report",
			wantIntent: SportsIntentInjuries,
			wantLeague: espn.LeagueNBA,
			wantTeam:   "Los Angeles Lakers",
		},
		// ── Transactions ─────────────────────────────────────────────────────
		{
			query:      "What recent transactions have the Brewers made?",
			wantIntent: SportsIntentTransactions,
			wantLeague: espn.LeagueMLB,
			wantTeam:   "Milwaukee Brewers",
		},
		// ── TeamSchedule with explicit season year ───────────────────────────
		{
			query:      "What was the 2024 Chiefs schedule?",
			wantIntent: SportsIntentTeamSchedule,
			wantLeague: espn.LeagueNFL,
			wantTeam:   "Kansas City Chiefs",
			wantSeason: 2024,
		},
		// ── Leaders with explicit top-N limit ────────────────────────────────
		{
			query:      "Show me the top 50 NFL receiving leaders this season.",
			wantIntent: SportsIntentLeaders,
			wantLeague: espn.LeagueNFL,
			wantLimit:  50,
		},
		// ── News ─────────────────────────────────────────────────────────────
		{
			query:      "What are the latest LA Kings news headlines?",
			wantIntent: SportsIntentNews,
			wantLeague: espn.LeagueNHL,
			wantTeam:   "Los Angeles Kings",
		},
		{
			query:      "What is the latest news for the LA Kings?",
			wantIntent: SportsIntentNews,
			wantLeague: espn.LeagueNHL,
			wantTeam:   "Los Angeles Kings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) = nil, false; want true", tt.query)
			}
			if got.Intent != tt.wantIntent {
				t.Errorf("intent = %q, want %q", got.Intent, tt.wantIntent)
			}
			if tt.wantLeague != "" && got.League != tt.wantLeague {
				t.Errorf("league = %q, want %q", got.League, tt.wantLeague)
			}
			if tt.wantTeam != "" && got.TeamQuery != tt.wantTeam {
				t.Errorf("teamQuery = %q, want %q", got.TeamQuery, tt.wantTeam)
			}
			if tt.wantDateLabel != "" && got.DateLabel != tt.wantDateLabel {
				t.Errorf("dateLabel = %q, want %q", got.DateLabel, tt.wantDateLabel)
			}
			if tt.wantSeason != 0 && got.Season != tt.wantSeason {
				t.Errorf("season = %d, want %d", got.Season, tt.wantSeason)
			}
			if tt.wantLimit != 0 && got.Limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", got.Limit, tt.wantLimit)
			}
		})
	}
}

// TestDetectAthleteIntentExtended covers athlete-specific queries where the
// expected AthleteQuery is validated with a substring check.
func TestDetectAthleteIntentExtended(t *testing.T) {
	tests := []struct {
		query               string
		wantIntent          SportsIntentType
		wantAthleteContains string
		wantSeason          int
	}{
		{
			query:               "Show me Connor McDavid's current season stats.",
			wantIntent:          SportsIntentAthleteStats,
			wantAthleteContains: "connor mcdavid",
		},
		{
			query:               "What was LeBron James' gamelog for the 2024 season?",
			wantIntent:          SportsIntentAthleteStats,
			wantAthleteContains: "lebron james",
			wantSeason:          2024,
		},
		// Fix 4: no-league news query falls back to AthleteNews when an athlete
		// name can be extracted.
		{
			query:               "Show me Caitlin Clark's latest news.",
			wantIntent:          SportsIntentAthleteNews,
			wantAthleteContains: "caitlin clark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) = nil, false; want true", tt.query)
			}
			if got.Intent != tt.wantIntent {
				t.Errorf("intent = %q, want %q", got.Intent, tt.wantIntent)
			}
			if !strings.Contains(got.AthleteQuery, tt.wantAthleteContains) {
				t.Errorf("athleteQuery = %q; want it to contain %q", got.AthleteQuery, tt.wantAthleteContains)
			}
			if tt.wantSeason != 0 && got.Season != tt.wantSeason {
				t.Errorf("season = %d, want %d", got.Season, tt.wantSeason)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectStatMetricBoundaries: verifies that each major statistical alias
// resolves to the correct league, StatName, and sort key.
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectStatMetricBoundaries(t *testing.T) {
	tests := []struct {
		query        string
		wantLeague   string
		wantStatName string
		wantSort     string
	}{
		{"who leads the NBA in ppg?", espn.LeagueNBA, "avgPoints", "offensive.avgPoints:desc"},
		{"who leads the NBA in pts?", espn.LeagueNBA, "avgPoints", "offensive.avgPoints:desc"},
		{"NBA assists leaders", espn.LeagueNBA, "avgAssists", "offensive.avgAssists:desc"},
		{"NBA ast leaders", espn.LeagueNBA, "avgAssists", "offensive.avgAssists:desc"},
		{"NBA reb leaders", espn.LeagueNBA, "avgRebounds", "general.avgRebounds:desc"},
		// "hrs" plural abbreviation for home runs — was previously not matched
		{"who led the MLB in HRs in 1985?", espn.LeagueMLB, "homeRuns", "batting.homeRuns:desc"},
		{"MLB hrs leaders", espn.LeagueMLB, "homeRuns", "batting.homeRuns:desc"},
		{"NHL goals leaders", espn.LeagueNHL, "goals", "scoring.goals:desc"},
		// NHL has its own "assists" entry; hasLeague=NHL causes the NBA entry to
		// be skipped and the NHL entry to be returned.
		{"NHL assists leaders", espn.LeagueNHL, "assists", "scoring.assists:desc"},
		// Aliases use the plural form ("sacks", "interceptions").
		{"NFL sacks leaders", espn.LeagueNFL, "sacks", "defensive.sacks:desc"},
		{"NFL interceptions leaders", espn.LeagueNFL, "interceptions", "defensive.interceptions:desc"},
		{"who has the most saves in MLB?", espn.LeagueMLB, "saves", "pitching.saves:desc"},
		// No explicit league in query — DefaultLeague of ERA entry provides MLB.
		{"ERA leaders this season", espn.LeagueMLB, "ERA", "pitching.ERA:asc"},
		{"WHIP leaders", espn.LeagueMLB, "WHIP", "pitching.WHIP:asc"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) = nil, false; want true", tt.query)
			}
			if got.Intent != SportsIntentLeaders {
				t.Fatalf("intent = %q, want leaders", got.Intent)
			}
			if got.League != tt.wantLeague {
				t.Errorf("league = %q, want %q", got.League, tt.wantLeague)
			}
			if got.StatName != tt.wantStatName {
				t.Errorf("statName = %q, want %q", got.StatName, tt.wantStatName)
			}
			if got.StatSort != tt.wantSort {
				t.Errorf("statSort = %q, want %q", got.StatSort, tt.wantSort)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNormalizeRosterAdditional: nil/empty guards and grouped-JSON extraction.
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeRosterAdditional(t *testing.T) {
	t.Run("nil roster returns nil", func(t *testing.T) {
		rows := normalizeRoster(nil)
		if rows != nil {
			t.Fatalf("expected nil, got %v", rows)
		}
	})

	t.Run("empty athletes JSON returns no rows", func(t *testing.T) {
		roster := &espn.TeamRoster{Athletes: json.RawMessage(`[]`)}
		rows := normalizeRoster(roster)
		if len(rows) != 0 {
			t.Fatalf("expected 0 rows, got %d", len(rows))
		}
	})

	t.Run("single athlete in grouped format", func(t *testing.T) {
		athletes := json.RawMessage(`[{"position":"Forward","items":[` +
			`{"displayName":"Alex Ovechkin","jersey":"8","age":38,` +
			`"displayHeight":"6'3\"","displayWeight":"235 lb",` +
			`"position":{"abbreviation":"LW","displayName":"Left Wing"},` +
			`"status":{"abbreviation":"A","name":"Active"}}` +
			`]}]`)
		roster := &espn.TeamRoster{Athletes: athletes}
		rows := normalizeRoster(roster)
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		r := rows[0]
		if r.Group != "Forward" {
			t.Errorf("Group = %q, want Forward", r.Group)
		}
		if r.Name != "Alex Ovechkin" {
			t.Errorf("Name = %q, want Alex Ovechkin", r.Name)
		}
		if r.Jersey != "8" {
			t.Errorf("Jersey = %q, want 8", r.Jersey)
		}
		if r.Position != "LW" {
			t.Errorf("Position = %q, want LW", r.Position)
		}
		if r.Age != "38" {
			t.Errorf("Age = %q, want 38", r.Age)
		}
		if r.Status != "A" {
			t.Errorf("Status = %q, want A", r.Status)
		}
	})

	t.Run("multiple groups produce correct ordering", func(t *testing.T) {
		athletes := json.RawMessage(`[` +
			`{"position":"Guard","items":[{"displayName":"Steph Curry","jersey":"30","age":36}]},` +
			`{"position":"Forward","items":[{"displayName":"Draymond Green","jersey":"23","age":34}]}` +
			`]`)
		roster := &espn.TeamRoster{Athletes: athletes}
		rows := normalizeRoster(roster)
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
		if rows[0].Group != "Guard" || rows[0].Name != "Steph Curry" {
			t.Errorf("row[0] = %+v; want Group=Guard, Name=Steph Curry", rows[0])
		}
		if rows[1].Group != "Forward" || rows[1].Name != "Draymond Green" {
			t.Errorf("row[1] = %+v; want Group=Forward, Name=Draymond Green", rows[1])
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNormalizeLeaderboardExtra: additional normalizeLeaderboard payloads for
// NFL passing yards and NHL goals, plus edge-case inputs.
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeLeaderboardExtra(t *testing.T) {
	t.Run("NFL passing yards", func(t *testing.T) {
		raw := []byte(`{
			"requestedSeason": {"year": 2024, "displayName": "2024", "type": {"name": "Regular Season"}},
			"categories": [{"name": "passing", "labels": ["CMP", "YDS", "TD"], "names": ["completions", "passingYards", "passingTouchdowns"]}],
			"athletes": [
				{
					"athlete": {"displayName": "Patrick Mahomes", "teamShortName": "KC", "position": {"abbreviation": "QB"}},
					"categories": [{"name": "passing", "totals": ["415", "4183", "26"], "ranks": ["3", "1", "2"]}]
				},
				{
					"athlete": {"displayName": "Joe Burrow", "teamShortName": "CIN", "position": {"abbreviation": "QB"}},
					"categories": [{"name": "passing", "totals": ["375", "3895", "24"], "ranks": ["8", "2", "4"]}]
				}
			]
		}`)
		req := SportsRequest{StatCategory: "passing", StatName: "passingYards", StatLabel: "YDS"}
		rows, label, season := normalizeLeaderboard(raw, req)
		if label != "YDS" {
			t.Errorf("label = %q, want YDS", label)
		}
		if season != "2024 Regular Season" {
			t.Errorf("season = %q, want \"2024 Regular Season\"", season)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2", len(rows))
		}
		// Rank comes from ranks[statIdx]; statIdx for "passingYards" (index 1) → ranks[1]="1"
		if rows[0].Rank != 1 || rows[0].Athlete != "Patrick Mahomes" || rows[0].Value != "4183" {
			t.Errorf("row[0] = %+v; want Rank=1, Athlete=Patrick Mahomes, Value=4183", rows[0])
		}
		if rows[1].Rank != 2 || rows[1].Athlete != "Joe Burrow" {
			t.Errorf("row[1] = %+v; want Rank=2, Athlete=Joe Burrow", rows[1])
		}
	})

	t.Run("NHL goals", func(t *testing.T) {
		raw := []byte(`{
			"requestedSeason": {"year": 2025, "displayName": "2025", "type": {"name": "Regular Season"}},
			"categories": [{"name": "scoring", "labels": ["G", "A", "PTS"], "names": ["goals", "assists", "points"]}],
			"athletes": [
				{
					"athlete": {"displayName": "Auston Matthews", "teamShortName": "TOR", "position": {"abbreviation": "C"}},
					"categories": [{"name": "scoring", "totals": ["69", "37", "106"], "ranks": ["1", "5", "3"]}]
				}
			]
		}`)
		req := SportsRequest{StatCategory: "scoring", StatName: "goals", StatLabel: "G"}
		rows, label, _ := normalizeLeaderboard(raw, req)
		if label != "G" {
			t.Errorf("label = %q, want G", label)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		if rows[0].Rank != 1 || rows[0].Athlete != "Auston Matthews" || rows[0].Value != "69" {
			t.Errorf("row[0] = %+v; want Rank=1, Athlete=Auston Matthews, Value=69", rows[0])
		}
	})

	t.Run("empty payload returns no rows", func(t *testing.T) {
		raw := []byte(`{}`)
		req := SportsRequest{StatCategory: "batting", StatName: "homeRuns", StatLabel: "HR"}
		rows, _, _ := normalizeLeaderboard(raw, req)
		if len(rows) != 0 {
			t.Fatalf("expected 0 rows for empty payload, got %d", len(rows))
		}
	})

	t.Run("invalid JSON returns empty result with original label", func(t *testing.T) {
		raw := []byte(`not-json`)
		req := SportsRequest{StatCategory: "batting", StatName: "homeRuns", StatLabel: "HR"}
		rows, label, _ := normalizeLeaderboard(raw, req)
		if len(rows) != 0 {
			t.Fatalf("expected 0 rows for invalid JSON, got %d", len(rows))
		}
		if label != "HR" {
			t.Errorf("label = %q; want original StatLabel %q on parse error", label, "HR")
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// TestFilterNewsByTeam: validates team-name filtering of NewsRow slices.
// ─────────────────────────────────────────────────────────────────────────────

func TestFilterNewsByTeam(t *testing.T) {
	rows := []NewsRow{
		{Headline: "LA Kings crush Ducks 5-1", Description: "The Kings dominated all game."},
		{Headline: "Maple Leafs edge Bruins", Description: "A close game last night."},
		{Headline: "Kings forward injured", Description: "Los Angeles Kings forward misses practice."},
	}

	t.Run("empty team name returns all rows unchanged", func(t *testing.T) {
		out := filterNewsByTeam(rows, "")
		if len(out) != len(rows) {
			t.Fatalf("expected %d rows, got %d", len(rows), len(out))
		}
	})

	t.Run("nil rows returns nil", func(t *testing.T) {
		out := filterNewsByTeam(nil, "LA Kings")
		if out != nil {
			t.Fatalf("expected nil, got %v", out)
		}
	})

	t.Run("headline match included", func(t *testing.T) {
		out := filterNewsByTeam(rows, "LA Kings")
		found := false
		for _, r := range out {
			if strings.Contains(r.Headline, "LA Kings") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected headline match for LA Kings; got %v", out)
		}
	})

	t.Run("description match included", func(t *testing.T) {
		out := filterNewsByTeam(rows, "Los Angeles Kings")
		found := false
		for _, r := range out {
			if strings.Contains(r.Description, "Los Angeles Kings") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected description match for Los Angeles Kings; got %v", out)
		}
	})

	t.Run("non-matching team returns empty slice", func(t *testing.T) {
		out := filterNewsByTeam(rows, "Edmonton Oilers")
		if len(out) != 0 {
			t.Fatalf("expected 0 rows for non-matching team, got %d", len(out))
		}
	})

	t.Run("filter is case-insensitive via normalizeText", func(t *testing.T) {
		// filterNewsByTeam calls normalizeText on both the team name and each row,
		// so "LA KINGS" normalises to "la kings" and still matches.
		out := filterNewsByTeam(rows, "LA KINGS")
		if len(out) == 0 {
			t.Fatal("expected matches for case-insensitive 'LA KINGS'")
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectSportsIntentNegative: queries that must return ok=false.
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectSportsIntentNegative(t *testing.T) {
	queries := []string{
		// empty input
		"",
		// league detected but Unknown intent with no temporal phrase
		"what's a good fantasy football team?",
		// isNonLookupQuery: "short story"
		"write a short story about baseball",
		// league detected (Cubs → MLB) but Unknown intent with no temporal phrase
		"What year did the Cubs last win the World Series?",
		// two leagues detected; Unknown intent; no temporal phrase
		"compare the NBA and NFL offseason",
		// Generic current-news topics must continue through web search / the LLM,
		// never be treated as athlete names by the ESPN fallback.
		"What's the latest tech news?",
		"What's the latest news in politics?",
		"Give me the latest AI headlines",
		"What is new in cybersecurity?",
		"latest business news",
		"latest entertainment headlines",
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			got, ok := DetectSportsIntent(q, fixedNow())
			if ok {
				t.Errorf("DetectSportsIntent(%q) = %+v, true; want false", q, got)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderRosterMarkdown: Markdown output validation.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderRosterMarkdown(t *testing.T) {
	now := time.Date(2026, 5, 7, 18, 0, 0, 0, time.UTC)
	nhlCfg := LeagueConfig{DisplayName: "NHL"}

	t.Run("title uses TeamQuery when set", func(t *testing.T) {
		req := SportsRequest{TeamQuery: "Los Angeles Kings"}
		rows := []RosterRow{
			{Group: "Forward", Name: "Anze Kopitar", Position: "C", Jersey: "11", Age: "37", Height: "6'3\"", Weight: "225 lb", Status: "A"},
		}
		out := RenderRosterMarkdown(req, nhlCfg, rows, now)
		if !strings.Contains(out, "### Los Angeles Kings Roster") {
			t.Errorf("missing title; got:\n%s", out)
		}
	})

	t.Run("title falls back to cfg.DisplayName when TeamQuery is empty", func(t *testing.T) {
		req := SportsRequest{}
		out := RenderRosterMarkdown(req, nhlCfg, nil, now)
		if !strings.Contains(out, "### NHL Roster") {
			t.Errorf("expected '### NHL Roster'; got:\n%s", out)
		}
	})

	t.Run("output contains expected headers and row data", func(t *testing.T) {
		req := SportsRequest{TeamQuery: "Los Angeles Kings"}
		rows := []RosterRow{
			{Group: "Defense", Name: "Drew Doughty", Position: "D", Jersey: "8", Age: "34", Height: "6'1\"", Weight: "218 lb", Status: "A"},
		}
		out := RenderRosterMarkdown(req, nhlCfg, rows, now)
		for _, want := range []string{"Player", "Pos", "Drew Doughty", "Defense"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in output:\n%s", want, out)
			}
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderLeaderboardMarkdown: Markdown output validation.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderLeaderboardMarkdown(t *testing.T) {
	now := time.Date(2026, 5, 7, 18, 0, 0, 0, time.UTC)
	nbaCfg := LeagueConfig{DisplayName: "NBA"}

	t.Run("title and stat column present", func(t *testing.T) {
		req := SportsRequest{StatLabel: "PTS"}
		rows := []LeaderboardRow{
			{Rank: 1, Athlete: "Shai Gilgeous-Alexander", Team: "OKC", Position: "G", Value: "32.7"},
		}
		out := RenderLeaderboardMarkdown(req, nbaCfg, rows, now)
		if !strings.Contains(out, "### NBA PTS Leaders") {
			t.Errorf("missing title; got:\n%s", out)
		}
		if !strings.Contains(out, "Shai Gilgeous-Alexander") {
			t.Errorf("missing athlete name; got:\n%s", out)
		}
		if !strings.Contains(out, "32.7") {
			t.Errorf("missing stat value; got:\n%s", out)
		}
	})

	t.Run("season year appended to title", func(t *testing.T) {
		req := SportsRequest{StatLabel: "HR", Season: 2024}
		rows := []LeaderboardRow{{Rank: 1, Athlete: "Cal Raleigh", Team: "SEA", Value: "60"}}
		mlbCfg := LeagueConfig{DisplayName: "MLB"}
		out := RenderLeaderboardMarkdown(req, mlbCfg, rows, now)
		if !strings.Contains(out, "— 2024") {
			t.Errorf("missing season in title; got:\n%s", out)
		}
	})

	t.Run("date label appended to title and takes precedence over season", func(t *testing.T) {
		req := SportsRequest{StatLabel: "PTS", DateLabel: "Today", Season: 2024}
		rows := []LeaderboardRow{{Rank: 1, Athlete: "Player", Team: "TM", Value: "28.0"}}
		out := RenderLeaderboardMarkdown(req, nbaCfg, rows, now)
		if !strings.Contains(out, "— Today") {
			t.Errorf("missing date label in title; got:\n%s", out)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRenderSimpleMarkdown: verifies title, headers, and row data appear in
// the rendered output.
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderSimpleMarkdown(t *testing.T) {
	now := time.Date(2026, 5, 7, 18, 0, 0, 0, time.UTC)
	title := "### NFL Standings"
	table := SimpleTable{
		Headers: []string{"Team", "W", "L"},
		Rows: [][]string{
			{"Kansas City Chiefs", "15", "2"},
			{"Baltimore Ravens", "13", "4"},
		},
	}

	out := RenderSimpleMarkdown(title, table, now)

	for _, want := range []string{title, "Team", "W", "Kansas City Chiefs", "Baltimore Ravens"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestParseDateValueExtended — Group 2: extended date / temporal phrase cases.
// Tests that ParseDateValue handles "last night", "this weekend", "this week",
// and named weekdays without panicking and returns the expected date + label.
// All dates are relative to fixedNow() = 2026-05-07 (Thursday).
// ─────────────────────────────────────────────────────────────────────────────

func TestParseDateValueExtended(t *testing.T) {
	now := fixedNow() // 2026-05-07 Thu

	tests := []struct {
		phrase    string
		wantDate  string // "" means nil date is expected
		wantLabel string
		wantErr   bool
	}{
		// "last night" → yesterday
		{"last night", "2026-05-06", "Yesterday", false},
		// "this weekend" → next Saturday (2 days ahead from Thursday)
		{"this weekend", "2026-05-09", "This Weekend", false},
		// "this week" → no anchor date, but label set
		{"this week", "", "This Week", false},
		// Named weekday: Monday night → next Monday (4 days ahead)
		{"Monday night", "2026-05-11", "Mon May 11", false},
		// Named weekday: Friday → next Friday (1 day ahead from Thursday)
		{"Friday", "2026-05-08", "Fri May 8", false},
		// Named weekday: Thursday (same weekday as now) → next Thursday (+7)
		{"Thursday", "2026-05-14", "Thu May 14", false},
		// "2025 season" and "Week 1" are not date values — must not panic; error OK
		{"2025 season", "", "", true},
		{"Week 1 of the 2025 regular season", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.phrase, func(t *testing.T) {
			got, label, err := ParseDateValue(tt.phrase, now, SportsIntentSchedule)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDateValue(%q) err = nil, want non-nil", tt.phrase)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDateValue(%q) unexpected err: %v", tt.phrase, err)
			}
			if label != tt.wantLabel {
				t.Errorf("label = %q, want %q", label, tt.wantLabel)
			}
			if tt.wantDate == "" {
				if got != nil {
					t.Errorf("date = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("date = nil, want %s", tt.wantDate)
			}
			if got.Format("2006-01-02") != tt.wantDate {
				t.Errorf("date = %s, want %s", got.Format("2006-01-02"), tt.wantDate)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNormalizeLeaderboardNBAAndPitching — Group 5 additions: NBA avgPoints
// and MLB pitching ERA not previously covered.
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeLeaderboardNBAAndPitching(t *testing.T) {
	t.Run("NBA offensive avgPoints", func(t *testing.T) {
		raw := []byte(`{
			"requestedSeason": {"year": 2025, "displayName": "2025", "type": {"name": "Regular Season"}},
			"categories": [{"name": "offensive", "labels": ["PTS", "REB", "AST"], "names": ["avgPoints", "avgRebounds", "avgAssists"]}],
			"athletes": [
				{
					"athlete": {"displayName": "Shai Gilgeous-Alexander", "teamShortName": "OKC", "position": {"abbreviation": "G"}},
					"categories": [{"name": "offensive", "totals": ["32.7", "5.1", "6.4"], "ranks": ["1", "22", "8"]}]
				},
				{
					"athlete": {"displayName": "Giannis Antetokounmpo", "teamShortName": "MIL", "position": {"abbreviation": "F"}},
					"categories": [{"name": "offensive", "totals": ["30.4", "11.9", "6.5"], "ranks": ["2", "3", "7"]}]
				}
			]
		}`)
		req := SportsRequest{StatCategory: "offensive", StatName: "avgPoints", StatLabel: "PTS"}
		rows, label, season := normalizeLeaderboard(raw, req)
		if label != "PTS" {
			t.Errorf("label = %q, want PTS", label)
		}
		if season != "2025 Regular Season" {
			t.Errorf("season = %q, want \"2025 Regular Season\"", season)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2", len(rows))
		}
		if rows[0].Rank != 1 || rows[0].Athlete != "Shai Gilgeous-Alexander" || rows[0].Value != "32.7" {
			t.Errorf("row[0] = %+v; want Rank=1, Athlete=Shai Gilgeous-Alexander, Value=32.7", rows[0])
		}
		if rows[1].Rank != 2 || rows[1].Athlete != "Giannis Antetokounmpo" {
			t.Errorf("row[1] = %+v; want Rank=2, Athlete=Giannis Antetokounmpo", rows[1])
		}
	})

	t.Run("MLB pitching ERA", func(t *testing.T) {
		raw := []byte(`{
			"requestedSeason": {"year": 2025, "displayName": "2025", "type": {"name": "Regular Season"}},
			"categories": [{"name": "pitching", "labels": ["ERA", "W", "K"], "names": ["ERA", "wins", "strikeouts"]}],
			"athletes": [
				{
					"athlete": {"displayName": "Zack Wheeler", "teamShortName": "PHI", "position": {"abbreviation": "SP"}},
					"categories": [{"name": "pitching", "totals": ["2.21", "14", "189"], "ranks": ["1", "5", "8"]}]
				},
				{
					"athlete": {"displayName": "Framber Valdez", "teamShortName": "HOU", "position": {"abbreviation": "SP"}},
					"categories": [{"name": "pitching", "totals": ["2.48", "16", "171"], "ranks": ["2", "3", "12"]}]
				}
			]
		}`)
		req := SportsRequest{StatCategory: "pitching", StatName: "ERA", StatLabel: "ERA"}
		rows, label, _ := normalizeLeaderboard(raw, req)
		if label != "ERA" {
			t.Errorf("label = %q, want ERA", label)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2", len(rows))
		}
		// ERA is ascending — rank 1 has the lowest ERA (rank index 0)
		if rows[0].Rank != 1 || rows[0].Athlete != "Zack Wheeler" || rows[0].Value != "2.21" {
			t.Errorf("row[0] = %+v; want Rank=1, Athlete=Zack Wheeler, Value=2.21", rows[0])
		}
		if rows[1].Rank != 2 || rows[1].Athlete != "Framber Valdez" {
			t.Errorf("row[1] = %+v; want Rank=2, Athlete=Framber Valdez", rows[1])
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectSportsIntentSoccer — Group 7: soccer club aliases and new leagues
// (Champions League, La Liga, Bundesliga).
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectSportsIntentSoccer(t *testing.T) {
	tests := []struct {
		query      string
		wantIntent SportsIntentType
		wantLeague string
		wantTeam   string
	}{
		// EPL club aliases
		{
			query:      "What is Arsenal next match?",
			wantIntent: SportsIntentTeamSchedule,
			wantLeague: espn.LeagueEPL,
			wantTeam:   "Arsenal",
		},
		{
			query:      "Show me Liverpool scores today",
			wantIntent: SportsIntentScores,
			wantLeague: espn.LeagueEPL,
			wantTeam:   "Liverpool",
		},
		{
			query:      "Man City vs Man United today",
			wantIntent: SportsIntentScores,
			wantLeague: espn.LeagueEPL,
		},
		// Champions League
		{
			query:      "Champions League scores today",
			wantIntent: SportsIntentScores,
			wantLeague: espn.LeagueChampionsLg,
		},
		{
			query:      "UEFA Champions League standings",
			wantIntent: SportsIntentStandings,
			wantLeague: espn.LeagueChampionsLg,
		},
		// La Liga
		{
			query:      "Real Madrid next game",
			wantIntent: SportsIntentTeamSchedule,
			wantLeague: espn.LeagueLaLiga,
			wantTeam:   "Real Madrid",
		},
		{
			query:      "Barcelona scores today",
			wantIntent: SportsIntentScores,
			wantLeague: espn.LeagueLaLiga,
			wantTeam:   "FC Barcelona",
		},
		{
			query:      "La Liga standings",
			wantIntent: SportsIntentStandings,
			wantLeague: espn.LeagueLaLiga,
		},
		// Bundesliga
		{
			query:      "Bundesliga scores today",
			wantIntent: SportsIntentScores,
			wantLeague: espn.LeagueBundesliga,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) = nil, false; want true", tt.query)
			}
			if got.Intent != tt.wantIntent {
				t.Errorf("intent = %q, want %q", got.Intent, tt.wantIntent)
			}
			if tt.wantLeague != "" && got.League != tt.wantLeague {
				t.Errorf("league = %q, want %q", got.League, tt.wantLeague)
			}
			if tt.wantTeam != "" && got.TeamQuery != tt.wantTeam {
				t.Errorf("teamQuery = %q, want %q", got.TeamQuery, tt.wantTeam)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectSportsIntentNegativeExtended — Group 8 additions: NHL "history"
// query that should return false.
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectSportsIntentNegativeExtended(t *testing.T) {
	queries := []string{
		// Historical fact question
		"When did the Blackhawks last win the Stanley Cup",
		// Request to explain a concept — how it is calculated
		"explain how batting average is calculated",
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			got, ok := DetectSportsIntent(q, fixedNow())
			if ok {
				t.Errorf("DetectSportsIntent(%q) = %+v, true; want false", q, got)
			}
		})
	}
}

func TestDetectSportsIntentHistoricalLeaderSearchFallback(t *testing.T) {
	got, ok := DetectSportsIntent("Who has scored the most goals in NHL history", fixedNow())
	if !ok {
		t.Fatal("expected historical NHL leader query to route to ESPN search fallback")
	}
	if got.Intent != SportsIntentSearch {
		t.Fatalf("intent = %q, want %q", got.Intent, SportsIntentSearch)
	}
	if got.League != espn.LeagueNHL {
		t.Fatalf("league = %q, want %q", got.League, espn.LeagueNHL)
	}
}
