//go:build integration

package sports

// sports_integration_test.go — Live ESPN API roundtrip smoke tests.
//
// These tests are NOT run by default (they require a live network connection
// and valid ESPN API access). To run them:
//
//   cd backend && go test ./internal/sports/... -tags=integration -count=1 -v
//
// Each test creates a real ESPNClient and calls the ESPN API, asserting that
// the response is non-empty and properly normalized.
//
// The tests use the LeagueConfig lookup and DetectSportsIntent to ensure
// end-to-end intent → client → normalization works correctly.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

// newLiveClient returns an ESPNClient backed by the real ESPN API.
func newLiveClient() *ESPNClient {
	return NewESPNClient()
}

func TestIntegration_NBAScoreboard(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("NBA scores today", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	result, err := c.Lookup(ctx, *req)
	if err != nil {
		if errors.Is(err, ErrNoGames) || errors.Is(err, ErrNoMatchingGames) {
			t.Skipf("no NBA games scheduled today, skipping: %v", err)
		}
		t.Fatalf("Lookup error: %v", err)
	}
	if result == nil {
		t.Fatal("Lookup returned nil result")
	}
}

func TestIntegration_MLBStandings(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("MLB standings", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	result, err := c.Lookup(ctx, *req)
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if result == nil {
		t.Fatal("Lookup returned nil result")
	}
}

func TestIntegration_SerieAScores(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("Serie A scores today", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for Serie A")
	}
	if req.League != espn.LeagueSerieA {
		t.Fatalf("league = %q, want Serie A", req.League)
	}
	result, err := c.Lookup(ctx, *req)
	if err != nil {
		if errors.Is(err, ErrNoGames) || errors.Is(err, ErrNoMatchingGames) {
			t.Skipf("no Serie A games scheduled today, skipping: %v", err)
		}
		t.Fatalf("Lookup error: %v", err)
	}
	if result == nil {
		t.Fatal("Lookup returned nil result")
	}
}

func TestIntegration_F1Standings(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("F1 standings", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false for F1")
	}
	result, err := c.Lookup(ctx, *req)
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if result == nil {
		t.Fatal("Lookup returned nil result")
	}
}

func TestIntegration_PatrickMahomesQBR2023(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("Show Patrick Mahomes' QBR for 2023", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	if req.Intent != SportsIntentQBR {
		t.Fatalf("intent = %q, want qbr", req.Intent)
	}
	if req.AthleteQuery != "patrick mahomes" {
		t.Fatalf("AthleteQuery = %q, want patrick mahomes", req.AthleteQuery)
	}
	result, err := c.Lookup(ctx, *req)
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if result == nil || result.Markdown == "" {
		t.Fatal("expected non-empty QBR result")
	}
	if !strings.Contains(result.Markdown, "Patrick Mahomes") {
		t.Fatalf("QBR markdown missing Patrick Mahomes:\n%s", result.Markdown)
	}
	if !strings.Contains(result.Markdown, "63.9") {
		t.Fatalf("QBR markdown missing 2023 Total QBR 63.9:\n%s", result.Markdown)
	}
}

func TestIntegration_LAKingsNews(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("What is the latest news for the LA Kings?", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	if req.Intent != SportsIntentNews {
		t.Fatalf("intent = %q, want news", req.Intent)
	}
	if req.TeamQuery != "Los Angeles Kings" {
		t.Fatalf("TeamQuery = %q, want Los Angeles Kings", req.TeamQuery)
	}
	result, err := c.Lookup(ctx, *req)
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if result == nil || result.Markdown == "" {
		t.Fatal("expected non-empty Kings news result")
	}
	if !strings.Contains(result.Markdown, "Los Angeles Kings") {
		t.Fatalf("Kings news markdown missing team title:\n%s", result.Markdown)
	}
}

// TestIntegration_NFLRushingTDLeaders1985 exercises the historical leader
// lookup: "Who led the NFL in rushing TDs in 1985". ESPN's byathlete stats
// endpoint returns HTTP 400 for seasons before ~2002, so we expect either a
// successful result OR the specific "historical data not available" message.
func TestIntegration_NFLRushingTDLeaders1985(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("Who led the NFL in rushing TDs in 1985", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	t.Logf("intent=%s league=%s statSort=%q season=%d teamQuery=%q athleteQuery=%q",
		req.Intent, req.League, req.StatSort, req.Season, req.TeamQuery, req.AthleteQuery)

	if req.Intent != SportsIntentLeaders {
		t.Fatalf("intent = %q, want leaders", req.Intent)
	}
	if req.StatSort == "" {
		t.Fatal("StatSort is empty — rushing TDs alias not matched")
	}

	result, err := c.Lookup(ctx, *req)
	if err != nil {
		// ESPN returns 400 for pre-2002 seasons; verify we get the helpful message.
		if errors.Is(err, ErrNoSportsData) {
			msg := UserFacingError(*req, err)
			t.Logf("ESPN historical data unavailable (expected): %s", msg)
			if req.Season > 0 && !strings.Contains(msg, "2002") {
				t.Errorf("expected historical-data message to mention 2002, got: %s", msg)
			}
			return // acceptable — ESPN doesn't have 1985 data
		}
		t.Fatalf("unexpected Lookup error: %v", err)
	}
	t.Logf("result markdown (first 500 chars):\n%.500s", result.Markdown)
}

// TestIntegration_ChicagoBearsRushingTDLeaders1985 exercises a team-scoped
// historical leader query: "Who led the Chicago Bears in Rushing TDs in 1985".
func TestIntegration_ChicagoBearsRushingTDLeaders1985(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("Who led the Chicago Bears in Rushing TDs in 1985", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	t.Logf("intent=%s league=%s statSort=%q season=%d teamQuery=%q athleteQuery=%q",
		req.Intent, req.League, req.StatSort, req.Season, req.TeamQuery, req.AthleteQuery)

	if req.Intent != SportsIntentLeaders {
		t.Fatalf("intent = %q, want leaders", req.Intent)
	}
	if req.StatSort == "" {
		t.Fatal("StatSort is empty — rushing TDs alias not matched")
	}

	result, err := c.Lookup(ctx, *req)
	if err != nil {
		if errors.Is(err, ErrNoSportsData) {
			msg := UserFacingError(*req, err)
			t.Logf("ESPN historical data unavailable (expected): %s", msg)
			if req.Season > 0 && !strings.Contains(msg, "2002") {
				t.Errorf("expected historical-data message to mention 2002, got: %s", msg)
			}
			return // acceptable — ESPN doesn't have 1985 data
		}
		t.Fatalf("unexpected Lookup error: %v", err)
	}
	t.Logf("result markdown (first 500 chars):\n%.500s", result.Markdown)
}

// TestIntegration_ChicagoBearsRushingTDLeaders2025 is the concrete test for
// Bug 4: "Who led the Chicago Bears in Rushing TDs in 2025?" should return a
// leaderboard, NOT the "historical data not available" error.
func TestIntegration_ChicagoBearsRushingTDLeaders2025(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("Who led the Chicago Bears in Rushing TDs in 2025", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	t.Logf("intent=%s league=%s statSort=%q season=%d teamQuery=%q athleteQuery=%q",
		req.Intent, req.League, req.StatSort, req.Season, req.TeamQuery, req.AthleteQuery)

	result, err := c.Lookup(ctx, *req)
	if err != nil {
		if errors.Is(err, ErrNoSportsData) {
			msg := UserFacingError(*req, err)
			if strings.Contains(msg, "2002") {
				t.Errorf("Bug 4 regression: got historical-data error for 2025 season: %s", msg)
			} else {
				t.Logf("no stats for 2025 Bears (off-season?): %s", msg)
			}
			return
		}
		t.Fatalf("unexpected Lookup error: %v", err)
	}
	t.Logf("result markdown:\n%s", result.Markdown)
	if result.Markdown == "" {
		t.Fatal("expected non-empty markdown")
	}
}

// TestIntegration_NFLRushingTDLeaders2025 tests league-wide rushing TD leaders
// for the 2025 season using CoreLeaders.
func TestIntegration_NFLRushingTDLeaders2025(t *testing.T) {
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent("Who led the NFL in rushing TDs in 2025", time.Now())
	if !ok {
		t.Fatal("DetectSportsIntent returned false")
	}
	t.Logf("intent=%s league=%s statSort=%q season=%d teamQuery=%q athleteQuery=%q",
		req.Intent, req.League, req.StatSort, req.Season, req.TeamQuery, req.AthleteQuery)

	result, err := c.Lookup(ctx, *req)
	if err != nil {
		if errors.Is(err, ErrNoSportsData) {
			t.Logf("no data: %v", UserFacingError(*req, err))
			return
		}
		t.Fatalf("unexpected Lookup error: %v", err)
	}
	t.Logf("result markdown:\n%s", result.Markdown)
	if result.Markdown == "" {
		t.Fatal("expected non-empty markdown")
	}
}

// ---------------------------------------------------------------------------
// Additional stat-category integration tests (NFL 2025)
// ---------------------------------------------------------------------------

// runLeaderQueryTest is a shared helper for simple leader queries.
func runLeaderQueryTest(t *testing.T, query string) {
	t.Helper()
	c := newLiveClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, ok := DetectSportsIntent(query, time.Now())
	if !ok {
		t.Fatalf("DetectSportsIntent returned false for: %q", query)
	}
	t.Logf("intent=%s league=%s statSort=%q season=%d teamQuery=%q",
		req.Intent, req.League, req.StatSort, req.Season, req.TeamQuery)

	result, err := c.Lookup(ctx, *req)
	if err != nil {
		if errors.Is(err, ErrNoSportsData) {
			t.Logf("no data (acceptable off-season): %v", UserFacingError(*req, err))
			return
		}
		t.Fatalf("unexpected Lookup error: %v", err)
	}
	if result.Markdown == "" {
		t.Fatal("expected non-empty markdown")
	}
	// Basic format checks
	if !strings.Contains(result.Markdown, "| Rank |") {
		t.Errorf("markdown missing rank header:\n%s", result.Markdown)
	}
	if !strings.Contains(result.Markdown, "| 1 |") {
		t.Errorf("markdown missing rank-1 row:\n%s", result.Markdown)
	}
	t.Logf("result markdown:\n%s", result.Markdown)
}

func TestIntegration_NFLPassingYardsLeaders2025(t *testing.T) {
	runLeaderQueryTest(t, "Who led the NFL in passing yards in 2025")
}

func TestIntegration_NFLReceivingYardsLeaders2025(t *testing.T) {
	runLeaderQueryTest(t, "NFL receiving yards leaders 2025")
}

func TestIntegration_NFLRushingYardsLeaders2025(t *testing.T) {
	runLeaderQueryTest(t, "Who led the NFL in rushing yards in 2025")
}

func TestIntegration_NFLSacksLeaders2025(t *testing.T) {
	runLeaderQueryTest(t, "NFL sacks leaders 2025")
}

func TestIntegration_NFLPassingTDLeaders2025(t *testing.T) {
	// Use the statSort-aware phrasing so it routes through lookupLeagueStatLeaders.
	runLeaderQueryTest(t, "NFL passing touchdown leaders 2025")
}

func TestIntegration_KCChiefsRushingYards2025(t *testing.T) {
	runLeaderQueryTest(t, "Who led the Kansas City Chiefs in rushing yards in 2025")
}

func TestIntegration_KCChiefsReceivingYards2025(t *testing.T) {
	runLeaderQueryTest(t, "Who led the Chiefs in receiving yards 2025")
}

func TestIntegration_NFLLeagueWideNoSeason(t *testing.T) {
	// Query without a season year — should default to current year
	runLeaderQueryTest(t, "Who leads the NFL in rushing touchdowns")
}

func TestIntegration_NFLTeamQueryNoSeason(t *testing.T) {
	// Team query without a season year — should default to current year
	runLeaderQueryTest(t, "Who leads the Philadelphia Eagles in receiving yards")
}
