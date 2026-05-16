package sports

import (
	"strconv"
	"strings"
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
		{"IPL standings", true, SportsIntentStandings, LeagueIPL, "", ""},
		{"Indian Premier League points table", true, SportsIntentStandings, LeagueIPL, "", ""},
		{"CSK score today", true, SportsIntentScores, LeagueIPL, "Chennai Super Kings", "Today"},
		{"latest Mumbai Indians news", true, SportsIntentNews, LeagueIPL, "Mumbai Indians", ""},
		{"current NHL standings", true, SportsIntentStandings, espn.LeagueNHL, "", "Current"},
		{"college football scores yesterday", true, SportsIntentScores, espn.LeagueCollegeFootball, "", "Yesterday"},
		{"show me NBA scores in a table", true, SportsIntentScores, espn.LeagueNBA, "", ""},
		{"What'st the latest sports news?", true, SportsIntentNews, "", "", ""},
		{"What's the latest ESPN news?", true, SportsIntentNews, "", "", ""},
		{"ESPN headlines", true, SportsIntentNews, "", "", ""},
		{"What the latest Chicago Cubs news?", true, SportsIntentNews, espn.LeagueMLB, "Chicago Cubs", ""},
		{"latest MLB news", true, SportsIntentNews, espn.LeagueMLB, "", ""},
		{"latest NBA scores", true, SportsIntentScores, espn.LeagueNBA, "", ""},
		{"show me MLB standings in a table", true, SportsIntentStandings, espn.LeagueMLB, "", ""},
		{"Chicago Cubs roster", true, SportsIntentRoster, espn.LeagueMLB, "Chicago Cubs", ""},
		{"Cubs schedule 2025", true, SportsIntentTeamSchedule, espn.LeagueMLB, "Chicago Cubs", ""},
		{"Yankees injuries", true, SportsIntentInjuries, espn.LeagueMLB, "New York Yankees", ""},
		{"latest MLB transactions", true, SportsIntentTransactions, espn.LeagueMLB, "", ""},
		{"Yankees record", true, SportsIntentTeamRecord, espn.LeagueMLB, "New York Yankees", ""},
		{"show me NBA odds today", true, SportsIntentOdds, espn.LeagueNBA, "", "Today"},
		{"what are the NFL spreads tomorrow", true, SportsIntentOdds, espn.LeagueNFL, "", "Tomorrow"},
		{"Cubs betting odds", true, SportsIntentOdds, espn.LeagueMLB, "Chicago Cubs", ""},
		{"who is favored in the Cubs game today", true, SportsIntentOdds, espn.LeagueMLB, "Chicago Cubs", "Today"},
		{"current betting odds", true, SportsIntentOdds, "", "", "Today"},
		{"Shohei Ohtani stats 2025", true, SportsIntentAthleteStats, "", "", ""},
		{"Patrick Mahomes game log", true, SportsIntentAthleteStats, "", "", ""},
		{"write a story about baseball", false, SportsIntentUnknown, "", "", ""},
		{"write a sports news article", false, SportsIntentUnknown, "", "", ""},
		{"explain how betting odds work", false, SportsIntentUnknown, "", "", ""},
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
									Logos: []espn.Logo{
										{Href: "http://a.espncdn.com/i/teamlogos/mlb/500/chc.png", Rel: []string{"default"}},
									},
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
	if row.StatusType != "final" {
		t.Fatalf("status type = %q, want final", row.StatusType)
	}
	if row.Away.LogoURL != "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png" {
		t.Fatalf("away logo = %q", row.Away.LogoURL)
	}

	filtered := filterGameRowsByTeam(rows, "Cubs")
	if len(filtered) != 1 {
		t.Fatalf("filtered rows = %d, want 1", len(filtered))
	}
	if filtered[0].AwayAbbr != "CHC" {
		t.Fatalf("filtered wrong row: %+v", filtered[0])
	}
}

func TestExtractTeamIdentityFromCompetitor(t *testing.T) {
	identity := extractTeamIdentityFromCompetitor(espn.Competitor{
		Team: espn.Team{
			DisplayName:      "Chicago Cubs",
			ShortDisplayName: "Cubs",
			Location:         "Chicago",
			Abbreviation:     "CHC",
			Color:            "0E3386",
			AlternateColor:   "CC3433",
			Logos: []espn.Logo{
				{Href: "javascript:alert(1)", Rel: []string{"default"}},
				{Href: "https://a.espncdn.com/i/teamlogos/mlb/500/chc-dark.png", Rel: []string{"dark"}},
				{Href: "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png", Rel: []string{"default"}},
			},
		},
	})
	if identity.DisplayName != "Chicago Cubs" || identity.Abbreviation != "CHC" {
		t.Fatalf("identity basics = %+v", identity)
	}
	if identity.LogoURL != "https://a.espncdn.com/i/teamlogos/mlb/500/chc.png" {
		t.Fatalf("logo = %q", identity.LogoURL)
	}
	if identity.DarkLogoURL != "https://a.espncdn.com/i/teamlogos/mlb/500/chc-dark.png" {
		t.Fatalf("dark logo = %q", identity.DarkLogoURL)
	}
	if identity.PrimaryColor != "#0E3386" || identity.AlternateColor != "#CC3433" {
		t.Fatalf("colors = %q/%q", identity.PrimaryColor, identity.AlternateColor)
	}
}

func TestExtractTeamIdentityRejectsInvalidLogo(t *testing.T) {
	identity := extractTeamIdentityFromCompetitor(espn.Competitor{
		Team: espn.Team{
			DisplayName:  "Bad Logo",
			Abbreviation: "BAD",
			Logo:         "data:image/svg+xml;base64,abc",
		},
	})
	if identity.LogoURL != "" {
		t.Fatalf("logo = %q, want empty", identity.LogoURL)
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

func TestCricketStandingsStatExtraction(t *testing.T) {
	entry := espn.StandingsEntry{
		Team: espn.Team{DisplayName: "Chennai Super Kings", Abbreviation: "CSK"},
		Stats: []espn.Statistic{
			{Name: "rank", DisplayValue: "1"},
			{Name: "matchesWon", DisplayValue: "5"},
			{Name: "matchesLost", DisplayValue: "5"},
		},
	}
	entry.Stats = append(entry.Stats,
		espn.Statistic{Abbreviation: "M", DisplayValue: "10"},
		espn.Statistic{Abbreviation: "T", DisplayValue: "0"},
		espn.Statistic{Abbreviation: "N/R", DisplayValue: "1"},
		espn.Statistic{Abbreviation: "PT", DisplayValue: "11"},
		espn.Statistic{Abbreviation: "NRR", DisplayValue: "0.151"},
		espn.Statistic{Abbreviation: "FOR", DisplayValue: "1815/195.4"},
		espn.Statistic{DisplayName: "Against", DisplayValue: "1711/187.3"},
	)

	row := standingsRowFromEntry("Indian Premier League", entry)
	if row.GamesPlayed != "10" || row.Wins != "5" || row.Losses != "5" || row.Ties != "0" || row.NoResult != "1" {
		t.Fatalf("cricket record stats not extracted: %+v", row)
	}
	if row.Points != "11" || row.NetRunRate != "0.151" || row.For != "1815/195.4" || row.Against != "1711/187.3" {
		t.Fatalf("cricket table stats not extracted: %+v", row)
	}
}

func TestStandingsPlayoffSeedDoesNotOverrideDisplayOrder(t *testing.T) {
	entry := standingsEntry("Tampa Bay Rays", "TB", 0, "25", "12")
	entry.Stats = append(entry.Stats,
		espn.Statistic{Name: "playoffSeed", DisplayValue: "4"},
		espn.Statistic{Name: "winPercent", DisplayValue: ".676"},
	)

	row := standingsRowFromEntry("American League", entry)
	if row.Rank != 0 {
		t.Fatalf("rank = %d, want 0 when ESPN only returns playoffSeed", row.Rank)
	}
	got := renderDefaultStandingsTable([]StandingsRow{
		{Rank: 1, Team: "New York Yankees", Abbr: "NYY", Wins: "26", Losses: "12", Pct: ".684"},
		row,
	}, false, SportsRenderPlainMarkdown)
	if !strings.Contains(got, "| 2 | Tampa Bay Rays | 25 | 12 | .676 | — | — |") {
		t.Fatalf("expected second displayed row to render rank 2, got: %s", got)
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
				Images: []espn.Image{
					{URL: "http://a.espncdn.com/photo/2026/0507/cubs.jpg", Alt: "Cubs image"},
				},
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
	if rows[0].ImageURL != "https://a.espncdn.com/photo/2026/0507/cubs.jpg" || rows[0].ImageAlt != "Cubs image" {
		t.Fatalf("unexpected image fields: %+v", rows[0])
	}
}

func TestNormalizeOddsAndFilter(t *testing.T) {
	awayOdds := &espn.TeamOdds{MoneyLine: 120, Underdog: true, SpreadOdds: -110}
	homeOdds := &espn.TeamOdds{MoneyLine: -140, Favorite: true, SpreadOdds: -110}
	odds := espn.OddsSummary{
		Details:      "CHC -1.5",
		OverUnder:    8.5,
		OverOdds:     -105,
		UnderOdds:    -115,
		AwayTeamOdds: awayOdds,
		HomeTeamOdds: homeOdds,
	}
	odds.Provider.Name = "ESPN BET"
	sb := &espn.Scoreboard{
		Events: []espn.Event{
			{
				Date: "2026-05-07T18:20:00Z",
				Status: espn.Status{Type: espn.StatusType{
					ShortDetail: "7:20 PM",
					State:       "pre",
				}},
				Competitions: []espn.Competition{
					{
						Date: "2026-05-07T18:20:00Z",
						Competitors: []espn.Competitor{
							{
								HomeAway: "away",
								Team: espn.Team{
									DisplayName:  "St. Louis Cardinals",
									Abbreviation: "STL",
								},
							},
							{
								HomeAway: "home",
								Team: espn.Team{
									DisplayName:  "Chicago Cubs",
									Abbreviation: "CHC",
								},
							},
						},
						Odds: []espn.OddsSummary{odds},
					},
				},
			},
			{
				Date: "2026-05-07T20:00:00Z",
				Competitions: []espn.Competition{
					{Competitors: []espn.Competitor{{Team: espn.Team{DisplayName: "Away"}}}},
				},
			},
		},
	}

	rows := normalizeOdds(sb, LeagueConfig{DisplayName: "MLB"})
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Provider != "ESPN BET" || row.Spread != "CHC -1.5" || row.OverUnder != "8.5 (O -105 / U -115)" {
		t.Fatalf("unexpected odds row: %+v", row)
	}
	if row.AwayMoneyLine != "+120" || row.HomeMoneyLine != "-140" {
		t.Fatalf("moneylines = %q/%q", row.AwayMoneyLine, row.HomeMoneyLine)
	}
	filtered := filterOddsRowsByTeam(rows, "Cubs")
	if len(filtered) != 1 {
		t.Fatalf("filtered rows = %d, want 1", len(filtered))
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
				Images:      []espn.Image{{URL: "https://a.espncdn.com/photo/2026/0507/news.jpg", Caption: "News image"}},
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
	if rows[0].ImageURL == "" || rows[0].ImageAlt != "News image" {
		t.Fatalf("unexpected now image row: %+v", rows[0])
	}
}

func TestNormalizeNowNewsPayloadHeadlines(t *testing.T) {
	raw := []byte(`{
		"resultsCount": 1,
		"headlines": [
			{
				"headline": "Latest ESPN headline",
				"description": "<p>A broad ESPN sports update.</p>",
				"published": "2026-05-07T18:20:00Z",
				"links": {
					"web": {"href": "https://www.espn.com/story/_/id/1/latest-news"}
				},
				"images": [
					{"url": "https://a.espncdn.com/photo/2026/0507/news.jpg", "alt": "ESPN news image"}
				]
			}
		]
	}`)

	rows, err := normalizeNowNewsPayload(raw)
	if err != nil {
		t.Fatalf("normalizeNowNewsPayload error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Headline != "Latest ESPN headline" || rows[0].Description != "A broad ESPN sports update." {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
	if rows[0].URL != "https://www.espn.com/story/_/id/1/latest-news" {
		t.Fatalf("url = %q", rows[0].URL)
	}
	if rows[0].ImageURL == "" || rows[0].ImageAlt != "ESPN news image" {
		t.Fatalf("unexpected image row: %+v", rows[0])
	}
}

func TestNormalizeNowNewsPayloadFallbackTitleAndLinkArray(t *testing.T) {
	raw := []byte(`{
		"headlines": [
			{
				"title": "Title-only ESPN headline",
				"description": "A title fallback update.",
				"links": [
					{"href": "https://www.espn.com/title-only"}
				]
			}
		]
	}`)

	rows, err := normalizeNowNewsPayload(raw)
	if err != nil {
		t.Fatalf("normalizeNowNewsPayload error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Headline != "Title-only ESPN headline" || rows[0].URL != "https://www.espn.com/title-only" {
		t.Fatalf("unexpected fallback row: %+v", rows[0])
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

// TestDetectTeamNewsQueries validates that natural-language news questions for
// major teams across all four leagues are detected as SportsIntentNews with the
// correct league and team. This exercises the teamAliases map comprehensively,
// including teams that were previously missing and would silently return ok=false.
func TestDetectTeamNewsQueries(t *testing.T) {
	tests := []struct {
		query      string
		wantLeague string
		wantTeam   string
	}{
		// ── NFL ──────────────────────────────────────────────────────────────
		{"What's the latest Chicago Bears news?", espn.LeagueNFL, "Chicago Bears"},
		{"What's the latest Kansas City Chiefs news?", espn.LeagueNFL, "Kansas City Chiefs"},
		{"Baltimore Ravens news", espn.LeagueNFL, "Baltimore Ravens"},
		{"Pittsburgh Steelers latest headlines", espn.LeagueNFL, "Pittsburgh Steelers"},
		{"What's the latest Denver Broncos news?", espn.LeagueNFL, "Denver Broncos"},
		{"Las Vegas Raiders news", espn.LeagueNFL, "Las Vegas Raiders"},
		{"Seattle Seahawks news", espn.LeagueNFL, "Seattle Seahawks"},
		{"Miami Dolphins latest news", espn.LeagueNFL, "Miami Dolphins"},
		{"Minnesota Vikings news", espn.LeagueNFL, "Minnesota Vikings"},
		{"What's the latest Tampa Bay Buccaneers news?", espn.LeagueNFL, "Tampa Bay Buccaneers"},
		{"Cincinnati Bengals news", espn.LeagueNFL, "Cincinnati Bengals"},
		{"New York Giants latest headlines", espn.LeagueNFL, "New York Giants"},
		{"Houston Texans news", espn.LeagueNFL, "Houston Texans"},
		{"Cleveland Browns news", espn.LeagueNFL, "Cleveland Browns"},
		{"Washington Commanders latest news", espn.LeagueNFL, "Washington Commanders"},
		{"Jacksonville Jaguars news", espn.LeagueNFL, "Jacksonville Jaguars"},
		{"Tennessee Titans latest headlines", espn.LeagueNFL, "Tennessee Titans"},
		{"New Orleans Saints news", espn.LeagueNFL, "New Orleans Saints"},
		{"Atlanta Falcons news", espn.LeagueNFL, "Atlanta Falcons"},
		{"Los Angeles Rams latest news", espn.LeagueNFL, "Los Angeles Rams"},
		// ── MLB ──────────────────────────────────────────────────────────────
		{"What the latest Chicago Cubs news?", espn.LeagueMLB, "Chicago Cubs"},
		{"Houston Astros latest news", espn.LeagueMLB, "Houston Astros"},
		{"Texas Rangers news", espn.LeagueMLB, "Texas Rangers"},
		{"Toronto Blue Jays news", espn.LeagueMLB, "Toronto Blue Jays"},
		{"Baltimore Orioles latest headlines", espn.LeagueMLB, "Baltimore Orioles"},
		{"Seattle Mariners news", espn.LeagueMLB, "Seattle Mariners"},
		{"Cleveland Guardians news", espn.LeagueMLB, "Cleveland Guardians"},
		{"Detroit Tigers latest news", espn.LeagueMLB, "Detroit Tigers"},
		{"Kansas City Royals news", espn.LeagueMLB, "Kansas City Royals"},
		{"Milwaukee Brewers news", espn.LeagueMLB, "Milwaukee Brewers"},
		{"Minnesota Twins latest headlines", espn.LeagueMLB, "Minnesota Twins"},
		{"Colorado Rockies news", espn.LeagueMLB, "Colorado Rockies"},
		{"Arizona Diamondbacks news", espn.LeagueMLB, "Arizona Diamondbacks"},
		{"Washington Nationals latest news", espn.LeagueMLB, "Washington Nationals"},
		{"Cincinnati Reds news", espn.LeagueMLB, "Cincinnati Reds"},
		// ── NBA ──────────────────────────────────────────────────────────────
		{"What's the latest Los Angeles Lakers news?", espn.LeagueNBA, "Los Angeles Lakers"},
		{"Houston Rockets news", espn.LeagueNBA, "Houston Rockets"},
		{"Oklahoma City Thunder latest news", espn.LeagueNBA, "Oklahoma City Thunder"},
		{"Milwaukee Bucks news", espn.LeagueNBA, "Milwaukee Bucks"},
		{"Brooklyn Nets latest headlines", espn.LeagueNBA, "Brooklyn Nets"},
		{"Toronto Raptors news", espn.LeagueNBA, "Toronto Raptors"},
		{"Philadelphia 76ers news", espn.LeagueNBA, "Philadelphia 76ers"},
		{"San Antonio Spurs latest news", espn.LeagueNBA, "San Antonio Spurs"},
		{"Cleveland Cavaliers news", espn.LeagueNBA, "Cleveland Cavaliers"},
		{"Indiana Pacers news", espn.LeagueNBA, "Indiana Pacers"},
		{"Atlanta Hawks latest headlines", espn.LeagueNBA, "Atlanta Hawks"},
		{"Memphis Grizzlies news", espn.LeagueNBA, "Memphis Grizzlies"},
		{"Charlotte Hornets news", espn.LeagueNBA, "Charlotte Hornets"},
		{"Minnesota Timberwolves latest news", espn.LeagueNBA, "Minnesota Timberwolves"},
		// ── NHL ──────────────────────────────────────────────────────────────
		{"What's the latest Boston Bruins news?", espn.LeagueNHL, "Boston Bruins"},
		{"Pittsburgh Penguins news", espn.LeagueNHL, "Pittsburgh Penguins"},
		{"Washington Capitals latest headlines", espn.LeagueNHL, "Washington Capitals"},
		{"Edmonton Oilers news", espn.LeagueNHL, "Edmonton Oilers"},
		{"Carolina Hurricanes news", espn.LeagueNHL, "Carolina Hurricanes"},
		{"Montreal Canadiens latest news", espn.LeagueNHL, "Montreal Canadiens"},
		{"Seattle Kraken news", espn.LeagueNHL, "Seattle Kraken"},
		{"Vancouver Canucks news", espn.LeagueNHL, "Vancouver Canucks"},
		{"Nashville Predators latest headlines", espn.LeagueNHL, "Nashville Predators"},
		{"Detroit Red Wings news", espn.LeagueNHL, "Detroit Red Wings"},
		{"Calgary Flames news", espn.LeagueNHL, "Calgary Flames"},
		{"Buffalo Sabres latest news", espn.LeagueNHL, "Buffalo Sabres"},
		{"Philadelphia Flyers news", espn.LeagueNHL, "Philadelphia Flyers"},
		{"Dallas Stars news", espn.LeagueNHL, "Dallas Stars"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent returned false")
			}
			if got.Intent != SportsIntentNews {
				t.Fatalf("intent = %q, want news", got.Intent)
			}
			if got.League != tt.wantLeague {
				t.Fatalf("league = %q, want %q", got.League, tt.wantLeague)
			}
			if got.TeamQuery != tt.wantTeam {
				t.Fatalf("team = %q, want %q", got.TeamQuery, tt.wantTeam)
			}
		})
	}
}

// TestDetectStatsLeaderQueries validates that 25 natural-language stat questions
// across NFL, MLB, NBA, and NHL are detected as SportsIntentLeaders with the
// correct league, sort key, and season year.
func TestDetectStatsLeaderQueries(t *testing.T) {
	tests := []struct {
		query        string
		wantLeague   string
		wantStatSort string
		wantSeason   int
	}{
		// ── NFL ──────────────────────────────────────────────────────────────
		{
			query:        "Who had the most rushing TDs in the NFL in 1999",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "rushing.rushingTouchdowns:desc",
			wantSeason:   1999,
		},
		{
			query:        "Who had the most rushing TBs in the NFL in 1999",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "rushing.rushingTouchdowns:desc",
			wantSeason:   1999,
		},
		{
			query:        "most rushing yards in the NFL 2023",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "rushing.rushingYards:desc",
			wantSeason:   2023,
		},
		{
			query:        "top receiving yards leaders in the NFL 2024",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "receiving.receivingYards:desc",
			wantSeason:   2024,
		},
		{
			query:        "most receiving touchdowns in the NFL 2022",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "receiving.receivingTouchdowns:desc",
			wantSeason:   2022,
		},
		{
			query:        "most receptions in the NFL 2024",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "receiving.receptions:desc",
			wantSeason:   2024,
		},
		{
			query:        "most catches in the NFL 2023",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "receiving.receptions:desc",
			wantSeason:   2023,
		},
		{
			query:        "NFL interceptions leaders 2021",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "defensive.interceptions:desc",
			wantSeason:   2021,
		},
		{
			query:        "most picks in the NFL 2020",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "defensive.interceptions:desc",
			wantSeason:   2020,
		},
		{
			query:        "most sacks in the NFL 2023",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "defensive.sacks:desc",
			wantSeason:   2023,
		},
		{
			query:        "most passing touchdowns in the NFL 2024",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "passing.passingTouchdowns:desc",
			wantSeason:   2024,
		},
		{
			query:        "top passing yards leaders in the NFL",
			wantLeague:   espn.LeagueNFL,
			wantStatSort: "passing.passingYards:desc",
			wantSeason:   0,
		},
		// ── MLB ──────────────────────────────────────────────────────────────
		{
			query:        "who led MLB in home runs in 2019",
			wantLeague:   espn.LeagueMLB,
			wantStatSort: "batting.homeRuns:desc",
			wantSeason:   2019,
		},
		{
			query:        "most RBIs in MLB 2021",
			wantLeague:   espn.LeagueMLB,
			wantStatSort: "batting.RBIs:desc",
			wantSeason:   2021,
		},
		{
			query:        "MLB batting average leaders 2022",
			wantLeague:   espn.LeagueMLB,
			wantStatSort: "batting.avg:desc",
			wantSeason:   2022,
		},
		{
			query:        "most strikeouts leaders MLB 2023",
			wantLeague:   espn.LeagueMLB,
			wantStatSort: "pitching.strikeouts:desc",
			wantSeason:   2023,
		},
		{
			query:        "MLB ERA leaders 2024",
			wantLeague:   espn.LeagueMLB,
			wantStatSort: "pitching.ERA:asc",
			wantSeason:   2024,
		},
		{
			query:        "most stolen bases in MLB 2022",
			wantLeague:   espn.LeagueMLB,
			wantStatSort: "batting.stolenBases:desc",
			wantSeason:   2022,
		},
		// ── NBA ──────────────────────────────────────────────────────────────
		{
			query:        "NBA points per game leaders 2024",
			wantLeague:   espn.LeagueNBA,
			wantStatSort: "offensive.avgPoints:desc",
			wantSeason:   2024,
		},
		{
			query:        "most rebounds per game in the NBA 2023",
			wantLeague:   espn.LeagueNBA,
			wantStatSort: "general.avgRebounds:desc",
			wantSeason:   2023,
		},
		{
			query:        "most assists per game in the NBA 2022",
			wantLeague:   espn.LeagueNBA,
			wantStatSort: "offensive.avgAssists:desc",
			wantSeason:   2022,
		},
		{
			query:        "NBA steals leaders 2021",
			wantLeague:   espn.LeagueNBA,
			wantStatSort: "defensive.avgSteals:desc",
			wantSeason:   2021,
		},
		{
			query:        "NBA blocks leaders 2023",
			wantLeague:   espn.LeagueNBA,
			wantStatSort: "defensive.avgBlocks:desc",
			wantSeason:   2023,
		},
		// ── NHL ──────────────────────────────────────────────────────────────
		{
			query:        "most goals in the NHL 2024",
			wantLeague:   espn.LeagueNHL,
			wantStatSort: "scoring.goals:desc",
			wantSeason:   2024,
		},
		{
			query:        "NHL points leaders 2023",
			wantLeague:   espn.LeagueNHL,
			wantStatSort: "scoring.points:desc",
			wantSeason:   2023,
		},
		{
			query:        "most assists in the NHL 2022",
			wantLeague:   espn.LeagueNHL,
			wantStatSort: "scoring.assists:desc",
			wantSeason:   2022,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, ok := DetectSportsIntent(tt.query, fixedNow())
			if !ok {
				t.Fatalf("DetectSportsIntent returned false")
			}
			if got.Intent != SportsIntentLeaders {
				t.Fatalf("intent = %q, want leaders", got.Intent)
			}
			if got.League != tt.wantLeague {
				t.Fatalf("league = %q, want %q", got.League, tt.wantLeague)
			}
			if got.StatSort != tt.wantStatSort {
				t.Fatalf("stat sort = %q, want %q", got.StatSort, tt.wantStatSort)
			}
			if got.Season != tt.wantSeason {
				t.Fatalf("season = %d, want %d", got.Season, tt.wantSeason)
			}
		})
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
