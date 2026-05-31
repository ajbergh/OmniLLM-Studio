package sports

import (
	"errors"
	"strings"
	"testing"

	espn "github.com/chinmaykhachane/espn-go"
)

func TestValidateGameRowsSchedulePasses(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentSchedule, League: espn.LeagueMLB}
	rows := []GameRow{{
		AwayTeam: "Miami Marlins",
		HomeTeam: "Tampa Bay Rays",
		Time:     "1:05 PM",
	}}
	if err := ValidateGameRows(req, rows); err != nil {
		t.Fatalf("ValidateGameRows returned error: %v", err)
	}
}

func TestValidateGameRowsPitchingMatchupsPassesWithAtLeastOneMatchup(t *testing.T) {
	req := SportsRequest{
		Intent:            SportsIntentSchedule,
		League:            espn.LeagueMLB,
		GameDetailSubtype: "pitching_matchups",
	}
	rows := []GameRow{
		{AwayTeam: "Miami Marlins", HomeTeam: "Tampa Bay Rays"},
		{AwayTeam: "Boston Red Sox", HomeTeam: "Atlanta Braves", PitchingMatchup: "TBD vs Spencer Strider"},
	}
	if err := ValidateGameRows(req, rows); err != nil {
		t.Fatalf("ValidateGameRows returned error: %v", err)
	}
}

func TestValidateGameRowsPitchingMatchupsFailsWhenAllMatchupsMissing(t *testing.T) {
	req := SportsRequest{
		Intent:            SportsIntentSchedule,
		League:            espn.LeagueMLB,
		GameDetailSubtype: "pitching_matchups",
	}
	rows := []GameRow{
		{AwayTeam: "Miami Marlins", HomeTeam: "Tampa Bay Rays"},
		{AwayTeam: "Boston Red Sox", HomeTeam: "Atlanta Braves"},
	}
	err := ValidateGameRows(req, rows)
	if err == nil {
		t.Fatal("ValidateGameRows returned nil, want validation error")
	}
	if !errors.Is(err, ErrSportsResultMissingRequired) {
		t.Fatalf("error = %v, want ErrSportsResultMissingRequired", err)
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationMissingPitchingMatchups {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationMissingPitchingMatchups)
	}
	if validationErr.RetryHint != SportsRecoveryRetryRawScoreboardProbable {
		t.Fatalf("RetryHint = %q, want %q", validationErr.RetryHint, SportsRecoveryRetryRawScoreboardProbable)
	}
}

func TestValidateGameRowsPitchingMatchupsAllowsPartialPitcherData(t *testing.T) {
	req := SportsRequest{
		Intent:            SportsIntentScores,
		League:            espn.LeagueMLB,
		GameDetailSubtype: "pitching_matchups",
	}
	rows := []GameRow{
		{AwayTeam: "Miami Marlins", HomeTeam: "Tampa Bay Rays", PitchingMatchup: "Sandy Alcantara vs Drew Rasmussen"},
		{AwayTeam: "Boston Red Sox", HomeTeam: "Atlanta Braves"},
	}
	if err := ValidateGameRows(req, rows); err != nil {
		t.Fatalf("ValidateGameRows returned error: %v", err)
	}
}

func TestValidateGameRowsTeamQueryMustMatchReturnedRows(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentSchedule, League: espn.LeagueMLB, TeamQuery: "Chicago Cubs"}
	rows := []GameRow{{AwayTeam: "Miami Marlins", HomeTeam: "Tampa Bay Rays"}}
	err := ValidateGameRows(req, rows)
	if err == nil {
		t.Fatal("ValidateGameRows returned nil, want validation error")
	}
	if !errors.Is(err, ErrSportsResultMismatch) {
		t.Fatalf("error = %v, want ErrSportsResultMismatch", err)
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationMissingTeamMatch {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationMissingTeamMatch)
	}
}

func TestUserFacingErrorPitchingMatchups(t *testing.T) {
	req := SportsRequest{
		Intent:            SportsIntentSchedule,
		League:            espn.LeagueMLB,
		GameDetailSubtype: "pitching_matchups",
	}
	err := &SportsValidationError{
		Code: SportsValidationMissingPitchingMatchups,
		Err:  ErrSportsResultMissingRequired,
	}
	got := UserFacingError(req, err)
	if !strings.Contains(got, "probable pitchers") {
		t.Fatalf("UserFacingError = %q, want probable pitchers message", got)
	}
}

func TestPitchingMatchupQueryDoesNotAcceptGenericScheduleRows(t *testing.T) {
	req, ok := DetectSportsIntent("What are the pitching matchups for today's MLB games?", fixedNow())
	if !ok {
		t.Fatal("DetectSportsIntent did not handle pitching matchup query")
	}
	rows := []GameRow{{AwayTeam: "Miami Marlins", HomeTeam: "Tampa Bay Rays", Time: "1:05 PM"}}
	err := ValidateGameRows(*req, rows)
	if err == nil {
		t.Fatal("ValidateGameRows returned nil, want missing pitching matchups")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationMissingPitchingMatchups {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationMissingPitchingMatchups)
	}
}

func TestValidateGameRowsBroadcasts(t *testing.T) {
	req := SportsRequest{
		Intent:            SportsIntentSchedule,
		League:            espn.LeagueMLB,
		GameDetailSubtype: "broadcasts",
	}
	rows := []GameRow{{AwayTeam: "Miami Marlins", HomeTeam: "Tampa Bay Rays"}}
	err := ValidateGameRows(req, rows)
	if err == nil {
		t.Fatal("ValidateGameRows returned nil, want validation error")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationMissingBroadcasts {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationMissingBroadcasts)
	}
}

func TestValidateStandingsRowsNHLPointsPass(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, League: espn.LeagueNHL}
	rows := []StandingsRow{{Team: "Boston Bruins", Points: "96"}}
	if err := ValidateStandingsRows(req, rows); err != nil {
		t.Fatalf("ValidateStandingsRows returned error: %v", err)
	}
}

func TestValidateStandingsRowsSoccerMissingSignalsFails(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, League: espn.LeagueEPL}
	rows := []StandingsRow{{Team: "Arsenal"}}
	err := ValidateStandingsRows(req, rows)
	if err == nil {
		t.Fatal("ValidateStandingsRows returned nil, want validation error")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationMissingStandings {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationMissingStandings)
	}
}

func TestValidateStandingsRowsUnsupportedLeagueFails(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentStandings, League: "pga"}
	rows := []StandingsRow{{Team: "Scottie Scheffler", Rank: 1, Points: "100"}}
	err := ValidateStandingsRows(req, rows)
	if err == nil {
		t.Fatal("ValidateStandingsRows returned nil, want validation error")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationWrongResultType {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationWrongResultType)
	}
}

func TestValidateNewsRowsRequiresHeadline(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentNews}
	if err := ValidateNewsRows(req, []NewsRow{{Headline: "Trade deadline notes"}}); err != nil {
		t.Fatalf("ValidateNewsRows returned error: %v", err)
	}
	err := ValidateNewsRows(req, []NewsRow{{Description: "Missing headline"}})
	if err == nil {
		t.Fatal("ValidateNewsRows returned nil, want validation error")
	}
}

func TestValidateOddsRowsRequiresBettingField(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentOdds}
	if err := ValidateOddsRows(req, []OddsRow{{AwayTeam: "Cubs", HomeTeam: "White Sox", Spread: "CHC -1.5"}}); err != nil {
		t.Fatalf("ValidateOddsRows returned error: %v", err)
	}
	err := ValidateOddsRows(req, []OddsRow{{AwayTeam: "Cubs", HomeTeam: "White Sox"}})
	if err == nil {
		t.Fatal("ValidateOddsRows returned nil, want validation error")
	}
}

func TestValidateRosterRowsRequiresNames(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentRoster}
	if err := ValidateRosterRows(req, []RosterRow{{Name: "Connor McDavid", Position: "C"}}); err != nil {
		t.Fatalf("ValidateRosterRows returned error: %v", err)
	}
	err := ValidateRosterRows(req, []RosterRow{{Position: "C"}})
	if err == nil {
		t.Fatal("ValidateRosterRows returned nil, want validation error")
	}
}

func TestValidateLeaderboardRowsRequiresAthleteAndValue(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentLeaders}
	if err := ValidateLeaderboardRows(req, []LeaderboardRow{{Athlete: "Shohei Ohtani", Value: "42"}}); err != nil {
		t.Fatalf("ValidateLeaderboardRows returned error: %v", err)
	}
	err := ValidateLeaderboardRows(req, []LeaderboardRow{{Athlete: "Shohei Ohtani"}})
	if err == nil {
		t.Fatal("ValidateLeaderboardRows returned nil, want validation error")
	}
}

func TestValidateGameDetailTableRejectsSubtypeSummaryFallback(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentGameDetail, GameDetailSubtype: "officials"}
	table := SimpleTable{
		Headers: []string{"Field", "Value"},
		Rows:    [][]string{{"Name", "Cubs at Cardinals"}},
	}
	err := ValidateGameDetailTable(req, table, "### Summary: Cubs at Cardinals")
	if err == nil {
		t.Fatal("ValidateGameDetailTable returned nil, want validation error")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationWrongResultType {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationWrongResultType)
	}
}

func TestValidateGameDetailTableOfficialsPasses(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentGameDetail, GameDetailSubtype: "officials"}
	table := SimpleTable{
		Headers: []string{"Name", "Position"},
		Rows:    [][]string{{"Jane Smith", "Umpire"}},
	}
	if err := ValidateGameDetailTable(req, table, "### Officials: Cubs at Cardinals"); err != nil {
		t.Fatalf("ValidateGameDetailTable returned error: %v", err)
	}
}

func TestValidateGameDetailTableProbabilitiesRequiresRelevantColumns(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentGameDetail, GameDetailSubtype: "probabilities"}
	table := SimpleTable{
		Headers: []string{"Field", "Value"},
		Rows:    [][]string{{"Name", "Cubs at Cardinals"}},
	}
	err := ValidateGameDetailTable(req, table, "### Win Probability: Cubs at Cardinals")
	if err == nil {
		t.Fatal("ValidateGameDetailTable returned nil, want validation error")
	}
}

func TestValidateSimpleTableQBRLeagueAndColumns(t *testing.T) {
	table := SimpleTable{Headers: []string{"Name", "Total QBR"}, Rows: [][]string{{"Patrick Mahomes", "63.9"}}}
	if err := ValidateSimpleTable(SportsRequest{Intent: SportsIntentQBR, League: espn.LeagueNFL}, table); err != nil {
		t.Fatalf("ValidateSimpleTable returned error: %v", err)
	}
	err := ValidateSimpleTable(SportsRequest{Intent: SportsIntentQBR, League: espn.LeagueMLB}, table)
	if err == nil {
		t.Fatal("ValidateSimpleTable returned nil, want wrong-league error")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationWrongLeagueForIntent {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationWrongLeagueForIntent)
	}
}

func TestValidateSimpleTablePowerIndexRequiresShape(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentPowerIndex, League: espn.LeagueCollegeFootball}
	good := SimpleTable{Headers: []string{"Team", "FPI"}, Rows: [][]string{{"Georgia", "28.1"}}}
	if err := ValidateSimpleTable(req, good); err != nil {
		t.Fatalf("ValidateSimpleTable returned error: %v", err)
	}
	bad := SimpleTable{Headers: []string{"Team", "City"}, Rows: [][]string{{"Georgia", "Athens"}}}
	if err := ValidateSimpleTable(req, bad); err == nil {
		t.Fatal("ValidateSimpleTable returned nil, want missing-column error")
	}
}

func TestValidateSimpleTableRecruitsDraftBracketology(t *testing.T) {
	recruits := SimpleTable{Headers: []string{"Rank", "Athlete", "Position"}, Rows: [][]string{{"1", "Jane Smith", "QB"}}}
	if err := ValidateSimpleTable(SportsRequest{Intent: SportsIntentRecruits, League: espn.LeagueCollegeFootball}, recruits); err != nil {
		t.Fatalf("ValidateSimpleTable recruits returned error: %v", err)
	}
	draft := SimpleTable{Headers: []string{"Round", "Pick", "Player", "Team"}, Rows: [][]string{{"1", "1", "Alex Player", "Team"}}}
	if err := ValidateSimpleTable(SportsRequest{Intent: SportsIntentDraft, League: espn.LeagueNBA}, draft); err != nil {
		t.Fatalf("ValidateSimpleTable draft returned error: %v", err)
	}
	bracketology := SimpleTable{Headers: []string{"Seed", "Region", "Team"}, Rows: [][]string{{"1", "East", "Duke"}}}
	if err := ValidateSimpleTable(SportsRequest{Intent: SportsIntentBracketology, League: espn.LeagueMensCollegeBball}, bracketology); err != nil {
		t.Fatalf("ValidateSimpleTable bracketology returned error: %v", err)
	}
}

func TestValidateScoreboardHeaderBroadcasts(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentScoreboardHeader, GameDetailSubtype: "broadcasts"}
	good := SimpleTable{
		Headers: []string{"League", "Time/Status", "Matchup", "Broadcast"},
		Rows:    [][]string{{"MLB", "1:05 PM", "Cubs at Cardinals", "ESPN"}},
	}
	if err := ValidateScoreboardHeaderTable(req, good); err != nil {
		t.Fatalf("ValidateScoreboardHeaderTable returned error: %v", err)
	}
	bad := SimpleTable{
		Headers: []string{"League", "Time/Status", "Matchup", "Broadcast"},
		Rows:    [][]string{{"MLB", "1:05 PM", "Cubs at Cardinals", "-"}},
	}
	err := ValidateScoreboardHeaderTable(req, bad)
	if err == nil {
		t.Fatal("ValidateScoreboardHeaderTable returned nil, want missing broadcasts")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationMissingBroadcasts {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationMissingBroadcasts)
	}
}

func TestValidateSearchEntitiesAthleteScoped(t *testing.T) {
	req := SportsRequest{Intent: SportsIntentSearch, RawQuery: "search player Patrick Mahomes", AthleteQuery: "Patrick Mahomes"}
	good := []SearchEntity{{Name: "Patrick Mahomes", Type: "athlete"}}
	if err := ValidateSearchEntities(req, good); err != nil {
		t.Fatalf("ValidateSearchEntities returned error: %v", err)
	}
	bad := []SearchEntity{{Name: "Kansas City Chiefs", Type: "team"}}
	err := ValidateSearchEntities(req, bad)
	if err == nil {
		t.Fatal("ValidateSearchEntities returned nil, want wrong-result-type")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != SportsValidationWrongResultType {
		t.Fatalf("Code = %q, want %q", validationErr.Code, SportsValidationWrongResultType)
	}
}

func TestValidateSimpleTableRemainingEndpointShapes(t *testing.T) {
	cases := []struct {
		name  string
		req   SportsRequest
		table SimpleTable
	}{
		{
			name:  "venues",
			req:   SportsRequest{Intent: SportsIntentVenues, League: espn.LeagueMLB},
			table: SimpleTable{Headers: []string{"Venue", "Location"}, Rows: [][]string{{"Wrigley Field", "Chicago"}}},
		},
		{
			name:  "seasons",
			req:   SportsRequest{Intent: SportsIntentSeasons, League: espn.LeagueNFL},
			table: SimpleTable{Headers: []string{"#", "Season"}, Rows: [][]string{{"1", "2025"}}},
		},
		{
			name:  "tournaments",
			req:   SportsRequest{Intent: SportsIntentTournaments, League: espn.LeaguePGA},
			table: SimpleTable{Headers: []string{"#", "Tournament"}, Rows: [][]string{{"1", "Masters"}}},
		},
		{
			name:  "champions",
			req:   SportsRequest{Intent: SportsIntentChampions, League: espn.LeagueNBA},
			table: SimpleTable{Headers: []string{"Game", "Date", "Winner"}, Rows: [][]string{{"NBA Finals", "2025-06-20", "Thunder"}}},
		},
		{
			name:  "coaches",
			req:   SportsRequest{Intent: SportsIntentCoaches, League: espn.LeagueNFL},
			table: SimpleTable{Headers: []string{"Team", "Coach"}, Rows: [][]string{{"Chiefs", "Andy Reid"}}},
		},
		{
			name:  "fantasy",
			req:   SportsRequest{Intent: SportsIntentFantasy},
			table: SimpleTable{Headers: []string{"Status", "Detail"}, Rows: [][]string{{"Unavailable", "No public rows"}}},
		},
		{
			name:  "hot zones",
			req:   SportsRequest{Intent: SportsIntentHotZones, League: espn.LeagueMLB},
			table: SimpleTable{Headers: []string{"Split", "Value"}, Rows: [][]string{{"vs RHP", ".310"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSimpleTable(tc.req, tc.table); err != nil {
				t.Fatalf("ValidateSimpleTable returned error: %v", err)
			}
		})
	}
}

func TestValidateSimpleTableAthleteComparison(t *testing.T) {
	req := SportsRequest{
		Intent:             SportsIntentAthleteComparison,
		League:             espn.LeagueNBA,
		AthleteQuery:       "Nikola Jokic",
		SecondAthleteQuery: "Joel Embiid",
	}
	good := SimpleTable{Headers: []string{"Stat", "Nikola Jokic", "Joel Embiid"}, Rows: [][]string{{"PTS", "28", "30"}}}
	if err := ValidateSimpleTable(req, good); err != nil {
		t.Fatalf("ValidateSimpleTable returned error: %v", err)
	}
	bad := SimpleTable{Headers: []string{"Stat", "Nikola Jokic"}, Rows: [][]string{{"PTS", "28"}}}
	err := ValidateSimpleTable(req, bad)
	if err == nil {
		t.Fatal("ValidateSimpleTable returned nil, want incomplete comparison")
	}
	var validationErr *SportsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *SportsValidationError", err)
	}
	if validationErr.Code != "incomplete_comparison" {
		t.Fatalf("Code = %q, want incomplete_comparison", validationErr.Code)
	}
}
