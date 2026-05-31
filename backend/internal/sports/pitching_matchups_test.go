package sports

import (
	"strings"
	"testing"
)

func TestDetectSportsIntentPitchingMatchups(t *testing.T) {
	req, ok := DetectSportsIntent("What are the pitching matchups for todays mlb games?", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent did not handle pitching matchup query")
	}
	if req.Intent != SportsIntentSchedule {
		t.Fatalf("Intent = %q, want schedule", req.Intent)
	}
	if req.GameDetailSubtype != "pitching_matchups" {
		t.Fatalf("GameDetailSubtype = %q, want pitching_matchups", req.GameDetailSubtype)
	}
}

func TestRenderPitchingMatchupsMarkdown(t *testing.T) {
	req := SportsRequest{
		Intent:            SportsIntentSchedule,
		GameDetailSubtype: "pitching_matchups",
		RenderMode:        SportsRenderPlainMarkdown,
	}
	cfg := LeagueConfig{DisplayName: "MLB", League: "mlb", Sport: "baseball"}
	rows := []GameRow{{
		Time:            "1:05 PM",
		AwayTeam:        "Miami Marlins",
		HomeTeam:        "Tampa Bay Rays",
		PitchingMatchup: "Sandy Alcantara (2-4, 4.10) vs Drew Rasmussen (3-1, 3.16)",
		Venue:           "Tropicana Field",
		Broadcasts:      "Peacock",
	}}
	got := RenderGamesMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "MLB Pitching Matchups") {
		t.Fatalf("missing pitching matchup title: %s", got)
	}
	if !strings.Contains(got, "Pitching Matchup") || !strings.Contains(got, "Drew Rasmussen") {
		t.Fatalf("missing pitching matchup column/value: %s", got)
	}
}

func TestNormalizePitchingMatchupScoreboard(t *testing.T) {
	raw := []byte(`{
		"events": [{
			"date": "2026-05-17T17:15Z",
			"competitions": [{
				"date": "2026-05-17T17:15Z",
				"competitors": [
					{"homeAway":"home","team":{"displayName":"Tampa Bay Rays","abbreviation":"TB"},"probables":[{"athlete":{"displayName":"Drew Rasmussen"},"record":"(3-1, 3.16)"}]},
					{"homeAway":"away","team":{"displayName":"Miami Marlins","abbreviation":"MIA"},"probables":[{"athlete":{"displayName":"Sandy Alcantara"},"record":"(2-4, 4.10)"}]}
				]
			}]
		}]
	}`)
	rows := normalizePitchingMatchupScoreboard(raw)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].PitchingMatchup != "Sandy Alcantara (2-4, 4.10) vs Drew Rasmussen (3-1, 3.16)" {
		t.Fatalf("PitchingMatchup = %q", rows[0].PitchingMatchup)
	}
}
