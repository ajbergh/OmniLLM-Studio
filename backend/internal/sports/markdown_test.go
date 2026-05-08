package sports

import (
	"strings"
	"testing"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

func TestEscapeMarkdownCell(t *testing.T) {
	got := escapeMarkdownCell("A | B\nC")
	want := `A \| B C`
	if got != want {
		t.Fatalf("escapeMarkdownCell = %q, want %q", got, want)
	}
}

func TestEscapeHTML(t *testing.T) {
	got := escapeHTML(`<img src=x onerror=alert(1)>`)
	want := "&lt;img src=x onerror=alert(1)&gt;"
	if got != want {
		t.Fatalf("escapeHTML = %q, want %q", got, want)
	}
}

func TestSanitizeImageURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"https", "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png", "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png"},
		{"http upgraded", "http://a.espncdn.com/i/teamlogos/mlb/500/chc.png", "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png"},
		{"protocol relative", "//a.espncdn.com/i/teamlogos/mlb/500/chc.png", "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png"},
		{"javascript rejected", "javascript:alert(1)", ""},
		{"data rejected", "data:image/svg+xml;base64,abc", ""},
		{"file rejected", "file:///tmp/logo.png", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeImageURL(tt.input); got != tt.want {
				t.Fatalf("sanitizeImageURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatFloatStat(t *testing.T) {
	tests := map[float64]string{
		26:     "26",
		0.684:  ".684",
		-0.125: "-.125",
		1.5:    "1.5",
	}
	for input, want := range tests {
		if got := formatFloatStat(input); got != want {
			t.Fatalf("formatFloatStat(%v) = %q, want %q", input, got, want)
		}
	}
}

func TestRenderGamesMarkdownScores(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentScores, DateLabel: "Today", RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "MLB", Sport: espn.SportBaseball, League: espn.LeagueMLB}
	retrieved := time.Date(2026, 5, 7, 18, 45, 0, 0, time.Local)
	rows := []GameRow{
		{
			Status:    "Final",
			AwayTeam:  "Chicago Cubs",
			AwayScore: "5",
			HomeTeam:  "St. Louis | Cardinals",
			HomeScore: "3",
			Venue:     "Busch Stadium",
		},
	}

	got := RenderGamesMarkdown(req, cfg, rows, retrieved)
	if !strings.Contains(got, "### MLB Scores — Today") {
		t.Fatalf("missing title: %s", got)
	}
	if !strings.Contains(got, "| Final | Chicago Cubs | 5 | St. Louis \\| Cardinals | 3 | Busch Stadium |") {
		t.Fatalf("missing escaped score row: %s", got)
	}
	if !strings.Contains(got, "_Source: ESPN public API. Retrieved: 2026-05-07 6:45 PM_") {
		t.Fatalf("missing source line: %s", got)
	}
}

func TestRenderGamesMarkdownScheduleWhenPregame(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentScores, DateLabel: "Tomorrow", RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "NFL", Sport: espn.SportFootball, League: espn.LeagueNFL}
	rows := []GameRow{
		{Time: "7:20 PM", AwayTeam: "Dallas Cowboys", HomeTeam: "New York Giants", Venue: "MetLife Stadium", Broadcasts: "NBC"},
	}

	got := RenderGamesMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "| Time | Away | Home | Venue | Broadcast |") {
		t.Fatalf("expected schedule table for pregame scores: %s", got)
	}
}

func TestRenderGamesMarkdownEnhancedScores(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentScores, DateLabel: "Today"}
	cfg := LeagueConfig{DisplayName: "MLB", Sport: espn.SportBaseball, League: espn.LeagueMLB}
	rows := []GameRow{
		{
			Status:     "Final",
			StatusType: "final",
			Away: TeamIdentity{
				DisplayName:  "Chicago Cubs",
				Abbreviation: "CHC",
				LogoURL:      "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png",
			},
			AwayTeam:  "Chicago Cubs",
			AwayAbbr:  "CHC",
			AwayScore: "5",
			Home: TeamIdentity{
				DisplayName:  "St. Louis | Cardinals",
				Abbreviation: "STL",
			},
			HomeTeam:  "St. Louis | Cardinals",
			HomeAbbr:  "STL",
			HomeScore: "3",
			Venue:     "Busch Stadium",
		},
	}

	got := RenderGamesMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "### ![MLB logo]") {
		t.Fatalf("missing league logo header: %s", got)
	}
	if !strings.Contains(got, "| Status | Matchup | Score | Venue |") {
		t.Fatalf("missing enhanced score header: %s", got)
	}
	if !strings.Contains(got, "![CHC logo](https://a.espncdn.com/i/teamlogos/mlb/500/chc.png) Chicago Cubs at **STL** St. Louis \\| Cardinals") {
		t.Fatalf("missing logo matchup cell: %s", got)
	}
	if !strings.Contains(got, "CHC 5 · STL 3") {
		t.Fatalf("missing compact score: %s", got)
	}
}

func TestRenderTeamCellWithoutLogoUsesAbbreviationBadge(t *testing.T) {
	got := renderTeamCell(TeamIdentity{DisplayName: "New York Yankees", Abbreviation: "NYY"}, SportsRenderEnhancedMarkdown)
	want := "**NYY** New York Yankees"
	if got != want {
		t.Fatalf("renderTeamCell = %q, want %q", got, want)
	}
}

func TestRenderStatusBadgeLabels(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		statusType string
		want       string
	}{
		{"final", "Final", "final", "Final"},
		{"live", "7th", "live", "**7th**"},
		{"preview", "8:10 PM", "scheduled", "8:10 PM"},
		{"postponed", "Postponed", "postponed", "Postponed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderStatusBadge(tt.status, tt.statusType, SportsRenderEnhancedMarkdown); got != tt.want {
				t.Fatalf("renderStatusBadge = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderStandingsMarkdownGrouped(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "MLB", Sport: espn.SportBaseball, League: espn.LeagueMLB}
	rows := []StandingsRow{
		{Group: "American League East", Rank: 1, Team: "New York Yankees", Wins: "26", Losses: "12", Pct: ".684", GamesBack: "", Streak: "W2"},
		{Group: "American League East", Rank: 2, Team: "Tampa Bay Rays", Wins: "25", Losses: "12", Pct: ".676", GamesBack: "0.5", Streak: "L1"},
		{Group: "National League West", Rank: 1, Team: "Los Angeles Dodgers", Wins: "24", Losses: "14", Pct: ".632", Streak: "W1"},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "#### American League") || !strings.Contains(got, "##### East") ||
		!strings.Contains(got, "#### National League") || !strings.Contains(got, "##### West") {
		t.Fatalf("missing grouped headings: %s", got)
	}
	if strings.Contains(got, "| Group | Rank |") {
		t.Fatalf("grouped standings should not include redundant group column: %s", got)
	}
	if !strings.Contains(got, "| 1 | New York Yankees | 26 | 12 | .684 | W2 | — | — |") {
		t.Fatalf("missing standings row: %s", got)
	}
}

func TestRenderMLBStandingsSplitsLeagueRowsByDivision(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "MLB", Sport: espn.SportBaseball, League: espn.LeagueMLB}
	rows := []StandingsRow{
		{Group: "American League", Team: "New York Yankees", Abbr: "NYY", Wins: "26", Losses: "12", Pct: ".684", Streak: "W1", LastTen: "8-2"},
		{Group: "American League", Team: "Tampa Bay Rays", Abbr: "TB", Wins: "25", Losses: "12", Pct: ".676", Streak: "W7", LastTen: "9-1"},
		{Group: "American League", Team: "Athletics", Abbr: "ATH", Wins: "19", Losses: "18", Pct: ".514", Streak: "W1", LastTen: "5-5"},
		{Group: "American League", Team: "Cleveland Guardians", Abbr: "CLE", Wins: "20", Losses: "19", Pct: ".513", Streak: "W2", LastTen: "5-5"},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "##### East") || !strings.Contains(got, "##### Central") || !strings.Contains(got, "##### West") {
		t.Fatalf("missing MLB division headings: %s", got)
	}
	if !strings.Contains(got, "| 2 | Tampa Bay Rays | 25 | 12 | .676 | W7 | 9-1 | 0.5 |") {
		t.Fatalf("Rays should be second in AL East, got: %s", got)
	}
	if !strings.Contains(got, "| 1 | Cleveland Guardians | 20 | 19 | .513 | W2 | 5-5 | — |") {
		t.Fatalf("Guardians should lead AL Central, got: %s", got)
	}
	if !strings.Contains(got, "| 1 | Athletics | 19 | 18 | .514 | W1 | 5-5 | — |") {
		t.Fatalf("Athletics should lead AL West, got: %s", got)
	}
}

func TestRenderNFLStandingsSplitsConferenceRowsByDivision(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "NFL", Sport: espn.SportFootball, League: espn.LeagueNFL}
	rows := []StandingsRow{
		{Group: "AFC", Team: "Buffalo Bills", Abbr: "BUF", Wins: "10", Losses: "3", Pct: ".769", Streak: "W2", LastTen: "8-2"},
		{Group: "AFC", Team: "Miami Dolphins", Abbr: "MIA", Wins: "8", Losses: "5", Pct: ".615", Streak: "L1", LastTen: "6-4"},
		{Group: "AFC", Team: "Baltimore Ravens", Abbr: "BAL", Wins: "9", Losses: "4", Pct: ".692", Streak: "W1", LastTen: "7-3"},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "#### AFC") || !strings.Contains(got, "##### East") || !strings.Contains(got, "##### North") {
		t.Fatalf("missing NFL division headings: %s", got)
	}
	if !strings.Contains(got, "| 2 | Miami Dolphins | 8 | 5 | .615 | L1 | 6-4 | 2.0 |") {
		t.Fatalf("expected Dolphins second in AFC East with division GB: %s", got)
	}
	if !strings.Contains(got, "| 1 | Baltimore Ravens | 9 | 4 | .692 | W1 | 7-3 | — |") {
		t.Fatalf("expected Ravens to lead AFC North: %s", got)
	}
}

func TestRenderNBAStandingsSplitsConferenceRowsByDivision(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "NBA", Sport: espn.SportBasketball, League: espn.LeagueNBA}
	rows := []StandingsRow{
		{Group: "Eastern Conference", Team: "Boston Celtics", Abbr: "BOS", Wins: "50", Losses: "20", Pct: ".714", Streak: "W3", LastTen: "7-3"},
		{Group: "Eastern Conference", Team: "Cleveland Cavaliers", Abbr: "CLE", Wins: "48", Losses: "22", Pct: ".686", Streak: "W1", LastTen: "6-4"},
		{Group: "Eastern Conference", Team: "Miami Heat", Abbr: "MIA", Wins: "41", Losses: "29", Pct: ".586", Streak: "L1", LastTen: "5-5"},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "#### Eastern Conference") ||
		!strings.Contains(got, "##### Atlantic") ||
		!strings.Contains(got, "##### Central") ||
		!strings.Contains(got, "##### Southeast") {
		t.Fatalf("missing NBA division headings: %s", got)
	}
	if !strings.Contains(got, "| 1 | Cleveland Cavaliers | 48 | 22 | .686 | W1 | 6-4 | — |") {
		t.Fatalf("expected Cavaliers to lead Central: %s", got)
	}
}

func TestRenderNHLStandingsUsesDivisionAndHockeyColumns(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "NHL", Sport: espn.SportHockey, League: espn.LeagueNHL}
	rows := []StandingsRow{
		{Group: "Eastern Conference", Team: "Boston Bruins", Abbr: "BOS", GamesPlayed: "70", Wins: "42", Losses: "20", Ties: "8", Points: "92", Pct: ".657", Streak: "W2", LastTen: "7-2-1"},
		{Group: "Eastern Conference", Team: "New York Rangers", Abbr: "NYR", GamesPlayed: "70", Wins: "40", Losses: "23", Ties: "7", Points: "87", Pct: ".621", Streak: "L1", LastTen: "5-4-1"},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "##### Atlantic") || !strings.Contains(got, "##### Metropolitan") {
		t.Fatalf("missing NHL division headings: %s", got)
	}
	if !strings.Contains(got, "| Rank | Team | GP | W | L | OT | Pts | Pct | Strk | L10 |") {
		t.Fatalf("missing NHL table columns: %s", got)
	}
	if !strings.Contains(got, "| 1 | New York Rangers | 70 | 40 | 23 | 7 | 87 | .621 | L1 | 5-4-1 |") {
		t.Fatalf("expected Rangers to lead Metropolitan: %s", got)
	}
}

func TestRenderMLSStandingsSplitsByConference(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "MLS", Sport: espn.SportSoccer, League: espn.LeagueMLS}
	rows := []StandingsRow{
		{Team: "Inter Miami CF", Abbr: "MIA", GamesPlayed: "10", Wins: "7", Draws: "2", Losses: "1", Points: "23", GoalDifferential: "+12"},
		{Team: "Los Angeles FC", Abbr: "LAFC", GamesPlayed: "10", Wins: "6", Draws: "1", Losses: "3", Points: "19", GoalDifferential: "+8"},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "#### Eastern Conference") || !strings.Contains(got, "#### Western Conference") {
		t.Fatalf("missing MLS conference headings: %s", got)
	}
	if !strings.Contains(got, "| Rank | Club | GP | W | D | L | Pts | GD |") {
		t.Fatalf("missing MLS soccer columns: %s", got)
	}
	if !strings.Contains(got, "| 1 | Los Angeles FC | 10 | 6 | 1 | 3 | 19 | +8 |") {
		t.Fatalf("expected LAFC to render in Western Conference: %s", got)
	}
}

func TestRenderStandingsMarkdownEnhancedGrouped(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings}
	cfg := LeagueConfig{DisplayName: "MLB", Sport: espn.SportBaseball, League: espn.LeagueMLB}
	rows := []StandingsRow{
		{
			Group: "American League East",
			Rank:  1,
			TeamIdentity: TeamIdentity{
				DisplayName:  "New York Yankees",
				Abbreviation: "NYY",
				LogoURL:      "https://a.espncdn.com/i/teamlogos/mlb/500/nyy.png",
			},
			Team:      "New York Yankees",
			Abbr:      "NYY",
			Wins:      "26",
			Losses:    "12",
			Pct:       ".684",
			GamesBack: "",
			Streak:    "W2",
		},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "### ![MLB logo]") {
		t.Fatalf("missing enhanced standings header: %s", got)
	}
	if !strings.Contains(got, "![NYY logo](https://a.espncdn.com/i/teamlogos/mlb/500/nyy.png) New York Yankees") {
		t.Fatalf("missing standings logo row: %s", got)
	}
}

func TestRenderSoccerStandingsMarkdown(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "Premier League", Sport: espn.SportSoccer, League: espn.LeagueEPL}
	rows := []StandingsRow{
		{Rank: 1, Team: "Arsenal", GamesPlayed: "38", Wins: "26", Draws: "8", Losses: "4", Points: "86", GoalDifferential: "+48"},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "| Rank | Club | GP | W | D | L | Pts | GD |") {
		t.Fatalf("missing soccer table header: %s", got)
	}
	if !strings.Contains(got, "| 1 | Arsenal | 38 | 26 | 8 | 4 | 86 | +48 |") {
		t.Fatalf("missing soccer row: %s", got)
	}
}

func TestRenderCricketStandingsMarkdown(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "Indian Premier League", Sport: espn.SportCricket, League: LeagueIPL}
	rows := []StandingsRow{
		{
			Rank:        1,
			Team:        "Chennai Super Kings",
			Abbr:        "CSK",
			GamesPlayed: "10",
			Wins:        "5",
			Losses:      "5",
			Ties:        "0",
			NoResult:    "1",
			Points:      "11",
			NetRunRate:  "0.151",
			For:         "1815/195.4",
			Against:     "1711/187.3",
		},
	}

	got := RenderStandingsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "| Rank | Team | M | W | L | T | N/R | PT | NRR | For | Against |") {
		t.Fatalf("missing cricket table header: %s", got)
	}
	if !strings.Contains(got, "| 1 | Chennai Super Kings | 10 | 5 | 5 | 0 | 1 | 11 | 0.151 | 1815/195.4 | 1711/187.3 |") {
		t.Fatalf("missing cricket row: %s", got)
	}
}

func TestRenderNewsMarkdown(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentNews, TeamQuery: "Chicago Cubs", RenderMode: SportsRenderPlainMarkdown}
	rows := []NewsRow{
		{
			Published:   "May 7, 1:20 PM",
			Headline:    "Cubs | bullpen update",
			Description: "Chicago gets late-inning help.",
			URL:         "https://www.espn.com/mlb/story/_/id/1/cubs-news",
		},
	}

	got := RenderNewsMarkdown(req, "MLB", rows, fixedNow())
	if !strings.Contains(got, "### Chicago Cubs News") {
		t.Fatalf("missing title: %s", got)
	}
	if !strings.Contains(got, "| Published | Headline | Summary | Link |") {
		t.Fatalf("missing news table: %s", got)
	}
	if !strings.Contains(got, "| May 7, 1:20 PM | Cubs \\| bullpen update | Chicago gets late-inning help. | [ESPN](https://www.espn.com/mlb/story/_/id/1/cubs-news) |") {
		t.Fatalf("missing escaped news row: %s", got)
	}
}

func TestRenderNewsMarkdownEnhancedNewspaper(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentNews, TeamQuery: "Chicago Cubs"}
	rows := []NewsRow{
		{
			Published:   "May 7, 1:20 PM",
			Headline:    "Cubs make late move",
			Description: "Chicago gets late-inning help.",
			Byline:      "ESPN News",
			URL:         "https://www.espn.com/mlb/story/_/id/1/cubs-news",
			ImageURL:    "https://a.espncdn.com/photo/2026/0507/cubs.jpg",
			ImageAlt:    "Cubs celebration",
		},
		{
			Published:   "May 7, 12:00 PM",
			Headline:    "Bullpen notes",
			Description: "A quick look around the roster.",
			URL:         "https://www.espn.com/mlb/story/_/id/2/bullpen",
		},
	}

	got := RenderNewsMarkdown(req, "MLB", rows, fixedNow())
	if !strings.Contains(got, "#### Lead Story") || !strings.Contains(got, "#### More Headlines") {
		t.Fatalf("missing newspaper sections: %s", got)
	}
	if !strings.Contains(got, "![Sports news image: Cubs celebration](https://a.espncdn.com/photo/2026/0507/cubs.jpg)") {
		t.Fatalf("missing safe lead image: %s", got)
	}
	if !strings.Contains(got, "### [Cubs make late move](https://www.espn.com/mlb/story/_/id/1/cubs-news)") {
		t.Fatalf("missing linked lead headline: %s", got)
	}
	if !strings.Contains(got, "_May 7, 1:20 PM · ESPN News_") {
		t.Fatalf("missing metadata line: %s", got)
	}
	if !strings.Contains(got, "> Chicago gets late-inning help.") {
		t.Fatalf("missing lead summary: %s", got)
	}
}

func TestRenderNewsMarkdownRejectsUnsafeImageAndLink(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentNews}
	rows := []NewsRow{{
		Headline:    "Unsafe story",
		Description: "No unsafe URL should be emitted.",
		URL:         "javascript:alert(1)",
		ImageURL:    "data:image/svg+xml;base64,abc",
	}}

	got := RenderNewsMarkdown(req, "Sports", rows, fixedNow())
	if strings.Contains(got, "javascript:") || strings.Contains(got, "data:image") {
		t.Fatalf("unsafe URL emitted: %s", got)
	}
	if !strings.Contains(got, "### Unsafe story") {
		t.Fatalf("expected plain headline fallback: %s", got)
	}
}

func TestRenderBroadNewsMarkdown(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentNews}
	rows := []NewsRow{{Headline: "Latest headline"}}

	got := RenderNewsMarkdown(req, "Sports", rows, fixedNow())
	if !strings.Contains(got, "### Latest Sports News") {
		t.Fatalf("missing broad title: %s", got)
	}
}

func TestRenderOddsMarkdown(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentOdds, DateLabel: "Today", RenderMode: SportsRenderPlainMarkdown}
	cfg := LeagueConfig{DisplayName: "NBA"}
	rows := []OddsRow{
		{
			Date:          "May 7",
			Time:          "7:30 PM",
			AwayTeam:      "Boston Celtics",
			AwayMoneyLine: "-120",
			HomeTeam:      "New York Knicks",
			HomeMoneyLine: "+100",
			Spread:        "BOS -2.5",
			OverUnder:     "214.5",
			Provider:      "ESPN BET",
		},
	}

	got := RenderOddsMarkdown(req, cfg, rows, fixedNow())
	if !strings.Contains(got, "### NBA Betting Odds — Today") {
		t.Fatalf("missing odds title: %s", got)
	}
	if !strings.Contains(got, "| Date | Time | Away | Away ML | Home | Home ML | Spread | O/U | Provider |") {
		t.Fatalf("missing odds table header: %s", got)
	}
	if !strings.Contains(got, "| May 7 | 7:30 PM | Boston Celtics | -120 | New York Knicks | +100 | BOS -2.5 | 214.5 | ESPN BET |") {
		t.Fatalf("missing odds row: %s", got)
	}
}
