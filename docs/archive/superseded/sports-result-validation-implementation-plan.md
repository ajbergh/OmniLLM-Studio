> **Archived — superseded implementation plan.** Its broad validation phases are implemented; retained for payload-contract rationale.

# Sports Result Validation Layer Implementation Plan

## Status

Last updated: 2026-05-17

Status: Phase 3 SimpleTable validation broadly implemented; latest verification pending.

Progress notes:

- Added `backend/internal/sports/validation.go` with structured sports validation errors, validation codes, retry hints, and `ValidateGameRows`.
- Implemented V1 schedule/scores validation for generic game rows, team-match checks, `GameDetailSubtype: pitching_matchups`, and `GameDetailSubtype: broadcasts`.
- Wired `ValidateGameRows` into `LookupScores` after ESPN scoreboard normalization, raw MLB probable-pitcher enrichment, and team filtering, but before markdown rendering.
- Extended `UserFacingError`, empty lookup titles, empty lookup status text, and graceful lookup handling so validation failures return clear user-facing no-detail messages instead of generic schedules.
- Expanded the `SportsRequest.GameDetailSubtype` doc comment to include schedule/scores subtypes such as `pitching_matchups` and `broadcasts`.
- Added `backend/internal/sports/validation_test.go` covering normal schedule validation, pitching matchup success/failure/partial data, team mismatch, broadcast missing-data validation, user-facing probable-pitcher messaging, and the exact pitching-matchup query failing generic schedule rows.
- Verified the focused validation slice with `go test ./internal/sports ./internal/router ./internal/api`.
- Verified the full backend suite with `go test ./...`.
- Added typed-row validators for standings, news, odds, roster, and leaderboard results.
- Wired those validators into `LookupStandings`, `LookupNews`, `LookupOdds`, `LookupRoster`, and the typed-row branch of `LookupLeaders` before markdown rendering.
- Added focused tests for NHL standings success, soccer standings missing-signal failure, unsupported standings league failure, news headline validation, odds betting-field validation, roster name validation, and leaderboard athlete/value validation.
- Verified the expanded sports validation tests with `go test ./internal/sports`.
- Verified the expanded typed-row validation slice with `go test ./internal/sports ./internal/router ./internal/api` and `go test ./...`.
- Added initial SimpleTable validation for `SportsIntentGameDetail` subtypes. `officials`, `probabilities`, `predictor`, `plays`, and `team_stats` now reject silent fallback to `SummaryRaw` when the user asked for a specific game-detail endpoint.
- Wired game-detail table validation into `LookupGameDetail` before `RenderSimpleMarkdown`.
- Added tests for rejecting `officials` requests that fallback to summary, accepting officials-shaped tables, and rejecting probability tables without probability-shaped columns/content.
- Verified the game-detail SimpleTable validation slice with `go test ./internal/sports`.
- Re-verified the full backend suite after SimpleTable validation with `go test ./...`.
- Added generic `ValidateSimpleTable` coverage for QBR, power index, recruits, bracketology, and draft table-shaped endpoints.
- Wired `ValidateSimpleTable` into `LookupQBR`, `LookupPowerIndex`, `LookupRecruits`, `LookupBracketology`, and `LookupDraft` before markdown rendering.
- Added `wrong_league_for_intent` validation and user-facing error mapping for sport-specific lookup/league mismatches.
- Added `ValidateScoreboardHeaderTable` and wired it into `LookupScoreboardHeader`, including the broadcast-only contract for "what games are on TV" style questions.
- Added `ValidateSearchEntities` and wired it into `LookupSearch`, including athlete-scoped search result validation.
- Added focused tests for QBR league/column validation, power-index shape validation, recruits/draft/bracketology table validation, scoreboard-header broadcast validation, and athlete-scoped search validation.
- Verified the expanded Phase 3 slice with `go test ./internal/sports`, `go test ./internal/sports ./internal/router ./internal/api`, and `go test ./...`.
- Added additional `ValidateSimpleTable` coverage for athlete comparison, hot zones, venues, seasons, tournaments, champions, coaches, fantasy, athlete awards/seasons/records, injuries, transactions, team schedule, rankings, league stats, calendar, and team history.
- Wired additional table validation into `LookupVenues`, team venue lookup, `LookupSeasons`, `LookupTournaments`, `LookupFantasy`, fantasy availability results, `LookupAthlete`, athlete fallback paths, `LookupGeneric`, `LookupAthleteComparison`, athlete-comparison fallback, `LookupHotZones`, hot-zones fallback, champion rendering, and coach lookup/fallback paths.
- Added focused tests for the remaining endpoint table shapes and athlete-comparison completeness.
- Latest verification is pending: the environment rejected the required outside-sandbox `go test` run because the escalation approval system reported a usage limit. The previous full backend suite passed before this final batch.

Outstanding items:

- Consider adding a full message-handler regression test once the sports client can be injected/mocked cleanly at that layer.
- Add optional `sports_validation` telemetry in a follow-up if trace output needs to expose validation details beyond the user-facing message.
- Continue Phase 2 by validating typed-row outputs still returned through `LookupGeneric` and adding stricter group/category checks for roster and leaders.
- Run `go test ./internal/sports`, `go test ./internal/sports ./internal/router ./internal/api`, and `go test ./...` after outside-sandbox test execution is available again.
- Review any failures from the latest endpoint-specific table validation batch and relax/tune contracts where ESPN's normalized table shapes are broader than expected.
- Resolve the hot-zones contract mismatch before enforcing wrong-league validation there: the plan describes basketball-only hot zones, while the current detector/code still supports MLB/Mookie Betts-style hot-zone lookups. Current implementation validates shape only and intentionally does not enforce a hot-zones league restriction.

Scope: all ESPN-backed leagues currently in [backend/internal/sports/detector.go](../../backend/internal/sports/detector.go) (MLB, NFL, NBA, WNBA, NHL, NCAAF, NCAAMB, NCAAWB, EPL, MLS, UCL, La Liga, Bundesliga, Serie A, Ligue 1, IPL, F1, NASCAR Cup, PGA, ATP) and every `SportsIntentType` declared in [backend/internal/sports/types.go](../../backend/internal/sports/types.go). V1 ships MLB-only validation for `pitching_matchups`; subsequent phases generalize the framework to every intent × league pair.

This document defines a deterministic `ValidateSportsResult(req, result)` layer for ESPN-backed sports lookups. The goal is to catch cases where ESPN returned valid data, but not the data needed to answer the user's specific question, before the app renders a misleading markdown response.

## Problem

The router and local sports detector can correctly identify a sports lookup while the final answer still fails the user intent.

Example:

```text
User: What are the pitching matchups for today's MLB games?
```

The app can route this to MLB schedule lookup, ESPN can return today's games, and the renderer can produce a valid schedule table. But if the output does not include probable pitchers, the answer is not actually satisfying the query.

The same pattern recurs across other sports:

- "Who's starting in goal tonight for the Bruins?" — NHL schedule returned, no starting-goalie info.
- "What are the NFL inactives for today?" — schedule returned, no inactives list.
- "What's the starting XI for Arsenal?" — match info returned, no formation.
- "Who has pole position at Monaco?" — race schedule returned, no qualifying grid.
- "What are the tee times for the third round?" — tournament page returned, no tee-time table.
- "Bracketology for next season's tournament?" — empty stub returned instead of "projections not yet published".

This is a post-tool validation problem, not primarily an intent-classification problem.

## Design Goal

Add a deterministic validation layer between ESPN data normalization and markdown rendering:

```text
SportsRequest -> ESPN lookup -> typed rows/result payload -> ValidateSportsResult -> render markdown
```

The validation layer should answer:

1. Did ESPN return data for the requested league, date, team, athlete, or event?
2. Does the returned data contain the fields required by the requested intent/subtype?
3. If required fields are missing, can the lookup retry with a richer ESPN endpoint or raw payload path?
4. If not, can the app return a precise user-facing explanation instead of a generic table?

## Non-Goals

- Do not ask the router model to judge every ESPN response.
- Do not make the main chat model review raw ESPN payloads.
- Do not validate markdown strings when typed rows are available.
- Do not block valid empty states, such as no games scheduled today, as long as the answer clearly says that.

LLM review can be added later for ambiguous natural-language cases, but V1 should be deterministic Go validation.

## Proposed API

Add a new file:

```text
backend/internal/sports/validation.go
```

Suggested public entry point:

```go
func ValidateSportsResult(req SportsRequest, result SportsResultPayload) error
```

Because the current `SportsLookupResult` stores only rendered markdown, introduce an internal typed payload for validation before rendering:

```go
type SportsResultPayload struct {
    Intent        SportsIntentType
    League        string
    LeagueName    string
    Sport         string
    DateLabel     string
    Games         []GameRow
    Standings     []StandingsRow
    News          []NewsRow
    Odds          []OddsRow
    Roster        []RosterRow
    Leaders       []LeaderboardRow
    SimpleTable   *SimpleTable
    RetrievedAt   time.Time
    Source        string
    RecoveryHints []SportsRecoveryHint
}
```

`SimpleTable` already exists at [backend/internal/sports/advanced.go:871](../../backend/internal/sports/advanced.go#L871) and is the catch-all container produced by `rawJSONTable` / `keyValueTable` for intents that do not yet have typed rows (game detail, venues, draft, fantasy, etc.). `SportsRecoveryHint` is new — it carries a deterministic retry path such as `retry_raw_scoreboard_probables`.

If introducing a broad payload is too large for the first patch, start with a narrower validator for schedule/scores:

```go
func ValidateGameRows(req SportsRequest, rows []GameRow) error
```

Then expand to additional row types incrementally.

## Error Model

Add structured validation errors so callers can distinguish "retry possible" from "answer unavailable":

```go
var (
    ErrSportsResultMismatch        = errors.New("sports result does not match request")
    ErrSportsResultMissingRequired = errors.New("sports result missing required fields")
)

type SportsValidationError struct {
    Code        string
    Message     string
    RetryHint   string
    Recoverable bool
}
```

Suggested codes:

```text
# Generic / cross-intent
missing_games
missing_rows
missing_team_match
missing_athlete_match
missing_required_columns
wrong_result_type
wrong_league_for_intent
date_mismatch

# Pregame participants (one per subtype)
missing_pitching_matchups
missing_starting_lineups
missing_starting_goalies
missing_inactives
missing_starting_xi
missing_tee_times
missing_match_schedule
missing_qualifying_grid
missing_playing_xi

# Schedule / scores adjacent
missing_broadcasts
missing_linescore

# Standings / rankings
missing_standings
wrong_poll

# Athlete / stats
missing_injury_status
missing_awards
missing_records
incomplete_comparison
wrong_stat_category

# Odds / betting
missing_odds

# Calendar / season
wrong_calendar_shape
```

## Validation Contracts

Each intent and subtype should declare required fields. Keep this explicit and readable rather than using reflection.

### Schedule and Scores

For `SportsIntentSchedule` and `SportsIntentScores`:

- Require at least one `GameRow`, unless ESPN truly returned no games for the date.
- If `TeamQuery` is set, require at least one row matching the team. (`LookupScores` already returns `ErrNoMatchingGames` in that case — validation should preserve that behavior rather than double-firing.)
- The lookup date is forwarded to ESPN via `opts.SetDate(*req.Date)` in `LookupScores`. Validation does not need to re-parse `GameRow.Date` (which is the localized `"Jan 2"` string from `formatGameDate`) to confirm the date — it should instead ensure that when `req.Date` was set, the request actually got that far. A cross-check on row dates is only useful as a sanity probe for the racing-calendar fallback path.
- If all games are pregame and intent is `scores`, rendering a schedule is acceptable only if the title and status make that clear.

### Pregame Participants (generalized)

"Probable pitchers" is the MLB instance of a broader pattern: when a user asks about *who is starting* in a scheduled but not-yet-played event, the schedule alone is not a satisfying answer. The validator must treat this as one shared shape with per-sport field requirements.

Note: although the underlying field is named `GameDetailSubtype`, the values discussed here (`pitching_matchups`, `broadcasts`, etc.) are set by `scheduleSubtype` ([backend/internal/sports/detector.go:798](../../backend/internal/sports/detector.go#L798)), not by `detectGameDetailSubtype`. They are carried on `SportsIntentSchedule` / `SportsIntentScores` requests and consumed inside `LookupScores`, not `LookupGameDetail`. The shared field name should not mislead the validator — the validation contract keys on `(Intent, GameDetailSubtype, League)` together. The field's doc comment in [backend/internal/sports/types.go:97](../../backend/internal/sports/types.go#L97) is currently incomplete and should be expanded as part of this work.

#### Per-sport participant table

| Subtype                | Sport / League             | Required source field (ESPN)            | Carrier on `GameRow` / payload     | V1?  |
| ---------------------- | -------------------------- | --------------------------------------- | ---------------------------------- | ---- |
| `pitching_matchups`    | MLB                        | `competitors[].probables[].athlete`     | `GameRow.PitchingMatchup`          | Yes  |
| `starting_lineups`     | NBA / WNBA / NCAAMB/WB     | `competitors[].leaders` or boxscore     | `GameRow.StartingLineup` (new)     | No   |
| `starting_goalies`     | NHL                        | `competitors[].probables[].athlete`     | `GameRow.StartingGoalies` (new)    | No   |
| `inactives`            | NFL / NCAAF                | `competitors[].roster` filtered out     | `GameRow.Inactives` (new)          | No   |
| `starting_xi`          | EPL / MLS / UCL / La Liga / Bundesliga / Serie A / Ligue 1 | `boxscore.form.competitors[].formation` | `GameRow.StartingXI` (new)         | No   |
| `tee_times`            | PGA                        | `event.competitions[].competitors[].startPosition` | `SimpleTable` rows                 | No   |
| `match_schedule`       | ATP                        | `event.competitions[].competitors[]`    | `SimpleTable` rows                 | No   |
| `qualifying_grid`      | F1 / NASCAR                | `event.competitions[].competitors[].startPosition` | `SimpleTable` rows                 | No   |
| `playing_xi`           | IPL (cricket)              | `boxscore.players` or `lineups`         | `SimpleTable` rows                 | No   |

V1 ships `pitching_matchups` only. The other subtypes are listed so:

1. The validator framework is built around a `(subtype → required field) -> error code` table rather than a hardcoded MLB check.
2. Detector phrases for "starting lineup", "starting goalies", "starting XI", etc. can be added without re-architecting the validator.
3. The `GameRow` struct can grow new optional string fields rather than acquiring sport-specific row types.

#### V1 required (MLB pitching matchups)

- `Intent` is `schedule` or `scores`.
- League is MLB (`espn.LeagueMLB`).
- At least one `GameRow` exists.
- The response should include `PitchingMatchup` for at least one scheduled MLB game.

Acceptable partial state:

- Some games have pitcher matchups and some do not. Render available matchups and show `TBD` or an explicit empty value for missing games.

Failure state:

- No rows have `PitchingMatchup`.

User-facing fallback message:

```text
I found today's MLB schedule, but ESPN did not provide probable pitchers for those games yet.
```

If a raw scoreboard retry has not already happened, validation should return a retry hint:

```text
retry_raw_scoreboard_probables
```

#### Generalization rule

For any pregame-participants subtype, the validator should:

1. Confirm `Intent in {schedule, scores}` and `League` is the sport-appropriate one (e.g. NHL for `starting_goalies`).
2. Confirm at least one row exists.
3. Confirm at least one row carries the participant field listed in the table.
4. If the participant field is missing for every row, emit `missing_<subtype>` (e.g. `missing_starting_goalies`) with a sport-appropriate retry hint pointing at the raw scoreboard path that includes probables/lineups.

### Broadcasts

For `GameDetailSubtype: broadcasts` on `SportsIntentSchedule` / `SportsIntentScores` / `SportsIntentScoreboardHeader`:

- Require at least one `GameRow`.
- Require at least one row with `Broadcasts`.
- If no broadcasts are present, return a clear "ESPN did not provide broadcast listings" message.

The `broadcasts` subtype is currently consumed by `LookupScoreboardHeader` ([backend/internal/sports/advanced_catalog.go:24](../../backend/internal/sports/advanced_catalog.go#L24)) for the "what games are on TV" flow, so the validator must run in that path as well as `LookupScores`.

### Game Details

For `SportsIntentGameDetail` ([backend/internal/sports/advanced_extended.go:516](../../backend/internal/sports/advanced_extended.go#L516)):

- `officials`: require an officials row or simple-table field.
- `predictor`: require predictor fields.
- `probabilities`: require win-probability fields.
- `plays`: require at least one play-by-play row.
- `team_stats`: require per-team statistic rows.
- `gamepackage` or `summary` (the default): require a specific event match and summary data.

`LookupGameDetail` already falls back to `SummaryRaw` when the requested subtype returns nothing — the validator must distinguish "intentional fallback to summary" (acceptable; title is rewritten to `### Summary: …`) from "wrong intent answered" (not acceptable). A simple heuristic: if the rendered title still reflects the requested subtype, treat it as valid; if the title was rewritten to `Summary:` but the user asked for officials/predictor/etc., return a recoverable validation error so the user message says ESPN did not provide that specific data.

Do not render a generic scoreboard table for a game-detail query unless the validation layer marks it as an intentional fallback.

### Standings

For `SportsIntentStandings`:

- Require `StandingsRow` data with at least one row.
- Require team identity (`Team` or `TeamIdentity.DisplayName`) on every row.
- Require at least one league-appropriate ranking signal on each row, per the table below. Missing all of them indicates ESPN returned a stub or a placeholder group.
- Do not render schedule rows for standings requests.

Per-sport required signal (at least one must be present per row):

| League / Sport                                | Required signal (any one)                                 |
| --------------------------------------------- | --------------------------------------------------------- |
| MLB / NFL / NBA / WNBA / NCAAMB / NCAAWB / NCAAF | `Wins` and `Losses`, or `Pct`                             |
| NHL                                           | `Points`, or `Wins`+`Losses`+`OT`/`OTL` (carried in `Ties`)|
| EPL / MLS / UCL / La Liga / Bundesliga / Serie A / Ligue 1 | `Points`, or `Wins`+`Draws`+`Losses`                      |
| IPL (cricket)                                 | `Points`, `NetRunRate`, or `Wins`+`Losses`                |
| PGA (golf) / ATP (tennis)                     | Standings not modeled — use `Rankings` or `Tournaments`   |
| F1 / NASCAR                                   | Standings not modeled — use `Rankings` or `PowerIndex`    |

If the request league is one of the "not modeled" rows above and the standings call still ran, treat it as `wrong_result_type` and route the user toward rankings / tournament leaderboards instead.

### News

For `SportsIntentNews` and `SportsIntentAthleteNews`:

- Require at least one `NewsRow`.
- Require headline text.
- If `TeamQuery` or `AthleteQuery` is present, prefer source data tied to that entity. If the ESPN endpoint only returns broad league news, label it as league news rather than pretending it is entity-specific.

### Injuries

For `SportsIntentInjuries` and `SportsIntentAthleteInjuries`:

- Require athlete/player name.
- Require some status, injury, note, or return-date field.
- Do not render a generic roster when the user asked for injuries.

### Odds

For `SportsIntentOdds`:

- Require odds rows.
- Require at least one betting field such as spread, over/under, moneyline, or provider.
- If ESPN returns games but no odds, return `ErrNoOdds` or a validation error mapped to the existing no-odds user-facing message.

### Leaders and Stats

For `SportsIntentLeaders`, `SportsIntentLeagueStats`, and `SportsIntentAthleteStats`:

- Require a stat label or category when the user asked for one.
- Require rows with values.
- If historical season data is unavailable, preserve the existing historical-data explanation.

Sport-specific required stat-category presence:

| Sport      | Required category vocabulary (StatCategory normalizes to)                                   |
| ---------- | ------------------------------------------------------------------------------------------- |
| MLB        | `batting` / `pitching` / `fielding` (with stat names like `era`, `avg`, `hr`, `rbi`, `ops`) |
| NFL / CFB  | `passing` / `rushing` / `receiving` / `defense` / `kicking`                                 |
| NBA / WNBA / NCAAMB/WB | `scoring` / `rebounds` / `assists` / `steals` / `blocks` / `fg%` / `3p%` / `ft%` |
| NHL        | `points` / `goals` / `assists` / `plus_minus` / `save%` / `gaa`                             |
| Soccer     | `goals` / `assists` / `clean_sheets` / `cards`                                              |
| Cricket    | `runs` / `wickets` / `average` / `strike_rate` / `economy`                                  |
| Golf       | `scoring_average` / `fairways_hit` / `gir` / `putts`                                        |
| Tennis     | `aces` / `service_pct` / `break_points`                                                     |
| F1 / NASCAR| `points` / `wins` / `top_5` / `top_10` / `poles`                                            |

If `req.StatCategory` is set but the response carries an unrelated category (e.g. user asked for `era` and got `home_runs`), emit `wrong_stat_category`.

### Roster

For `SportsIntentRoster`:

- Require at least one `RosterRow`.
- Require `Name` on every row; require `Position` when the sport uses positional roles (all sports above except golf/tennis individuals).
- If `Group` is set on the request (e.g. "pitchers", "goalies", "midfielders"), at least one row must match.

### Athlete Detail Intents

For `SportsIntentAthleteAwards`, `SportsIntentAthleteSeasons`, `SportsIntentAthleteRecords`:

- Require `AthleteQuery` resolved successfully (no `ErrAthleteNotFound`).
- Require at least one row in the `SimpleTable` containing the athlete's name reference *and* the season/year column.
- `athlete_awards`: require an award/title field. If the table only contains seasons-played, treat as `missing_awards`.
- `athlete_seasons`: require a `season` or year column with at least one statistical column populated.
- `athlete_records`: require a record/category column distinct from raw season stats. A response that is structurally identical to `athlete_stats` indicates ESPN returned the wrong sub-endpoint — emit `wrong_result_type`.

For `SportsIntentAthleteComparison`:

- Require `AthleteQuery` and `SecondAthleteQuery` both resolved.
- Require a side-by-side `SimpleTable` whose header row references both athletes.
- A response that contains only one athlete's stats indicates the comparison failed — emit `incomplete_comparison`.

### Football-Specific: QBR

For `SportsIntentQBR`:

- Require league is `nfl` or `college-football`. Other leagues are invalid by definition — emit `wrong_league_for_intent`.
- Require at least one row with a `QBR` value (column may be named `qbr`, `total_qbr`, or `total_qbr_season`).
- Athlete-scoped QBR additionally requires that the row's player name matches `req.AthleteQuery`.

### Basketball-Specific: Hot Zones

For `SportsIntentHotZones`:

- Require league is `nba`, `wnba`, `mens-college-basketball`, or `womens-college-basketball`.
- Require `AthleteQuery` resolved.
- Require shot-chart data with at least one zone label and one make/attempt or percentage value.
- A response containing only season averages indicates a wrong endpoint — emit `wrong_result_type`.

### Team Catalog Intents

For `SportsIntentTeams` (list-all-teams):

- Require at least one team with `Name` and (where ESPN provides it) `Location` and `Abbreviation`.
- League must be one of the supported team-sports leagues. Racing/Golf/Tennis do not have "teams" in this sense — emit `wrong_result_type`.

For `SportsIntentTeamRecord`:

- Require `TeamQuery` resolved.
- Require at least one of `Wins`, `Losses`, `Points`, or `Pct` populated for the team.

For `SportsIntentTeamSchedule`:

- Require `TeamQuery` resolved.
- Require at least one `GameRow` whose teams include the requested team.
- If the request is for a specific season, ensure the rows are from that season (label-level check is fine — `GameRow.Date` can be parsed when needed).

For `SportsIntentTeamHistory`:

- Require `TeamQuery` resolved.
- Require at least one row with a historical year/season column and at least one outcome column (record, finish, championship).
- A response that is the current-season schedule indicates the wrong endpoint — emit `wrong_result_type`.

### Transactions

For `SportsIntentTransactions`:

- Require at least one row with `Date` (or `Published`), `Team`, and a transaction description (`Note` or `Description`).
- If `TeamQuery` is set, require at least one row referencing that team.
- If `AthleteQuery` is set, require at least one row referencing that player.

### Rankings

For `SportsIntentRankings`:

- Applicable primarily to college sports (`college-football`, `mens-college-basketball`, `womens-college-basketball`) and any league exposing FPI/BPI/SP+.
- Require at least one row with `Rank` and `Team`/`Athlete` name.
- If `req.StatSort` is set (e.g. "ap", "coaches"), title or group name should reflect that poll. Mismatch → `wrong_poll`.

### Power Index

For `SportsIntentPowerIndex`:

- Require league supports FPI/BPI/SP+ (NFL, NBA, NCAAF, NCAAMB).
- Require at least one row with team and a power-index numeric value.
- A request against MLB/NHL/Soccer must emit `wrong_league_for_intent` rather than rendering blank.

### Scoreboard Header

For `SportsIntentScoreboardHeader`:

- Require at least one `GameRow` (it shares the schedule contract).
- If the user query implied broadcasts ("what games are on TV"), apply the Broadcasts contract above.

### Search

For `SportsIntentSearch`:

- Require at least one `SearchEntity` with `Name` and `Type`.
- If `req.AthleteQuery` was the search term, require at least one entity of type `player` or `athlete`. If only `team` results are returned for an athlete search, emit `wrong_result_type` and fall through to athlete lookup.

### Seasons and Calendar

For `SportsIntentSeasons`:

- Require at least one season row with a year/range column.
- Race/golf/tennis tours may return event-list calendars — that is acceptable as long as labeling makes the distinction clear.

For `SportsIntentCalendar`:

- Require at least one date or week label.
- For NFL/CFB, expect "Week N" labels; for MLB/NBA/NHL, expect date ranges; for racing/golf/tennis, expect event names. If the calendar shape does not match the league's expected format, emit `wrong_calendar_shape`.

### Champions

For `SportsIntentChampions`:

- Require at least one row with `Year`/`Season` and `Champion` (team or athlete).
- For racing/golf/tennis, the "champion" is per-event/per-major. Require at least one event with a winner row.

### Draft

For `SportsIntentDraft`:

- Applicable to leagues with drafts: NFL, NBA, NHL, MLB, WNBA, NCAA -> pro pipelines.
- Require at least one pick row with `Round`, `Pick`, `Athlete`, and `Team`.
- Soccer leagues, IPL, golf, tennis, racing do not have drafts — emit `wrong_league_for_intent`.

### Coaches

For `SportsIntentCoaches`:

- Require at least one row with `Name` and either `Team` or `Position`/`Title`.
- If `TeamQuery` is set, require at least one row matching that team.

### Venues

For `SportsIntentVenues`:

- Require at least one row with `Name` (venue / stadium / arena / track / course / court).
- Where ESPN provides it, surface capacity, city, and country — but do not require them.
- If `TeamQuery` is set, require at least one venue tied to that team.

### Recruits

For `SportsIntentRecruits`:

- Applicable only to NCAAF (and NCAAMB once detector supports it).
- Require at least one row with `Athlete`, `Position`, `Class`/`Year`, and either `Rating` or `Rank`.
- Non-college leagues must emit `wrong_league_for_intent`.

### Bracketology

For `SportsIntentBracketology`:

- Applicable to NCAAMB / NCAAWB.
- Require at least one row with seeding/projection columns (`Seed`, `Region`, `Team`).
- Pre-season requests (no projection data yet) should produce a clear "bracket projections not yet published" message rather than a stub table.

### Tournaments

For `SportsIntentTournaments`:

- Applicable primarily to golf (PGA) and tennis (ATP), plus league-managed tournaments (UCL, March Madness when scoped that way).
- Require at least one event row with `Name`, `Date` / date range, and either `Venue` or `Champion`.
- A response missing all of those indicates the wrong endpoint — emit `wrong_result_type`.

### Fantasy

For `SportsIntentFantasy`:

- Out of scope for V1 validation; `LookupFantasy` ([backend/internal/sports/advanced_catalog.go:359](../../backend/internal/sports/advanced_catalog.go#L359)) currently returns `SimpleTable` rows that are sub-shape-dependent.
- V1 should at least verify the response is non-empty and labeled as fantasy data. Fine-grained shape checks deferred.

## Payload-Shape Escape Hatches

Several intents/leagues return `*SimpleTable` rather than typed rows, including:

- All `SportsIntentGameDetail` subtypes (officials, plays, team_stats, etc.).
- Racing scoreboards (the racing fallback in [client.go:267](../../backend/internal/sports/client.go#L267)).
- `LookupTournaments`, `LookupFantasy`, `LookupVenues`, `LookupDraft`, `LookupCoaches`, `LookupChampions`, `LookupSeasons`, `LookupCalendar`, `LookupBracketology`, `LookupPowerIndex`, `LookupRecruits`, `LookupTeamHistory`, `LookupSearch`, `LookupQBR`, `LookupAthleteComparison`, `LookupHotZones`, `LookupAthlete` (awards/seasons/records subtypes).

The validator should treat `SportsResultPayload.SimpleTable` as a first-class option:

```go
if payload.SimpleTable != nil {
    // validate by required-column-name set instead of typed-field presence
    requiredColumns := requiredColumnsFor(req.Intent, req.GameDetailSubtype, req.League)
    if !tableHasColumns(payload.SimpleTable, requiredColumns) {
        return missingColumnsError(req, requiredColumns)
    }
}
```

`requiredColumnsFor` is a `(intent, subtype, league) -> []string` lookup. Column-name matching should normalize via `normalizeText` so that "Total QBR" matches both `total qbr` and `qbr`.

## Integration Points

### Short-Term Minimal Patch

Start inside `LookupScores` in:

```text
backend/internal/sports/client.go
```

Current flow (from [backend/internal/sports/client.go:229-264](../../backend/internal/sports/client.go#L229-L264)):

```go
leagueLogoURL := logoURLFromScoreboard(sb, cfg)
req.LeagueLogoURL = leagueLogoURL
rows := normalizeScoreboard(sb)
if wantsPitchingMatchups(req) && cfg.League == espn.LeagueMLB {
    if rawRows, rawErr := c.lookupPitchingMatchupRows(ctx, cfg, req); rawErr == nil && len(rawRows) > 0 {
        rows = mergePitchingMatchups(rows, rawRows)
    }
}
// ... empty-rows / team-filter handling ...
return &SportsLookupResult{
    Markdown: RenderGamesMarkdown(req, cfg, rows, retrievedAt),
    // ...
}, nil
```

Target flow:

```go
leagueLogoURL := logoURLFromScoreboard(sb, cfg)
req.LeagueLogoURL = leagueLogoURL
rows := normalizeScoreboard(sb)
// optional retries / enrichments (pitching merge, team filter, racing fallback)
if err := ValidateGameRows(req, rows); err != nil {
    return nil, err
}
return &SportsLookupResult{
    Markdown: RenderGamesMarkdown(req, cfg, rows, retrievedAt),
    // ...
}, nil
```

Note: `normalizeScoreboard` returns only `[]GameRow`; league logo/identity comes from the separate `logoURLFromScoreboard` call. The pitching-matchup raw retry already happens inside `LookupScores` before the empty-rows check — validation must run after that retry so that "we tried the raw scoreboard and still got nothing" is what triggers `missing_pitching_matchups`.

For `pitching_matchups`, validation should run after the raw `probables` merge. If `PitchingMatchup` is still empty for every row, return a specific validation error.

### Medium-Term Refactor

Move rendering responsibility out of individual lookup methods:

```text
Lookup -> typed payload -> ValidateSportsResult -> RenderSportsResult
```

This avoids repeating validation logic across `client.go`, `advanced.go`, `advanced_catalog.go`, `advanced_extended.go`, `advanced_extra.go`, and `odds.go`.

## User-Facing Error Mapping

Extend `UserFacingError(req, err)` in:

```text
backend/internal/sports/types.go
```

Map validation errors to precise messages.

Examples:

```text
missing_pitching_matchups:
I found today's MLB schedule, but ESPN did not provide probable pitchers for those games yet.

missing_broadcasts:
I found the games, but ESPN did not provide broadcast information for them.

wrong_result_type:
I found sports data from ESPN, but it did not match the type of answer requested.
```

Keep raw validation codes out of normal user-facing output.

## Router Interaction

The router should keep doing intent classification and request shaping.

The validator should run regardless of whether the request came from:

- router model — `internal/router` produces a `RouterDecision` that is mapped to a `SportsRequest` by `sports_mapper.go`
- local sports detector — `sports.DetectSportsIntent` invoked from `MaybeHandleSportsQuery` in [backend/internal/sports/client.go:802](../../backend/internal/sports/client.go#L802) and from `MessageHandler.handleSportsLookupRequest` in [backend/internal/api/message_handler.go:2085](../../backend/internal/api/message_handler.go#L2085)
- direct sports tool call — `SportsLookupTool` in [backend/internal/sports/tool.go](../../backend/internal/sports/tool.go), registered with the tool registry at [backend/internal/api/router.go:146](../../backend/internal/api/router.go#L146)
- future MCP/tool routing

Because every path above eventually invokes `ESPNClient.Lookup` ([backend/internal/sports/client.go:34](../../backend/internal/sports/client.go#L34)), the cleanest single chokepoint for validation is inside the individual `LookupXxx` methods (short-term) or just before `Lookup` returns its `*SportsLookupResult` (medium-term, after the rendering refactor).

If validation fails:

1. Try deterministic recovery if available.
2. If recovery is unavailable or fails, return a clear user-facing message.
3. Include validation outcome in router/sports telemetry when router trace is enabled.

Do not send raw ESPN response bodies to the router model in V1.

## Telemetry

Add optional internal metadata fields:

```json
{
  "sports_validation": {
    "validated": true,
    "code": "missing_pitching_matchups",
    "recoverable": false,
    "retry_hint": "",
    "required_fields": ["GameRow.PitchingMatchup"]
  }
}
```

Only include this in developer/debug metadata or trace mode. Normal assistant output should stay concise.

## Test Plan

Add tests in:

```text
backend/internal/sports/validation_test.go
```

Existing coverage in [backend/internal/sports/pitching_matchups_test.go](../../backend/internal/sports/pitching_matchups_test.go) already exercises detection (`TestDetectSportsIntentPitchingMatchups`), rendering (`TestRenderPitchingMatchupsMarkdown`), and raw-payload normalization (`TestNormalizePitchingMatchupScoreboard`), but does **not** cover post-tool validation. Validation tests are net-new.

Required tests (V1):

1. `ValidateGameRows` passes for normal schedule rows.
2. `ValidateGameRows` passes for `pitching_matchups` when at least one row has `PitchingMatchup`.
3. `ValidateGameRows` fails for `pitching_matchups` when rows exist but all `PitchingMatchup` values are empty.
4. `ValidateGameRows` allows partial pitcher data and does not require every game to have a pitcher.
5. `ValidateGameRows` fails when `TeamQuery` is set and no returned game contains that team.
6. `UserFacingError` maps missing pitching matchups to the specific probable-pitcher message.
7. Router-originated and local-detector-originated requests both pass through the validator.
8. Game-detail `officials`/`predictor`/`probabilities` queries that silently fall back to `SummaryRaw` produce a validation error rather than a generic summary table.

Required tests (broader sport coverage, V1.5+):

9. Standings: NHL response with `Points` populated passes; soccer response missing both `Points` and `Wins/Draws/Losses` fails as `missing_standings`.
10. Standings: PGA/ATP/F1/NASCAR responses against `SportsIntentStandings` emit `wrong_result_type`.
11. Leaders: MLB request for `era` returns rows whose stat columns include ERA; an MLB request for `era` that gets back rows where stat is only `home_runs` emits `wrong_stat_category`.
12. QBR: a request against MLB / NBA / NHL leagues emits `wrong_league_for_intent`.
13. Hot zones: a non-basketball league emits `wrong_league_for_intent`; missing shot data emits `wrong_result_type`.
14. Recruits: request against NFL/NBA/MLB emits `wrong_league_for_intent`.
15. Bracketology: request before projections are published returns `missing_rows` with the projection-not-published explanation, not a stub table.
16. Athlete comparison: a comparison request that only returns one athlete's stats emits `incomplete_comparison`.
17. Draft: request against EPL/MLS/IPL/PGA/ATP/F1/NASCAR emits `wrong_league_for_intent`.
18. Power index: request against MLB/NHL/soccer leagues emits `wrong_league_for_intent`.
19. Roster: NFL request with `Group="quarterbacks"` rejects a roster that contains no QB rows (`missing_rows` for the filtered subgroup).
20. SimpleTable column-name validator: column-name lookup matches across casings (`Total QBR`, `total_qbr`, `total qbr season`).

Add regression coverage for this exact query:

```text
What are the pitching matchups for today's MLB games?
```

Expected behavior:

- If ESPN returns `probables`, output includes a `Pitching Matchup` column.
- If ESPN does not return `probables`, output clearly says probable pitchers were not available.
- The app must not return a generic MLB schedule as if it answered the pitching matchup question.

## Implementation Steps

### Phase 1 — MLB-focused V1

1. Add validation error types (`SportsValidationError`, error vars, and codes) in `validation.go`.
2. Implement `ValidateGameRows` covering schedule/scores + `pitching_matchups`.
3. Wire `ValidateGameRows` into `LookupScores` after the probable-pitcher merge and before `RenderGamesMarkdown`.
4. Extend `UserFacingError` with validation-specific messages (start with `missing_pitching_matchups`, `missing_broadcasts`, `wrong_result_type`).
5. Add focused validation tests + regression test for the pitching-matchup query.

### Phase 2 — Broaden to all team-sport intents

6. Implement `ValidateStandings`, `ValidateNews`, `ValidateInjuries`, `ValidateOdds`, `ValidateLeaders` with per-sport field tables.
7. Wire each into its respective `LookupXxx` method (`LookupStandings`, `LookupNews`, `LookupGeneric` for injuries/transactions/team_schedule/rankings/league_stats/calendar, `LookupOdds`, `LookupLeaders`).
8. Add per-sport tests (NHL standings, soccer standings, NFL roster filters, etc.).

### Phase 3 — Sport-specific and SimpleTable-shaped intents

9. Implement `ValidateSimpleTable(req, table)` using a `(intent, subtype, league) -> requiredColumns` lookup table.
10. Wire `ValidateSimpleTable` into `LookupGameDetail`, `LookupQBR`, `LookupHotZones`, `LookupAthleteComparison`, `LookupTeamHistory`, `LookupChampions`, `LookupDraft`, `LookupCoaches`, `LookupVenues`, `LookupPowerIndex`, `LookupRecruits`, `LookupBracketology`, `LookupTournaments`, `LookupSeasons`, `LookupCalendar`, `LookupSearch`, `LookupScoreboardHeader`, `LookupAthlete` (awards/seasons/records subtypes).
11. Add `wrong_league_for_intent` short-circuit checks at the top of each sport-specific lookup (QBR, hot_zones, recruits, bracketology, draft, power_index, tournaments).
12. Add the per-sport tests listed in the Test Plan (items 9–20).

### Phase 4 — Pregame participants for non-MLB sports

13. Add detector phrases for `starting_lineups` (NBA/WNBA/NCAAMB/WB), `starting_goalies` (NHL), `inactives` (NFL/NCAAF), `starting_xi` (soccer leagues), `tee_times` (PGA), `match_schedule` (ATP), `qualifying_grid` (F1/NASCAR), `playing_xi` (IPL).
14. Add raw-scoreboard enrichment paths analogous to `lookupPitchingMatchupRows` for each, populating new optional `GameRow` fields or `SimpleTable` rows per the participant table.
15. Extend `ValidateGameRows` to apply the generalization rule.

### Phase 5 — Telemetry and refactor

16. Add optional `sports_validation` block to router/sports telemetry.
17. Optional: medium-term refactor moving rendering out of `LookupXxx` so a single `ValidateSportsResult` runs at the `Lookup` boundary.

## Definition of Done

### V1 (after Phase 1)

- Sports lookup results are validated before markdown rendering for MLB schedule/scores including `pitching_matchups`.
- `pitching_matchups` can no longer silently render as a generic schedule.
- Missing ESPN fields produce precise user-facing explanations for at least: pitching matchups, broadcasts, wrong-result-type.
- Existing sports lookups still pass.
- `go test ./internal/sports ./internal/router ./internal/api` passes.
- The implementation prompt/status document is updated after completion.

### V1.5 (after Phase 2)

- Validation covers all team-sport typed-row intents (schedule, scores, standings, news, injuries, odds, leaders, roster, team_record, team_schedule, transactions, rankings, league_stats, calendar).
- Per-sport standings and stats contracts pass for NFL, NBA, WNBA, NHL, MLB, all soccer leagues, and IPL.

### V2 (after Phase 3 + 4)

- Every `SportsIntentType` listed in [backend/internal/sports/types.go](../../backend/internal/sports/types.go) has a validation contract, including the sport-specific intents (QBR, hot_zones, recruits, bracketology, tournaments, power_index, fantasy, draft).
- The pregame-participants framework is generalized — adding a new sport's "who is starting" subtype does not require touching the validator core, only the lookup table and a detector phrase.
- `wrong_league_for_intent` short-circuits prevent QBR/hot_zones/recruits/draft/bracketology/power_index from running against incompatible leagues.
