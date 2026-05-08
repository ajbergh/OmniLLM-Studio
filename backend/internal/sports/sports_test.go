package sports

import (
	"strconv"
	"testing"
	"time"

	espn "github.com/chinmaykhachane/espn-go"
)

func fixedNow() time.Time {
	return time.Date(2026, 5, 7, 18, 45, 0, 0, time.Local)
}

func TestDetectSportsIntent(t *testing.T) {
	tests := []struct {
		query      string
		wantOK     bool
		wantIntent SportsIntentType
		wantLeague string
		wantTeam   string
		wantLabel  string
	}{
		{"What are the current MLB standings?", true, SportsIntentStandings, espn.LeagueMLB, "", "Current"},
		{"show me NBA scores today", true, SportsIntentScores, espn.LeagueNBA, "", "Today"},
		{"what NFL games are on tomorrow", true, SportsIntentSchedule, espn.LeagueNFL, "", "Tomorrow"},
		{"Yankees score", true, SportsIntentScores, espn.LeagueMLB, "New York Yankees", ""},
		{"Premier League table", true, SportsIntentStandings, espn.LeagueEPL, "", ""},
		{"current NHL standings", true, SportsIntentStandings, espn.LeagueNHL, "", "Current"},
		{"college football scores yesterday", true, SportsIntentScores, espn.LeagueCollegeFootball, "", "Yesterday"},
		{"show me NBA scores in a table", true, SportsIntentScores, espn.LeagueNBA, "", ""},
		{"What'st the latest sports news?", true, SportsIntentNews, "", "", ""},
		{"What the latest Chicago Cubs news?", true, SportsIntentNews, espn.LeagueMLB, "Chicago Cubs", ""},
		{"latest MLB news", true, SportsIntentNews, espn.LeagueMLB, "", ""},
		{"latest NBA scores", true, SportsIntentScores, espn.LeagueNBA, "", ""},
		{"show me MLB standings in a table", true, SportsIntentStandings, espn.LeagueMLB, "", ""},
		{"Chicago Cubs roster", true, SportsIntentRoster, espn.LeagueMLB, "Chicago Cubs", ""},
		{"Cubs schedule 2025", true, SportsIntentTeamSchedule, espn.LeagueMLB, "Chicago Cubs", ""},
		{"Yankees injuries", true, SportsIntentInjuries, espn.LeagueMLB, "New York Yankees", ""},
		{"latest MLB transactions", true, SportsIntentTransactions, espn.LeagueMLB, "", ""},
		{"Yankees record", true, SportsIntentTeamRecord, espn.LeagueMLB, "New York Yankees", ""},
		{"Shohei Ohtani stats 2025", true, SportsIntentAthleteStats, "", "", ""},
		{"Patrick Mahomes game log", true, SportsIntentAthleteStats, "", "", ""},
		{"write a story about baseball", false, SportsIntentUnknown, "", "", ""},
		{"write a sports news article", false, SportsIntentUnknown, "", "", ""},
		{"explain how MLB standings work", false, SportsIntentUnknown, "", "", ""},
		{"who is the greatest baseball player ever", false, SportsIntentUnknown, "", "", ""},
		{"make a sports logo", false, SportsIntentUnknown, "", "", ""},
		{"compare football and baseball", false, SportsIntentUnknown, "", "", ""},
		{"latest sports movies", false, SportsIntentUnknown, "", "", ""},
		{"Print out the top 50 in HR for the 2025 MLB season in a table", true, SportsIntentLeaders, espn.LeagueMLB, "", ""},
		{"top 50 home run leaders for the 2025 MLB season", true, SportsIntentLeaders, espn.LeagueMLB, "", ""},
		{"MLB leaders", true, SportsIntentLeaders, espn.LeagueMLB, "", ""},
		{"college football rankings", true, SportsIntentRankings, espn.LeagueCollegeFootball, "", ""},
		{"Cubs stats", true, SportsIntentLeagueStats, espn.LeagueMLB, "Chicago Cubs", ""},
		{"MLB table", false, SportsIntentUnknown, "", "", ""},
		{"MLB stats", true, SportsIntentLeagueStats, espn.LeagueMLB, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Intent != tt.wantIntent {
				t.Fatalf("intent = %q, want %q", got.Intent, tt.wantIntent)
			}
			if got.League != tt.wantLeague {
				t.Fatalf("league = %q, want %q", got.League, tt.wantLeague)
			}
			if got.TeamQuery != tt.wantTeam {
				t.Fatalf("team = %q, want %q", got.TeamQuery, tt.wantTeam)
			}
			if got.DateLabel != tt.wantLabel {
				t.Fatalf("date label = %q, want %q", got.DateLabel, tt.wantLabel)
			}
		})
	}
}

func TestDetectSportsLeaderboardDetails(t *testing.T) {
	got, ok := DetectSportsIntent("Print out the top 50 in HR for the 2025 MLB season in a table", fixedNow())
	if !ok {
		t.Fatal("expected sports lookup")
	}
	if got.Intent != SportsIntentLeaders || got.League != espn.LeagueMLB {
		t.Fatalf("intent/league = %s/%s", got.Intent, got.League)
	}
	if got.Season != 2025 || got.Limit != 50 {
		t.Fatalf("season/limit = %d/%d", got.Season, got.Limit)
	}
	if got.StatCategory != "batting" || got.StatName != "homeRuns" || got.StatLabel != "HR" {
		t.Fatalf("stat mapping = category %q name %q label %q", got.StatCategory, got.StatName, got.StatLabel)
	}
}

func TestDetectSportsAthleteDetails(t *testing.T) {
	got, ok := DetectSportsIntent("Shohei Ohtani stats 2025", fixedNow())
	if !ok {
		t.Fatal("expected athlete stats lookup")
	}
	if got.Intent != SportsIntentAthleteStats {
		t.Fatalf("intent = %s", got.Intent)
	}
	if got.AthleteQuery != "shohei ohtani" {
		t.Fatalf("athlete query = %q", got.AthleteQuery)
	}
	if got.Season != 2025 {
		t.Fatalf("season = %d", got.Season)
	}
}

func TestParseDateValue(t *testing.T) {
	tests := []struct {
		value     string
		wantLabel string
		wantDate  string
	}{
		{"today", "Today", "2026-05-07"},
		{"tonight", "Tonight", "2026-05-07"},
		{"yesterday", "Yesterday", "2026-05-06"},
		{"tomorrow", "Tomorrow", "2026-05-08"},
		{"2026-06-01", "Jun 1, 2026", "2026-06-01"},
		{"6/2/2026", "Jun 2, 2026", "2026-06-02"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, label, err := ParseDateValue(tt.value, fixedNow(), SportsIntentScores)
			if err != nil {
				t.Fatalf("ParseDateValue error: %v", err)
			}
			if label != tt.wantLabel {
				t.Fatalf("label = %q, want %q", label, tt.wantLabel)
			}
			if got == nil {
				t.Fatalf("date is nil")
			}
			if got.Format("2006-01-02") != tt.wantDate {
				t.Fatalf("date = %s, want %s", got.Format("2006-01-02"), tt.wantDate)
			}
		})
	}
}

func TestValidateDateInQuery(t *testing.T) {
	if err := ValidateDateInQuery("MLB scores on 2026-05-07", fixedNow()); err != nil {
		t.Fatalf("expected valid date, got %v", err)
	}
	if err := ValidateDateInQuery("MLB scores on 2026-99-99", fixedNow()); err == nil {
		t.Fatal("expected malformed date error")
	}
}

func TestNormalizeScoreboardAndFilter(t *testing.T) {
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			{
				Date: "2026-05-07T18:20:00Z",
				Status: espn.Status{Type: espn.StatusType{
					ShortDetail: "Final",
					Completed:   true,
					State:       "post",
				}},
				Competitions: []espn.Competition{
					{
						Date:  "2026-05-07T18:20:00Z",
						Venue: &espn.Venue{FullName: "Busch Stadium"},
						Broadcasts: []espn.Broadcast{
							{Names: []string{"ESPN"}},
						},
						Competitors: []espn.Competitor{
							{
								HomeAway: "away",
								Score:    "5",
								Team: espn.Team{
									DisplayName:  "Chicago Cubs",
									Abbreviation: "CHC",
								},
							},
							{
								HomeAway: "home",
								Score:    "3",
								Team: espn.Team{
									DisplayName:  "St. Louis Cardinals",
									Abbreviation: "STL",
								},
							},
						},
					},
				},
			},
			{
				Date: "2026-05-07T20:00:00Z",
			},
		},
	}

	rows := normalizeScoreboard(sb)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.AwayTeam != "Chicago Cubs" || row.HomeTeam != "St. Louis Cardinals" {
		t.Fatalf("teams = %q at %q", row.AwayTeam, row.HomeTeam)
	}
	if row.AwayScore != "5" || row.HomeScore != "3" || row.Venue != "Busch Stadium" || row.Broadcasts != "ESPN" {
		t.Fatalf("unexpected row: %+v", row)
	}

	filtered := filterGameRowsByTeam(rows, "Cubs")
	if len(filtered) != 1 {
		t.Fatalf("filtered rows = %d, want 1", len(filtered))
	}
	if filtered[0].AwayAbbr != "CHC" {
		t.Fatalf("filtered wrong row: %+v", filtered[0])
	}
}

func TestNormalizeScoreboardNoPanicOnMissingFields(t *testing.T) {
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			{},
			{Competitions: []espn.Competition{{}}},
			{Competitions: []espn.Competition{{Competitors: []espn.Competitor{{Team: espn.Team{DisplayName: "Away"}}}}}},
		},
	}
	rows := normalizeScoreboard(sb)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
}

func TestCollectStandingsRowsNested(t *testing.T) {
	root := espn.StandingsGroup{
		Name: "MLB",
		Children: []espn.StandingsGroup{
			{
				Name: "American League",
				Children: []espn.StandingsGroup{
					{
						Name: "East",
						Standings: &espn.StandingsGroupEntries{
							Entries: []espn.StandingsEntry{
								standingsEntry("New York Yankees", "NYY", 1, "26", "12"),
							},
						},
					},
				},
			},
			{
				Name: "National League West",
				Standings: &espn.StandingsGroupEntries{
					Entries: []espn.StandingsEntry{
						standingsEntry("Los Angeles Dodgers", "LAD", 1, "24", "14"),
					},
				},
			},
		},
	}

	var rows []StandingsRow
	collectStandingsRows("", root, &rows)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Group != "American League East" {
		t.Fatalf("group[0] = %q", rows[0].Group)
	}
	if rows[1].Group != "National League West" {
		t.Fatalf("group[1] = %q", rows[1].Group)
	}
}

func TestStandingsStatExtraction(t *testing.T) {
	entry := standingsEntry("New York Yankees", "NYY", 2, "26", "12")
	entry.Stats = append(entry.Stats,
		espn.Statistic{Name: "winPercent", Value: 0.684},
		espn.Statistic{Abbreviation: "GB", DisplayValue: "0.5"},
		espn.Statistic{Name: "streak", DisplayValue: "W2"},
	)

	row := standingsRowFromEntry("American League East", entry)
	if row.Rank != 2 || row.Wins != "26" || row.Losses != "12" {
		t.Fatalf("basic stats not extracted: %+v", row)
	}
	if row.Pct != ".684" || row.GamesBack != "0.5" || row.Streak != "W2" {
		t.Fatalf("display stats not extracted: %+v", row)
	}
}

func TestNormalizeNewsFeed(t *testing.T) {
	feed := &espn.NewsFeed{
		Articles: []espn.Article{
			{
				Headline:    "Cubs win | again",
				Description: "<p>Chicago &amp; Milwaukee split the series with a long summary that stays readable.</p>",
				Byline:      "ESPN News",
				Published:   "2026-05-07T18:20:00Z",
				Links:       espn.ArticleLinks{Web: &espn.Link{Href: "https://www.espn.com/mlb/story/_/id/1"}},
			},
			{},
		},
	}

	rows := normalizeNewsFeed(feed)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Headline != "Cubs win | again" {
		t.Fatalf("headline = %q", rows[0].Headline)
	}
	if rows[0].Description != "Chicago & Milwaukee split the series with a long summary that stays readable." {
		t.Fatalf("description = %q", rows[0].Description)
	}
	if rows[0].Published == "" || rows[0].URL == "" || rows[0].Byline != "ESPN News" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

func TestNormalizeNowFeed(t *testing.T) {
	feed := &espn.NowFeed{
		Feed: []espn.NowItem{
			{
				Headline:    "Latest sports headline",
				Description: "A national sports update.",
				Published:   "2026-05-07T18:20:00Z",
				Links:       espn.ArticleLinks{Web: &espn.Link{Href: "https://www.espn.com/"}},
			},
		},
	}

	rows := normalizeNowFeed(feed)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Headline != "Latest sports headline" || rows[0].Description != "A national sports update." {
		t.Fatalf("unexpected now row: %+v", rows[0])
	}
}

func TestNormalizeLeaderboard(t *testing.T) {
	raw := []byte(`{
		"requestedSeason": {"year": 2025, "displayName": "2025", "type": {"name": "Regular Season"}},
		"categories": [{"name": "batting", "labels": ["GP", "HR"], "names": ["gamesPlayed", "homeRuns"]}],
		"athletes": [
			{
				"athlete": {"displayName": "Cal Raleigh", "teamShortName": "SEA", "position": {"abbreviation": "C"}},
				"categories": [{"name": "batting", "totals": ["159", "60"], "ranks": ["16", "1"]}]
			},
			{
				"athlete": {"displayName": "Shohei Ohtani", "teamShortName": "LAD", "position": {"abbreviation": "DH"}},
				"categories": [{"name": "batting", "totals": ["158", "55"], "ranks": ["20", "2"]}]
			}
		]
	}`)
	req := SportsRequest{StatCategory: "batting", StatName: "homeRuns", StatLabel: "HR"}

	rows, label, season := normalizeLeaderboard(raw, req)
	if label != "HR" || season != "2025 Regular Season" {
		t.Fatalf("label/season = %q/%q", label, season)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Rank != 1 || rows[0].Athlete != "Cal Raleigh" || rows[0].Value != "60" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

func TestESPNTeamMatchesQuery(t *testing.T) {
	team := espn.Team{
		ID:               "16",
		DisplayName:      "Chicago Cubs",
		ShortDisplayName: "Cubs",
		Name:             "Cubs",
		Location:         "Chicago",
		Abbreviation:     "CHC",
		Slug:             "chicago-cubs",
	}
	if !espnTeamMatchesQuery(team, normalizeText("Cubs")) {
		t.Fatal("expected Cubs to match")
	}
	if espnTeamMatchesQuery(team, normalizeText("Yankees")) {
		t.Fatal("did not expect Yankees to match Cubs")
	}
}

func standingsEntry(team, abbr string, rank int, wins, losses string) espn.StandingsEntry {
	return espn.StandingsEntry{
		Team: espn.Team{DisplayName: team, Abbreviation: abbr},
		Stats: []espn.Statistic{
			{Name: "rank", DisplayValue: strconv.Itoa(rank)},
			{Name: "wins", DisplayValue: wins},
			{Name: "losses", DisplayValue: losses},
		},
	}
}
