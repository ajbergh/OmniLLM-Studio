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
