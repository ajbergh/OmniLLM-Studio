package sports

import (
	"errors"
	"fmt"
	"strings"

	espn "github.com/chinmaykhachane/espn-go"
)

var (
	ErrSportsResultMismatch        = errors.New("sports result does not match request")
	ErrSportsResultMissingRequired = errors.New("sports result missing required fields")
)

const (
	SportsValidationMissingGames             = "missing_games"
	SportsValidationMissingRows              = "missing_rows"
	SportsValidationMissingTeamMatch         = "missing_team_match"
	SportsValidationMissingStandings         = "missing_standings"
	SportsValidationMissingPitchingMatchups  = "missing_pitching_matchups"
	SportsValidationMissingBroadcasts        = "missing_broadcasts"
	SportsValidationMissingOdds              = "missing_odds"
	SportsValidationMissingRequiredColumns   = "missing_required_columns"
	SportsValidationWrongStatCategory        = "wrong_stat_category"
	SportsValidationWrongResultType          = "wrong_result_type"
	SportsValidationWrongLeagueForIntent     = "wrong_league_for_intent"
	SportsRecoveryRetryRawScoreboardProbable = "retry_raw_scoreboard_probables"
)

type SportsValidationError struct {
	Code        string
	Message     string
	RetryHint   string
	Recoverable bool
	Err         error
}

func (e *SportsValidationError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.Code) != "" {
		return e.Code
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return ErrSportsResultMismatch.Error()
}

func (e *SportsValidationError) Unwrap() error {
	if e == nil || e.Err == nil {
		return ErrSportsResultMismatch
	}
	return e.Err
}

func (e *SportsValidationError) Is(target error) bool {
	if target == ErrSportsResultMismatch {
		return true
	}
	if target == ErrSportsResultMissingRequired {
		return errors.Is(e.Err, ErrSportsResultMissingRequired)
	}
	return errors.Is(e.Err, target)
}

func ValidateGameRows(req SportsRequest, rows []GameRow) error {
	if req.Intent != SportsIntentSchedule && req.Intent != SportsIntentScores && req.Intent != SportsIntentScoreboardHeader {
		return nil
	}
	if len(rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingGames,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	if strings.TrimSpace(req.TeamQuery) != "" && len(filterGameRowsByTeam(rows, req.TeamQuery)) == 0 {
		return &SportsValidationError{
			Code:    SportsValidationMissingTeamMatch,
			Message: fmt.Sprintf("ESPN returned games, but none matched %q.", req.TeamQuery),
			Err:     ErrSportsResultMismatch,
		}
	}
	if err := validatePregameParticipantRows(req, rows); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(req.GameDetailSubtype), "broadcasts") && !anyGameRowHas(rows, func(row GameRow) bool {
		return strings.TrimSpace(row.Broadcasts) != ""
	}) {
		return &SportsValidationError{
			Code: SportsValidationMissingBroadcasts,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	return nil
}

func ValidateStandingsRows(req SportsRequest, rows []StandingsRow) error {
	if req.Intent != SportsIntentStandings {
		return nil
	}
	if standingsUnsupportedForLeague(req.League) {
		return &SportsValidationError{
			Code: SportsValidationWrongResultType,
			Err:  ErrSportsResultMismatch,
		}
	}
	if len(rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingStandings,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	for _, row := range rows {
		if strings.TrimSpace(firstNonEmpty(row.Team, row.TeamIdentity.DisplayName)) == "" || !standingsRowHasSignal(req.League, row) {
			return &SportsValidationError{
				Code: SportsValidationMissingStandings,
				Err:  ErrSportsResultMissingRequired,
			}
		}
	}
	return nil
}

func ValidateNewsRows(req SportsRequest, rows []NewsRow) error {
	if req.Intent != SportsIntentNews && req.Intent != SportsIntentAthleteNews {
		return nil
	}
	if len(rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingRows,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	for _, row := range rows {
		if strings.TrimSpace(row.Headline) != "" {
			return nil
		}
	}
	return &SportsValidationError{
		Code: SportsValidationMissingRequiredColumns,
		Err:  ErrSportsResultMissingRequired,
	}
}

func ValidateOddsRows(req SportsRequest, rows []OddsRow) error {
	if req.Intent != SportsIntentOdds {
		return nil
	}
	if len(rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingOdds,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	for _, row := range rows {
		if strings.TrimSpace(firstNonEmpty(row.AwayMoneyLine, row.HomeMoneyLine, row.Spread, row.OverUnder, row.Provider)) != "" {
			return nil
		}
	}
	return &SportsValidationError{
		Code: SportsValidationMissingOdds,
		Err:  ErrSportsResultMissingRequired,
	}
}

func ValidateRosterRows(req SportsRequest, rows []RosterRow) error {
	if req.Intent != SportsIntentRoster {
		return nil
	}
	if len(rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingRows,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" {
			return &SportsValidationError{
				Code: SportsValidationMissingRequiredColumns,
				Err:  ErrSportsResultMissingRequired,
			}
		}
	}
	return nil
}

func ValidateLeaderboardRows(req SportsRequest, rows []LeaderboardRow) error {
	if req.Intent != SportsIntentLeaders && req.Intent != SportsIntentLeagueStats {
		return nil
	}
	if len(rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingRows,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	for _, row := range rows {
		if strings.TrimSpace(row.Athlete) != "" && strings.TrimSpace(row.Value) != "" {
			return nil
		}
	}
	return &SportsValidationError{
		Code: SportsValidationMissingRequiredColumns,
		Err:  ErrSportsResultMissingRequired,
	}
}

func ValidateGameDetailTable(req SportsRequest, table SimpleTable, title string) error {
	if req.Intent != SportsIntentGameDetail {
		return nil
	}
	if len(table.Rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingRows,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	subtype := strings.ToLower(strings.TrimSpace(req.GameDetailSubtype))
	if subtype == "" || subtype == "summary" || subtype == "gamepackage" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(title)), "### summary:") {
		return &SportsValidationError{
			Code: SportsValidationWrongResultType,
			Err:  ErrSportsResultMismatch,
		}
	}
	switch subtype {
	case "officials":
		if tableHasAnyColumn(table, "official", "officials", "name", "display name") || tableContainsAny(table, "official") {
			return nil
		}
	case "probabilities":
		if tableHasAnyColumn(table, "probability", "probabilities", "home win percentage", "away win percentage", "tie percentage", "win probability") ||
			tableContainsAny(table, "probability", "probabilities", "win percentage") {
			return nil
		}
	case "predictor":
		if tableHasAnyColumn(table, "predictor", "home team", "away team", "game projection", "projected winner") ||
			tableContainsAny(table, "predictor", "projection", "projected") {
			return nil
		}
	case "plays":
		if tableHasAnyColumn(table, "text", "play", "type", "clock", "period") {
			return nil
		}
	case "team_stats":
		if tableHasAnyColumn(table, "stat", "statistics", "team", "name", "display value") ||
			tableContainsAny(table, "statistics", "team stats") {
			return nil
		}
	default:
		return nil
	}
	return &SportsValidationError{
		Code: SportsValidationMissingRequiredColumns,
		Err:  ErrSportsResultMissingRequired,
	}
}

func ValidateSimpleTable(req SportsRequest, table SimpleTable) error {
	if len(table.Rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingRows,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	switch req.Intent {
	case SportsIntentQBR:
		if !leagueCompatible(req.League, espn.LeagueNFL, espn.LeagueCollegeFootball) {
			return wrongLeagueError()
		}
		if !tableHasAnyColumn(table, "qbr", "total qbr", "total_qbr", "sched adj qbr", "schedadjqbr") {
			return missingColumnsError()
		}
	case SportsIntentPowerIndex:
		if !leagueCompatible(req.League, espn.LeagueNFL, espn.LeagueNBA, espn.LeagueCollegeFootball, espn.LeagueMensCollegeBball) {
			return wrongLeagueError()
		}
		if !tableHasAnyColumn(table, "fpi", "bpi", "sp+", "power index", "rating", "rank") {
			return missingColumnsError()
		}
	case SportsIntentRecruits:
		if !leagueCompatible(req.League, espn.LeagueCollegeFootball) {
			return wrongLeagueError()
		}
		if !tableHasAnyColumn(table, "athlete", "player", "name", "position", "rank", "rating") {
			return missingColumnsError()
		}
	case SportsIntentBracketology:
		if !leagueCompatible(req.League, "", espn.LeagueMensCollegeBball, espn.LeagueWomensCollegeBall) {
			return wrongLeagueError()
		}
		if !tableHasAnyColumn(table, "seed", "region", "team", "projection", "bracket") &&
			!tableContainsAny(table, "seed", "region", "bracket", "projection") {
			return missingColumnsError()
		}
	case SportsIntentDraft:
		if !leagueCompatible(req.League, espn.LeagueNFL, espn.LeagueNBA, espn.LeagueNHL, espn.LeagueMLB, espn.LeagueWNBA) {
			return wrongLeagueError()
		}
		if !tableHasAnyColumn(table, "round", "pick", "selection", "athlete", "player", "name", "team") {
			return missingColumnsError()
		}
	case SportsIntentAthleteComparison:
		if strings.TrimSpace(req.AthleteQuery) != "" && strings.TrimSpace(req.SecondAthleteQuery) != "" {
			if !tableHasAnyColumn(table, req.AthleteQuery) || !tableHasAnyColumn(table, req.SecondAthleteQuery) {
				return &SportsValidationError{Code: "incomplete_comparison", Err: ErrSportsResultMismatch}
			}
		}
	case SportsIntentHotZones:
		if !tableHasAnyColumn(table, "zone", "area", "split", "stat", "name", "display value", "value") &&
			!tableContainsAny(table, "zone", "hot", "split", "shot") {
			return missingColumnsError()
		}
	case SportsIntentVenues:
		if !tableHasAnyColumn(table, "venue", "stadium", "arena", "location", "capacity", "surface") {
			return missingColumnsError()
		}
	case SportsIntentSeasons:
		if !tableHasAnyColumn(table, "season", "year") {
			return missingColumnsError()
		}
	case SportsIntentTournaments:
		if !leagueCompatible(req.League, espn.LeaguePGA, espn.LeagueATP, espn.LeagueChampionsLg, espn.LeagueMensCollegeBball, espn.LeagueWomensCollegeBall) {
			return wrongLeagueError()
		}
		if !tableHasAnyColumn(table, "tournament", "event", "name", "date", "venue", "champion") {
			return missingColumnsError()
		}
	case SportsIntentChampions:
		if !tableHasAnyColumn(table, "game", "date", "winner", "champion", "score", "loser", "season", "year") {
			return missingColumnsError()
		}
	case SportsIntentCoaches:
		if !tableHasAnyColumn(table, "coach", "name", "team", "position", "title", "ref url") {
			return missingColumnsError()
		}
	case SportsIntentFantasy:
		if !tableHasAnyColumn(table, "status", "detail", "team", "player", "league", "standing", "settings") {
			return missingColumnsError()
		}
	case SportsIntentAthleteAwards:
		if !tableHasAnyColumn(table, "award", "title", "honor", "season", "year", "name") &&
			!tableContainsAny(table, "award", "honor") {
			return missingColumnsError()
		}
	case SportsIntentAthleteSeasons:
		if !tableHasAnyColumn(table, "season", "year") {
			return missingColumnsError()
		}
	case SportsIntentAthleteRecords:
		if !tableHasAnyColumn(table, "record", "category", "stat", "value", "name") {
			return missingColumnsError()
		}
	case SportsIntentInjuries, SportsIntentAthleteInjuries:
		if !tableHasAnyColumn(table, "player", "athlete", "name", "status", "injury", "detail", "note") {
			return missingColumnsError()
		}
	case SportsIntentTransactions:
		if !tableHasAnyColumn(table, "date", "team", "transaction", "description", "note", "player", "athlete") {
			return missingColumnsError()
		}
	case SportsIntentTeamSchedule, SportsIntentRankings, SportsIntentLeagueStats, SportsIntentCalendar, SportsIntentTeamHistory:
		if len(table.Headers) == 0 {
			return missingColumnsError()
		}
	}
	return nil
}

func ValidateScoreboardHeaderTable(req SportsRequest, table SimpleTable) error {
	if req.Intent != SportsIntentScoreboardHeader {
		return nil
	}
	if len(table.Rows) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingRows,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	broadcastsOnly := strings.EqualFold(strings.TrimSpace(req.GameDetailSubtype), "broadcasts") ||
		hasAnyPhrase(normalizeText(req.RawQuery), "televised", "broadcast", "broadcasting", "national tv", "nationally televised")
	if broadcastsOnly && !tableHasAnyValueInColumn(table, "broadcast") {
		return &SportsValidationError{
			Code: SportsValidationMissingBroadcasts,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	if !tableHasAnyColumn(table, "league", "time", "status", "matchup", "score", "broadcast", "venue") {
		return missingColumnsError()
	}
	return nil
}

func ValidateSearchEntities(req SportsRequest, entities []SearchEntity) error {
	if req.Intent != SportsIntentSearch {
		return nil
	}
	if len(entities) == 0 {
		return &SportsValidationError{
			Code: SportsValidationMissingRows,
			Err:  ErrSportsResultMissingRequired,
		}
	}
	athleteScoped := strings.TrimSpace(req.AthleteQuery) != "" && hasAnyPhrase(normalizeText(req.RawQuery), "player", "athlete")
	for _, entity := range entities {
		if strings.TrimSpace(entity.Name) == "" || strings.TrimSpace(entity.Type) == "" {
			return &SportsValidationError{
				Code: SportsValidationMissingRequiredColumns,
				Err:  ErrSportsResultMissingRequired,
			}
		}
		if athleteScoped && isAthleteSearchEntity(entity) {
			return nil
		}
	}
	if athleteScoped {
		return &SportsValidationError{
			Code: SportsValidationWrongResultType,
			Err:  ErrSportsResultMismatch,
		}
	}
	return nil
}

func validatePregameParticipantRows(req SportsRequest, rows []GameRow) error {
	subtype := strings.TrimSpace(req.GameDetailSubtype)
	if subtype == "" {
		return nil
	}
	contract, ok := pregameParticipantContract(subtype)
	if !ok {
		return nil
	}
	if contract.League != "" && !strings.EqualFold(strings.TrimSpace(req.League), contract.League) {
		return &SportsValidationError{
			Code: SportsValidationWrongResultType,
			Err:  ErrSportsResultMismatch,
		}
	}
	if contract.HasRequiredField != nil && contract.HasRequiredField(rows) {
		return nil
	}
	return &SportsValidationError{
		Code:        contract.MissingCode,
		RetryHint:   contract.RetryHint,
		Recoverable: contract.RetryHint != "",
		Err:         ErrSportsResultMissingRequired,
	}
}

type pregameParticipantValidationContract struct {
	League           string
	MissingCode      string
	RetryHint        string
	HasRequiredField func([]GameRow) bool
}

func pregameParticipantContract(subtype string) (pregameParticipantValidationContract, bool) {
	switch strings.ToLower(strings.TrimSpace(subtype)) {
	case "pitching_matchups", "probable_pitchers":
		return pregameParticipantValidationContract{
			League:      espn.LeagueMLB,
			MissingCode: SportsValidationMissingPitchingMatchups,
			RetryHint:   SportsRecoveryRetryRawScoreboardProbable,
			HasRequiredField: func(rows []GameRow) bool {
				return anyGameRowHas(rows, func(row GameRow) bool {
					return strings.TrimSpace(row.PitchingMatchup) != ""
				})
			},
		}, true
	default:
		return pregameParticipantValidationContract{}, false
	}
}

func anyGameRowHas(rows []GameRow, predicate func(GameRow) bool) bool {
	for _, row := range rows {
		if predicate(row) {
			return true
		}
	}
	return false
}

func standingsUnsupportedForLeague(league string) bool {
	switch strings.ToLower(strings.TrimSpace(league)) {
	case "pga", "atp", "f1", "nascar", "nascar-cup":
		return true
	default:
		return false
	}
}

func standingsRowHasSignal(league string, row StandingsRow) bool {
	hasWinsLosses := strings.TrimSpace(row.Wins) != "" && strings.TrimSpace(row.Losses) != ""
	hasSoccerRecord := strings.TrimSpace(row.Wins) != "" && strings.TrimSpace(row.Draws) != "" && strings.TrimSpace(row.Losses) != ""
	switch strings.ToLower(strings.TrimSpace(league)) {
	case "nhl":
		return strings.TrimSpace(row.Points) != "" || (hasWinsLosses && strings.TrimSpace(row.Ties) != "")
	case "eng.1", "usa.1", "uefa.champions", "esp.1", "ger.1", "ita.1", "fra.1":
		return strings.TrimSpace(row.Points) != "" || hasSoccerRecord
	case LeagueIPL:
		return strings.TrimSpace(row.Points) != "" || strings.TrimSpace(row.NetRunRate) != "" || hasWinsLosses
	default:
		return hasWinsLosses || strings.TrimSpace(row.Pct) != "" || strings.TrimSpace(row.Points) != ""
	}
}

func tableHasAnyColumn(table SimpleTable, names ...string) bool {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[normalizeText(name)] = struct{}{}
	}
	for _, header := range table.Headers {
		norm := normalizeText(header)
		if _, ok := wanted[norm]; ok {
			return true
		}
		for wantedName := range wanted {
			if wantedName != "" && strings.Contains(norm, wantedName) {
				return true
			}
		}
	}
	return false
}

func tableContainsAny(table SimpleTable, phrases ...string) bool {
	for _, row := range table.Rows {
		for _, cell := range row {
			norm := normalizeText(cell)
			for _, phrase := range phrases {
				if strings.Contains(norm, normalizeText(phrase)) {
					return true
				}
			}
		}
	}
	return false
}

func tableHasAnyValueInColumn(table SimpleTable, columnPhrase string) bool {
	columnPhrase = normalizeText(columnPhrase)
	columnIndexes := make([]int, 0, len(table.Headers))
	for i, header := range table.Headers {
		if strings.Contains(normalizeText(header), columnPhrase) {
			columnIndexes = append(columnIndexes, i)
		}
	}
	if len(columnIndexes) == 0 {
		return false
	}
	for _, row := range table.Rows {
		for _, idx := range columnIndexes {
			if idx < len(row) {
				value := strings.TrimSpace(row[idx])
				if value != "" && value != "-" && !strings.EqualFold(value, "tbd") {
					return true
				}
			}
		}
	}
	return false
}

func leagueCompatible(league string, allowed ...string) bool {
	league = strings.ToLower(strings.TrimSpace(league))
	if league == "" {
		for _, value := range allowed {
			if strings.TrimSpace(value) == "" {
				return true
			}
		}
		return true
	}
	for _, value := range allowed {
		if strings.EqualFold(league, strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func wrongLeagueError() error {
	return &SportsValidationError{
		Code: SportsValidationWrongLeagueForIntent,
		Err:  ErrSportsResultMismatch,
	}
}

func missingColumnsError() error {
	return &SportsValidationError{
		Code: SportsValidationMissingRequiredColumns,
		Err:  ErrSportsResultMissingRequired,
	}
}

func sportsValidationCode(err error) string {
	var validationErr *SportsValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code
	}
	return ""
}
