package router

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sports"
	espn "github.com/chinmaykhachane/espn-go"
)

func TestSportsRequestFromDecisionMLBStandings(t *testing.T) {
	req, err := SportsRequestFromDecision("What are the MLB standings?", RouterDecision{
		Route:      RouteSportsLookup,
		Confidence: 0.98,
		Sports: &SportsRouteParams{
			Intent: "standings",
			League: "MLB",
		},
	}, time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SportsRequestFromDecision returned error: %v", err)
	}
	if req.Intent != sports.SportsIntentStandings || req.League != espn.LeagueMLB || req.Sport != espn.SportBaseball {
		t.Fatalf("unexpected request: %#v", req)
	}
}

func TestSportsRequestFromDecisionNHLGoalLeaders(t *testing.T) {
	limit := 25
	req, err := SportsRequestFromDecision("Who leads the NHL in goals?", RouterDecision{
		Route:      RouteSportsLookup,
		Confidence: 0.92,
		Sports: &SportsRouteParams{
			Intent: "leaders",
			League: "NHL",
			Metric: "goals",
			Limit:  &limit,
		},
	}, time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SportsRequestFromDecision returned error: %v", err)
	}
	if req.Intent != sports.SportsIntentLeaders || req.League != espn.LeagueNHL {
		t.Fatalf("unexpected request: %#v", req)
	}
	if req.StatCategory != "scoring" || req.StatName != "goals" || req.StatSort != "scoring.goals:desc" {
		t.Fatalf("goal metric was not mapped: %#v", req)
	}
}

func TestSportsRequestFromDecisionBadDate(t *testing.T) {
	_, err := SportsRequestFromDecision("MLB scores on bad date", RouterDecision{
		Route:      RouteSportsLookup,
		Confidence: 0.9,
		Sports: &SportsRouteParams{
			Intent: "scores",
			League: "MLB",
			Date:   "05/17/2026",
		},
	}, time.Now())
	if err != sports.ErrMalformedDate {
		t.Fatalf("err = %v, want ErrMalformedDate", err)
	}
}

func TestSportsRequestFromDecisionPitchingMatchups(t *testing.T) {
	req, err := SportsRequestFromDecision("What are the pitching matchups for todays mlb games?", RouterDecision{
		Route:      RouteSportsLookup,
		Confidence: 0.9,
		Sports: &SportsRouteParams{
			Intent: "schedule",
			League: "MLB",
		},
	}, time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SportsRequestFromDecision returned error: %v", err)
	}
	if req.GameDetailSubtype != "pitching_matchups" {
		t.Fatalf("GameDetailSubtype = %q, want pitching_matchups", req.GameDetailSubtype)
	}
}
