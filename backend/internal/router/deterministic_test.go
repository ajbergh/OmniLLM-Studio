package router

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sports"
)

func TestDeterministicWorldCupRoute(t *testing.T) {
	now := time.Date(2026, time.July, 19, 7, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	decision, ok := deterministicSportsRoute("What time does the World Cup game start today?", now)
	if !ok || decision.Route != RouteSportsLookup || decision.Sports == nil {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if decision.Sports.League != sports.LeagueFIFAWorldCup || decision.Sports.Intent != "schedule" {
		t.Fatalf("unexpected sports params: %#v", decision.Sports)
	}
}

func TestDeterministicSportsRouteRejectsNonSportsNews(t *testing.T) {
	now := time.Date(2026, time.July, 20, 10, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	queries := []string{
		"What's the latest tech news?",
		"What's the latest news in politics?",
		"Give me the latest AI headlines",
	}

	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			if decision, ok := deterministicSportsRoute(query, now); ok {
				t.Fatalf("deterministicSportsRoute(%q) = %#v, true; want no sports route", query, decision)
			}
		})
	}
}
