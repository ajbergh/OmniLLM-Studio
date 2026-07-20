package sports

import (
	"strings"
	"testing"
	"time"
)

func TestDetectWorldCupScheduleToday(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 19, 7, 30, 0, 0, loc)
	req, ok := DetectSportsIntent("What Time Does the World Cup Game Start Today", now)
	if !ok || req == nil {
		t.Fatal("expected World Cup schedule intent")
	}
	if req.Intent != SportsIntentSchedule || req.League != LeagueFIFAWorldCup {
		t.Fatalf("unexpected request: %#v", req)
	}
	if req.Date == nil || req.Date.Format("2006-01-02") != "2026-07-19" {
		t.Fatalf("today was not resolved in local timezone: %#v", req.Date)
	}
}

func TestDirectWorldCupScheduleMarkdown(t *testing.T) {
	req := SportsRequest{
		RawQuery: "What time does the World Cup game start today?",
		Intent:   SportsIntentSchedule,
	}
	rows := []GameRow{{
		AwayTeam: "Argentina",
		HomeTeam: "Spain",
		Time:     "3:00 PM CDT",
	}}
	got := RenderGamesMarkdown(req, LeagueConfig{DisplayName: "FIFA World Cup"}, rows, time.Now())
	want := "Argentina vs. Spain starts at 3:00 PM CDT."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if strings.Contains(got, "###") || strings.Contains(got, "|") || len(strings.Fields(got)) > 12 {
		t.Fatalf("direct lookup regressed to a verbose or tabular response: %q", got)
	}
}
