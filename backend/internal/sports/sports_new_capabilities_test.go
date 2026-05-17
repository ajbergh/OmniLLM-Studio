package sports

// sports_new_capabilities_test.go — unit tests for the extended ESPN
// capabilities introduced in Q10, Q46, Q52, Q53, Q58, Q62, Q63, Q68–Q76.
//
// All tests here are pure-logic: they exercise intent detection, helper
// functions, and normalization without making live ESPN API calls.

import (
	"encoding/json"
	"strings"
	"testing"

	espn "github.com/chinmaykhachane/espn-go"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectNewSportsIntents — verifies that each new intent type is correctly
// identified from representative queries (Q10, Q46, Q52, Q53, Q58, Q62, Q63,
// Q68–Q76 from the test plan).
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectNewSportsIntents(t *testing.T) {
	tests := []struct {
		query      string
		wantIntent SportsIntentType
		wantLeague string
		wantSeason int
	}{
		// ── Q10: ESPN Search ─────────────────────────────────────────────────
		{
			query:      "Search ESPN for Shohei Ohtani",
			wantIntent: SportsIntentSearch,
		},
		{
			query:      "ESPN search for LeBron James career stats",
			wantIntent: SportsIntentSearch,
		},
		// ── Q46: QBR ─────────────────────────────────────────────────────────
		{
			query:      "Show me Patrick Mahomes' QBR for 2023",
			wantIntent: SportsIntentQBR,
			wantLeague: espn.LeagueNFL,
			wantSeason: 2023,
		},
		{
			query:      "NFL quarterback rating leaders",
			wantIntent: SportsIntentQBR,
			wantLeague: espn.LeagueNFL,
		},
		{
			query:      "Total QBR rankings",
			wantIntent: SportsIntentQBR,
			wantLeague: espn.LeagueNFL,
		},
		// ── Q52: Athlete Comparison ───────────────────────────────────────────
		{
			query:      "Compare Nikola Jokic and Joel Embiid",
			wantIntent: SportsIntentAthleteComparison,
		},
		{
			query:      "Head to head: LeBron James vs Kevin Durant",
			wantIntent: SportsIntentAthleteComparison,
		},
		// ── Q53: Hot Zones ────────────────────────────────────────────────────
		{
			query:      "What are Mookie Betts' hot zones?",
			wantIntent: SportsIntentHotZones,
		},
		{
			query:      "Show me Aaron Judge hot zone data",
			wantIntent: SportsIntentHotZones,
		},
		// ── Q58: Win Probability ─────────────────────────────────────────────
		{
			query:      "What was the win probability in the last NFL Chiefs game?",
			wantIntent: SportsIntentGameDetail,
			wantLeague: espn.LeagueNFL,
		},
		// ── Q62: Predictor ────────────────────────────────────────────────────
		{
			query:      "What is ESPN's game predictor for the Cowboys next game?",
			wantIntent: SportsIntentGameDetail,
			wantLeague: espn.LeagueNFL,
		},
		// ── Q63: Officials ────────────────────────────────────────────────────
		{
			query:      "Who are the officials for the next NFL Patriots game?",
			wantIntent: SportsIntentGameDetail,
			wantLeague: espn.LeagueNFL,
		},
		// ── Q68: CDN Game Package ─────────────────────────────────────────────
		{
			query:      "Give me the full game package for the latest NFL Eagles game",
			wantIntent: SportsIntentGameDetail,
			wantLeague: espn.LeagueNFL,
		},
		// ── Q69–Q72: Champions history ────────────────────────────────────────
		{
			query:      "Who won the 2024 Super Bowl?",
			wantIntent: SportsIntentChampions,
			wantLeague: espn.LeagueNFL,
			wantSeason: 2024,
		},
		{
			query:      "Who was the NBA champion in 2023?",
			wantIntent: SportsIntentChampions,
			wantLeague: espn.LeagueNBA,
			wantSeason: 2023,
		},
		{
			query:      "Who won the Stanley Cup in 2022?",
			wantIntent: SportsIntentChampions,
			wantLeague: espn.LeagueNHL,
			wantSeason: 2022,
		},
		{
			query:      "Who won the 2023 World Series?",
			wantIntent: SportsIntentChampions,
			wantLeague: espn.LeagueMLB,
			wantSeason: 2023,
		},
		// ── Q73–Q74: Draft ────────────────────────────────────────────────────
		{
			query:      "Show me the 2024 NFL draft results",
			wantIntent: SportsIntentDraft,
			wantLeague: espn.LeagueNFL,
			wantSeason: 2024,
		},
		{
			query:      "NBA draft picks 2023",
			wantIntent: SportsIntentDraft,
			wantLeague: espn.LeagueNBA,
			wantSeason: 2023,
		},
		// ── Q75–Q76: Coaches ──────────────────────────────────────────────────
		{
			query:      "Who is the head coach of the Kansas City Chiefs?",
			wantIntent: SportsIntentCoaches,
			wantLeague: espn.LeagueNFL,
		},
		{
			query:      "Show me all NFL coaching staff",
			wantIntent: SportsIntentCoaches,
			wantLeague: espn.LeagueNFL,
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
			if tt.wantSeason != 0 && got.Season != tt.wantSeason {
				t.Errorf("season = %d, want %d", got.Season, tt.wantSeason)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestExtractTwoAthletes — unit tests for the extractTwoAthletes helper.
// ─────────────────────────────────────────────────────────────────────────────

func TestExtractTwoAthletes(t *testing.T) {
	tests := []struct {
		raw    string
		first  string
		second string
	}{
		{
			raw:    "Compare Nikola Jokic and Joel Embiid",
			first:  "Nikola Jokic",
			second: "Joel Embiid",
		},
		{
			raw:    "LeBron James vs Kevin Durant",
			first:  "LeBron James",
			second: "Kevin Durant",
		},
		{
			raw:    "Head to head: Patrick Mahomes and Josh Allen",
			first:  "Patrick Mahomes",
			second: "Josh Allen",
		},
		{
			raw:    "Compare Aaron Judge versus Mike Trout",
			first:  "Aaron Judge",
			second: "Mike Trout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			first, second := extractTwoAthletes(tt.raw)
			if !strings.EqualFold(first, tt.first) {
				t.Errorf("first = %q, want %q", first, tt.first)
			}
			if !strings.EqualFold(second, tt.second) {
				t.Errorf("second = %q, want %q", second, tt.second)
			}
		})
	}
}

// TestExtractTwoAthletesFallback ensures extractTwoAthletes returns ("","")
// when no separator can be found.
func TestExtractTwoAthletesFallback(t *testing.T) {
	first, second := extractTwoAthletes("No separator here at all")
	if first != "" || second != "" {
		t.Errorf("expected (\"\",\"\"), got (%q,%q)", first, second)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAthleteComparisonDetectionFields — verifies that AthleteQuery and
// SecondAthleteQuery are both populated for comparison queries.
// ─────────────────────────────────────────────────────────────────────────────

func TestAthleteComparisonDetectionFields(t *testing.T) {
	req, ok := DetectSportsIntent("Compare Nikola Jokic and Joel Embiid", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for comparison query")
	}
	if req.Intent != SportsIntentAthleteComparison {
		t.Errorf("intent = %q, want athlete_comparison", req.Intent)
	}
	if !strings.Contains(strings.ToLower(req.AthleteQuery), "jokic") {
		t.Errorf("AthleteQuery = %q; want it to contain 'jokic'", req.AthleteQuery)
	}
	if !strings.Contains(strings.ToLower(req.SecondAthleteQuery), "embiid") {
		t.Errorf("SecondAthleteQuery = %q; want it to contain 'embiid'", req.SecondAthleteQuery)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGameDetailSubtypes — verifies detectGameDetailSubtype produces the right
// subtype string for each Q58/Q62/Q63/Q68 variant.
// ─────────────────────────────────────────────────────────────────────────────

func TestGameDetailSubtypes(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"win probability in the Chiefs game", "probabilities"},
		{"win prob for the Bills game tonight", "probabilities"},
		{"who are the officials for the Super Bowl", "officials"},
		{"who are the referees for tonight's NBA game", "officials"},
		{"espn predictor for the Cowboys game", "predictor"},
		{"game predictor for the Lakers matchup", "predictor"},
		{"full game package for the Eagles game", "gamepackage"},
		{"game package data for the Patriots", "gamepackage"},
		{"something else entirely", "summary"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := detectGameDetailSubtype(normalizeText(tt.query))
			if got != tt.want {
				t.Errorf("subtype = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGameDetailSubtypeInRequest — verifies that the GameDetailSubtype field
// is set when DetectSportsIntent returns a SportsIntentGameDetail request.
func TestGameDetailSubtypeInRequest(t *testing.T) {
	tests := []struct {
		query       string
		wantSubtype string
	}{
		{
			"What was the win probability in the last NFL Chiefs game?",
			"probabilities",
		},
		{
			"Who are the officials for the next NFL Patriots game?",
			"officials",
		},
		{
			"What is ESPN's game predictor for the Cowboys next game?",
			"predictor",
		},
		{
			"Give me the full game package for the latest NFL Eagles game",
			"gamepackage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent(%q) = nil, false; want true", tt.query)
			}
			if req.Intent != SportsIntentGameDetail {
				t.Fatalf("intent = %q, want game_detail", req.Intent)
			}
			if req.GameDetailSubtype != tt.wantSubtype {
				t.Errorf("GameDetailSubtype = %q, want %q", req.GameDetailSubtype, tt.wantSubtype)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDetectLeagueFromChampionship — unit tests for the championship term →
// league mapping helper.
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectLeagueFromChampionship(t *testing.T) {
	tests := []struct {
		phrase     string
		wantLeague string
	}{
		{"super bowl winner", espn.LeagueNFL},
		{"nba champion", espn.LeagueNBA},
		{"nba finals", espn.LeagueNBA},
		{"stanley cup winner", espn.LeagueNHL},
		{"world series champion", espn.LeagueMLB},
		{"mlb champion", espn.LeagueMLB},
	}

	for _, tt := range tests {
		t.Run(tt.phrase, func(t *testing.T) {
			cfg, ok := detectLeagueFromChampionship(normalizeText(tt.phrase))
			if !ok {
				t.Fatalf("detectLeagueFromChampionship(%q) returned false", tt.phrase)
			}
			if cfg.League != tt.wantLeague {
				t.Errorf("league = %q, want %q", cfg.League, tt.wantLeague)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIsNonLookupQueryChampionExemption — verifies that champion-related
// queries are NOT filtered by isNonLookupQuery even when they contain "history"
// or similar trigger words.
// ─────────────────────────────────────────────────────────────────────────────

func TestIsNonLookupQueryChampionExemption(t *testing.T) {
	queries := []string{
		"who won the super bowl",
		"world series champion history",
		"nba championship history",
		"stanley cup winner in history",
	}
	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			if isNonLookupQuery(normalizeText(q)) {
				t.Errorf("isNonLookupQuery(%q) = true; expected false (champion exemption)", q)
			}
		})
	}
}

// TestIsNonLookupQueryStillBlocks ensures that non-champion "history" queries
// remain blocked.
func TestIsNonLookupQueryStillBlocks(t *testing.T) {
	queries := []string{
		"who has the all time record for home runs",
		"nba history",
		"sports history",
	}
	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			if !isNonLookupQuery(normalizeText(q)) {
				t.Errorf("isNonLookupQuery(%q) = false; expected true", q)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNormalizeSearchEntitiesExtended — verifies normalizeSearchEntities
// parses a representative ESPN search response JSON fixture.
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeSearchEntitiesExtended(t *testing.T) {
	fixture := json.RawMessage(`{
		"results": [
			{
				"type": "player",
				"contents": [
					{
						"id": "3908809",
						"uid": "s:1~l:10~a:3908809",
						"type": "player",
						"displayName": "Shohei Ohtani",
						"description": "DH • Los Angeles Dodgers",
						"subtitle": "Los Angeles Dodgers",
						"defaultLeagueSlug": "mlb",
						"sport": "baseball",
						"link": { "web": "https://www.espn.com/mlb/player/_/id/3908809/shohei-ohtani" }
					}
				]
			},
			{
				"type": "team",
				"contents": [
					{
						"id": "19",
						"uid": "s:1~l:10~t:19",
						"type": "team",
						"displayName": "Los Angeles Dodgers",
						"subtitle": "",
						"defaultLeagueSlug": "mlb",
						"sport": "baseball",
						"link": { "web": "https://www.espn.com/mlb/team/_/name/lad/los-angeles-dodgers" }
					}
				]
			}
		]
	}`)

	entities := normalizeSearchEntities(fixture, "")
	if len(entities) != 2 {
		t.Fatalf("got %d entities, want 2", len(entities))
	}
	player := entities[0]
	if player.Name != "Shohei Ohtani" {
		t.Errorf("name = %q, want 'Shohei Ohtani'", player.Name)
	}
	if player.League != "mlb" {
		t.Errorf("league = %q, want 'mlb'", player.League)
	}
	// ID should be extracted from UID "s:1~l:10~a:3908809"
	if player.ID != "3908809" {
		t.Errorf("id = %q, want '3908809'", player.ID)
	}

	team := entities[1]
	if team.Name != "Los Angeles Dodgers" {
		t.Errorf("team name = %q, want 'Los Angeles Dodgers'", team.Name)
	}
}

// TestNormalizeSearchEntitiesPlayerFilter — verifies that filtering by type
// works correctly.
func TestNormalizeSearchEntitiesPlayerFilter(t *testing.T) {
	fixture := json.RawMessage(`{
		"results": [
			{
				"type": "player",
				"contents": [
					{
						"id": "1",
						"uid": "s:1~l:10~a:1",
						"type": "player",
						"displayName": "Test Player",
						"defaultLeagueSlug": "mlb",
						"sport": "baseball",
						"link": {}
					}
				]
			},
			{
				"type": "team",
				"contents": [
					{
						"id": "2",
						"uid": "s:1~l:10~t:2",
						"type": "team",
						"displayName": "Test Team",
						"defaultLeagueSlug": "mlb",
						"sport": "baseball",
						"link": {}
					}
				]
			}
		]
	}`)

	// Filter to players only
	players := normalizeSearchEntities(fixture, "player")
	if len(players) != 1 {
		t.Fatalf("got %d entities with player filter, want 1", len(players))
	}
	if players[0].Name != "Test Player" {
		t.Errorf("name = %q, want 'Test Player'", players[0].Name)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNormalizeChampionData — verifies that postseason scoreboard data is
// correctly parsed into winner/loser rows.
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeChampionData(t *testing.T) {
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			{
				ID:   "401547417",
				Name: "Super Bowl LVIII: San Francisco 49ers at Kansas City Chiefs",
				Status: espn.Status{
					Type: espn.StatusType{Completed: true},
				},
				Competitions: []espn.Competition{
					{
						ID:   "401547417",
						Date: "2024-02-11T23:30:00Z",
						Competitors: []espn.Competitor{
							{
								ID:       "1",
								HomeAway: "home",
								Winner:   true,
								Score:    "25",
								Team: espn.Team{
									DisplayName: "Kansas City Chiefs",
								},
							},
							{
								ID:       "2",
								HomeAway: "away",
								Winner:   false,
								Score:    "22",
								Team: espn.Team{
									DisplayName: "San Francisco 49ers",
								},
							},
						},
					},
				},
			},
		},
	}

	cfg := LeagueConfig{DisplayName: "NFL", Sport: "football", League: "nfl"}
	rows := normalizeChampionData(sb, cfg)

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row := rows[0]
	// row = [game name, date, winner, score, loser]
	if len(row) != 5 {
		t.Fatalf("row has %d cells, want 5: %v", len(row), row)
	}
	if row[2] != "Kansas City Chiefs" {
		t.Errorf("winner = %q, want 'Kansas City Chiefs'", row[2])
	}
	if row[4] != "San Francisco 49ers" {
		t.Errorf("loser = %q, want 'San Francisco 49ers'", row[4])
	}
	if row[3] != "25-22" {
		t.Errorf("score = %q, want '25-22'", row[3])
	}
	if row[1] != "2024-02-11" {
		t.Errorf("date = %q, want '2024-02-11'", row[1])
	}
}

// TestNormalizeChampionDataIncomplete — incomplete (not finished) games are
// excluded from the output.
func TestNormalizeChampionDataIncomplete(t *testing.T) {
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			{
				ID:   "401547417",
				Name: "Upcoming Playoff Game",
				Status: espn.Status{
					Type: espn.StatusType{Completed: false},
				},
				Competitions: []espn.Competition{
					{
						Competitors: []espn.Competitor{
							{Winner: false, Team: espn.Team{DisplayName: "Team A"}},
							{Winner: false, Team: espn.Team{DisplayName: "Team B"}},
						},
					},
				},
			},
		},
	}

	cfg := LeagueConfig{DisplayName: "NFL", Sport: "football", League: "nfl"}
	rows := normalizeChampionData(sb, cfg)
	if len(rows) != 0 {
		t.Errorf("got %d rows for incomplete game, want 0", len(rows))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestFilterTableByAthlete — verifies filterTableByAthlete correctly narrows
// a table to rows matching an athlete name fragment.
// ─────────────────────────────────────────────────────────────────────────────

func TestFilterTableByAthlete(t *testing.T) {
	table := SimpleTable{
		Headers: []string{"Name", "QBR"},
		Rows: [][]string{
			{"Patrick Mahomes", "82.5"},
			{"Josh Allen", "78.1"},
			{"Lamar Jackson", "88.2"},
		},
	}

	filtered := filterTableByAthlete(table, "mahomes")
	if len(filtered.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(filtered.Rows))
	}
	if filtered.Rows[0][0] != "Patrick Mahomes" {
		t.Errorf("row name = %q, want 'Patrick Mahomes'", filtered.Rows[0][0])
	}
}

func TestFilterTableByAthleteNoMatch(t *testing.T) {
	table := SimpleTable{
		Headers: []string{"Name", "QBR"},
		Rows:    [][]string{{"Patrick Mahomes", "82.5"}},
	}
	filtered := filterTableByAthlete(table, "rodgers")
	if len(filtered.Rows) != 0 {
		t.Errorf("expected no rows for 'rodgers', got %d", len(filtered.Rows))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestExtractSearchQuery — verifies extractSearchQuery strips ESPN search
// trigger phrases and returns the clean search term.
// ─────────────────────────────────────────────────────────────────────────────

func TestExtractSearchQuery(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"Search ESPN for Shohei Ohtani", "Shohei Ohtani"},
		{"search espn LeBron James", "LeBron James"},
		{"ESPN search for Patrick Mahomes", "Patrick Mahomes"},
		{"find on espn Mike Trout", "Mike Trout"},
		// Raw query without a trigger prefix is returned as-is
		{"Just a plain query", "Just a plain query"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := extractSearchQuery(tt.raw)
			if !strings.EqualFold(got, tt.want) {
				t.Errorf("extractSearchQuery(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCdnSportSlug — verifies the CDN sport slug mapping.
// ─────────────────────────────────────────────────────────────────────────────

func TestCdnSportSlug(t *testing.T) {
	tests := []struct {
		league   string
		wantSlug string
	}{
		{espn.LeagueNFL, "nfl"},
		{espn.LeagueNBA, "nba"},
		{espn.LeagueMLB, "mlb"},
		{espn.LeagueNHL, "nhl"},
		{espn.LeagueCollegeFootball, "college-football"},
	}

	for _, tt := range tests {
		t.Run(tt.league, func(t *testing.T) {
			cfg, ok := leagueConfigByLeague(tt.league)
			if !ok {
				t.Fatalf("leagueConfigByLeague(%q) not found", tt.league)
			}
			got := cdnSportSlug(cfg)
			if got != tt.wantSlug {
				t.Errorf("cdnSportSlug = %q, want %q", got, tt.wantSlug)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNormalizeCoachRefs — verifies normalizeCoachRefs converts PagedRefs to
// simple table rows.
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeCoachRefs(t *testing.T) {
	paged := &espn.PagedRefs{
		Count: 2,
		Items: []espn.Ref{
			{Ref: "http://sports.core.api.espn.com/v2/sports/football/leagues/nfl/coaches/1"},
			{Ref: "http://sports.core.api.espn.com/v2/sports/football/leagues/nfl/coaches/2"},
		},
	}

	rows := normalizeCoachRefs(paged)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0][0] != "1" {
		t.Errorf("row 0 index = %q, want '1'", rows[0][0])
	}
	if rows[1][1] != paged.Items[1].Ref {
		t.Errorf("row 1 ref = %q, want %q", rows[1][1], paged.Items[1].Ref)
	}
}

// TestNormalizeCoachRefsNil verifies nil input returns nil/empty.
func TestNormalizeCoachRefsNil(t *testing.T) {
	rows := normalizeCoachRefs(nil)
	if len(rows) != 0 {
		t.Errorf("expected empty, got %d rows", len(rows))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestQBRDetectionNoLeagueDefaultsNFL — verifies that QBR queries without an
// explicit league default to NFL.
// ─────────────────────────────────────────────────────────────────────────────

func TestQBRDetectionNoLeagueDefaultsNFL(t *testing.T) {
	req, ok := DetectSportsIntent("Show me the QBR leaderboard", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for QBR query")
	}
	if req.Intent != SportsIntentQBR {
		t.Errorf("intent = %q, want qbr", req.Intent)
	}
	if req.League != espn.LeagueNFL {
		t.Errorf("league = %q, want nfl (default for QBR)", req.League)
	}
}

func TestQBRDetectionExtractsCleanAthlete(t *testing.T) {
	req, ok := DetectSportsIntent("Show Patrick Mahomes' QBR for 2023", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for QBR query")
	}
	if req.AthleteQuery != "patrick mahomes" {
		t.Errorf("AthleteQuery = %q, want %q", req.AthleteQuery, "patrick mahomes")
	}
	if req.Season != 2023 {
		t.Errorf("Season = %d, want 2023", req.Season)
	}
}

func TestQBRGroupForLeague(t *testing.T) {
	if got := qbrGroupForLeague(espn.LeagueNFL); got != 9 {
		t.Errorf("NFL QBR group = %d, want 9", got)
	}
	if got := qbrGroupForLeague(espn.LeagueCollegeFootball); got != espn.GroupFBS {
		t.Errorf("CFB QBR group = %d, want %d", got, espn.GroupFBS)
	}
}

func TestNormalizeQBRTable(t *testing.T) {
	raw := json.RawMessage(`{
		"items": [
			{
				"athlete": {"$ref": "http://sports.core.api.espn.com/v2/sports/football/leagues/nfl/seasons/2023/athletes/3139477?lang=en"},
				"team": {"$ref": "http://sports.core.api.espn.com/v2/sports/football/leagues/nfl/seasons/2023/teams/12?lang=en"},
				"splits": {"categories": [{"name": "general", "stats": [
					{"name": "schedAdjQBR", "displayValue": "63.9", "value": 63.9},
					{"name": "qbr", "displayValue": "65.1", "value": 65.1},
					{"name": "qbpaa", "displayValue": "32.5", "value": 32.5},
					{"name": "actionPlays", "displayValue": "706", "value": 706}
				]}]}
			},
			{
				"athlete": {"$ref": "http://sports.core.api.espn.com/v2/sports/football/leagues/nfl/seasons/2023/athletes/2577417?lang=en"},
				"team": {"$ref": "http://sports.core.api.espn.com/v2/sports/football/leagues/nfl/seasons/2023/teams/6?lang=en"},
				"splits": {"categories": [{"name": "general", "stats": [
					{"name": "schedAdjQBR", "displayValue": "73.4", "value": 73.4},
					{"name": "qbr", "displayValue": "75.3", "value": 75.3},
					{"name": "qbpaa", "displayValue": "57.0", "value": 56.98},
					{"name": "actionPlays", "displayValue": "724", "value": 724}
				]}]}
			}
		]
	}`)
	table := normalizeQBRTable(
		raw,
		map[string]string{"3139477": "Patrick Mahomes", "2577417": "Dak Prescott"},
		map[string]string{"12": "KC", "6": "DAL"},
		10,
	)
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if got := table.Rows[0][1]; got != "Dak Prescott" {
		t.Errorf("top player = %q, want Dak Prescott", got)
	}
	if got := table.Rows[1][1]; got != "Patrick Mahomes" {
		t.Errorf("second player = %q, want Patrick Mahomes", got)
	}
	if got := table.Rows[1][3]; got != "63.9" {
		t.Errorf("Patrick total QBR = %q, want 63.9", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestHotZonesDetectionExtractsAthlete — verifies that an athlete name is
// extracted from a hot-zones query.
// ─────────────────────────────────────────────────────────────────────────────

func TestHotZonesDetectionExtractsAthlete(t *testing.T) {
	req, ok := DetectSportsIntent("What are Mookie Betts' hot zones?", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for hot zones query")
	}
	if req.Intent != SportsIntentHotZones {
		t.Errorf("intent = %q, want hot_zones", req.Intent)
	}
	if !strings.Contains(strings.ToLower(req.AthleteQuery), "mookie") {
		t.Errorf("AthleteQuery = %q; want it to contain 'mookie'", req.AthleteQuery)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestSearchIntentPopulatesAthleteQuery — verifies that the search term is
// stored in AthleteQuery for SportsIntentSearch.
// ─────────────────────────────────────────────────────────────────────────────

func TestSearchIntentPopulatesAthleteQuery(t *testing.T) {
	req, ok := DetectSportsIntent("Search ESPN for Shohei Ohtani", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for search query")
	}
	if req.Intent != SportsIntentSearch {
		t.Fatalf("intent = %q, want search", req.Intent)
	}
	if !strings.Contains(req.AthleteQuery, "Ohtani") {
		t.Errorf("AthleteQuery = %q; want it to contain 'Ohtani'", req.AthleteQuery)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDraftDetectionWithSeason — verifies draft intent and season extraction.
// ─────────────────────────────────────────────────────────────────────────────

func TestDraftDetectionWithSeason(t *testing.T) {
	req, ok := DetectSportsIntent("Show me the 2024 NFL draft results", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for draft query")
	}
	if req.Intent != SportsIntentDraft {
		t.Errorf("intent = %q, want draft", req.Intent)
	}
	if req.League != espn.LeagueNFL {
		t.Errorf("league = %q, want nfl", req.League)
	}
	if req.Season != 2024 {
		t.Errorf("season = %d, want 2024", req.Season)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCoachesDetection — verifies coaches intent is detected and doesn't
// collide with the coaches poll (rankings).
// ─────────────────────────────────────────────────────────────────────────────

func TestCoachesDetection(t *testing.T) {
	tests := []struct {
		query      string
		wantIntent SportsIntentType
	}{
		// Should be coaches
		{"Who is the head coach of the Dallas Cowboys?", SportsIntentCoaches},
		{"Show me NFL coaching staff", SportsIntentCoaches},
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
		})
	}
}

// TestCoachesPolIsNotCoachesIntent verifies that "coaches poll" (college football
// rankings) does not trigger SportsIntentCoaches.
func TestCoachesPollIsNotCoachesIntent(t *testing.T) {
	q := "Show me the AP and coaches poll rankings"
	if req, ok := DetectSportsIntent(q, fixedNow()); ok {
		if req.Intent == SportsIntentCoaches {
			t.Errorf("'coaches poll' should not trigger SportsIntentCoaches, got %q", req.Intent)
		}
	}
	// It's OK for the query to return nil/false (no league context).
}
