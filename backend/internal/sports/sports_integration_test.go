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
	"testing"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

// newLiveClient returns an ESPNClient backed by the real ESPN API.
func newLiveClient() *ESPNClient {
	return NewESPNClient(espn.NewClient())
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
