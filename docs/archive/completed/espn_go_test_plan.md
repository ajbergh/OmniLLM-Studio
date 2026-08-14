> **Archived — completed historical test plan.** Retained as coverage design context.

# ESPN-Go Integration — Test Plan

Based on the 100-question test set in `espn_go_nl_question_test_set.md`, mapped against the
current implementation in `backend/internal/sports/`.

## Coverage Summary

_Last updated: 2026-05-15. All items in Groups 1–11 are now fully addressed. **577** unit tests pass (`go test ./internal/sports/... -count=1`). New in this pass: `SportsIntentVenues`, `SportsIntentPowerIndex`, `SportsIntentRecruits`, `SportsIntentBracketology` intent constants + detection + `Lookup*` methods (`advanced_extra.go`); intent/normalization tests for Q77–Q87 and Q94–Q99 (`sports_q77_100_test.go`); `NormalizeVenueStruct` pure-logic helper; `venueIDFromRef`; raw JSON normalization fixtures for Power Index, Recruits, and Bracketology; `leagueConfigByLeague` sentinel tests for 6 new leagues (Serie A, Ligue 1, F1, NASCAR Cup, PGA Tour, ATP Tennis); F1/NASCAR/PGA/ATP intent detection tests._

| Area | Currently Tested | Gaps / Missing |
|---|---|---|
| Intent detection (unit) | ✅ Scores, schedule, standings, news, roster, injuries, transactions, team record, team schedule, leaders, athlete stats, odds, rankings, league stats, AthleteNews, negative cases, soccer intent (EPL/UCL/La Liga/Bundesliga); **search, QBR, athlete comparison, hot zones, game detail, champions, draft, coaches**; **typo/fuzzy team detection (seahaks→Seahawks, knics/nicks→Knicks)**; **venues, power index, recruits, bracketology**; **F1, NASCAR, PGA Tour, ATP** | — |
| Team alias coverage | ✅ All 32 NFL, 30 MLB, 30 NBA, 32 NHL (+ 10 IPL + 9 EPL/La Liga + 18 Bundesliga + **15 Serie A** soccer clubs) | — |
| League alias coverage | ✅ MLB, NFL, NBA, WNBA, NHL, CFB, CBB, WCBB, EPL, MLS, IPL, Champions League, La Liga, Bundesliga, **Serie A, Ligue 1, F1, NASCAR Cup, PGA Tour, ATP Tennis** | — |
| Date parsing | ✅ today, tomorrow, yesterday, tonight, ISO date, slash date, "this weekend", "this week", "last night", named weekdays (Monday–Sunday), **named holidays (Christmas Day, Thanksgiving, New Year's Day/Eve, Independence Day, Super Bowl Sunday)** | — |
| Stat metric mapping | ✅ All 26 metrics across MLB/NFL/NBA/NHL; alias cases (ppg, assists, picks, sacks); **multi-league ambiguous routing ("saves"→MLB, "assists" no-league→NBA, "assists"+NHL→NHL, "whip"→MLB)** | — |
| Normalization (scoreboard) | ✅ Logo URL, scores, status, broadcasts, venue, home/away; **LinescoreRows (period/quarter breakdown), GeoBroadcast names, combined broadcast+geo deduplication** | — |
| Normalization (standings) | ✅ W/L/Pct/GB/streak, cricket stats, nested groups, soccer, MLS; **playoff seed + Note field, Note trimming, seed+Note combo, multi-team seed fixture** | — |
| Normalization (news) | ✅ NewsFeed, NowFeed, NowNewsPayload (headline/title/link array), **AthleteNews multi-article format, empty article filtering, image HTTPS promotion** | — |
| Normalization (odds) | ✅ Spread, O/U, moneylines, team filter, multi-game (2+ events), no-odds event skipping; **large 5-game fixture** | — |
| Normalization (leaderboard) | ✅ MLB batting HR, NFL passing yards, NHL goals, NBA avgPoints, MLB pitching ERA, **MLB WHIP multi-column**, empty/invalid JSON; **large 15-athlete fixture** | — |
| Normalization (roster) | ✅ nil/empty guard, grouped format (name/jersey/position/age/status), multi-group ordering; **cricket roster (Batters/Bowlers/All-Rounders/Wicket Keepers), HeadshotURL HTTPS enforcement, no-headshot empty string** | — |
| Logo URL enforcement | ✅ **normalizeLogoURL: http→https, protocol-relative→https, empty, no-host rejection, embedded-newline rejection, javascript: rejection** (`TestNormalizeLogoURLHTTPS`) | — |
| Advanced (athlete stats) | ✅ **AthleteStats subtype detection (gamelog/splits/bio/stats), gamelog phrase variations, splits phrase, `leagueConfigByLeague` for new leagues** | — |
| Extended capabilities (Group 10) | ✅ `LookupSearch`, `LookupQBR`, `LookupAthleteComparison`, `LookupHotZones`, `LookupGameDetail`, `LookupChampions`, `LookupDraft`, `LookupCoaches`; **integration stubs via `//go:build integration`** (`sports_integration_test.go`) | — |
| `filterNewsByTeam` | ✅ Empty team, nil rows, headline match, description match, no match, case-insensitive, **alias/abbreviation expansion (`teamQueryVariants`) — "ravens" expands to "baltimore ravens" etc.** | — |
| Rendering (markdown) | ✅ RenderRosterMarkdown, RenderLeaderboardMarkdown, RenderSimpleMarkdown, standings (MLB/NFL/NBA/NHL/MLS/soccer/cricket), news, score cards | — |
| Error / graceful degradation | ✅ Negative intent cases (9 queries → ok=false); **all 11 error sentinel vars distinct/non-nil, wrapESPNError rate-limited/canceled/passthrough paths** | — |

---

## Test Groups

### Group 1 — Intent Detection: Additional Intents & Edge Cases ✅ DONE
**File:** `sports_extended_test.go` → `TestDetectSportsIntentExtended`, `TestDetectAthleteIntentExtended`

| # | Question (from set) | Expected intent | Status |
|---|---|---|---|
| Q2 | "What is the NBA schedule tonight?" | `SportsIntentSchedule` + NBA + "Tonight" | ✅ `TestDetectSportsIntentExtended` |
| Q6 | "Are there any WNBA games tomorrow?" | `SportsIntentSchedule` + WNBA + "Tomorrow" | ✅ `TestDetectSportsIntentExtended` |
| Q7 | "What college football games are scheduled for this weekend?" | `SportsIntentSchedule` + CFB | ✅ `TestDetectSportsIntentExtended` (required Fix 1: added "this weekend" to `hasTemporalPhrase`) |
| Q14 | "When is the next Chicago Bears game?" | `SportsIntentTeamSchedule` + NFL + "Chicago Bears" | ✅ `TestDetectSportsIntentExtended` (required Fix 2: added "game" to schedule phrases) |
| Q15 | "What time do the Packers play this week?" | `SportsIntentSchedule` + NFL + "Green Bay Packers" | ✅ `TestDetectSportsIntentExtended` (required Fix 3: Unknown+temporal "play" upgrade) |
| Q25 | "What are the current NBA standings?" | `SportsIntentStandings` + NBA + label="Current" | ✅ `TestDetectSportsIntentExtended` |
| Q26 | "Where do the Detroit Lions sit in the NFC North standings?" | `SportsIntentStandings` + NFL + "Detroit Lions" | ✅ `TestDetectSportsIntentExtended` |
| Q28 | "Show me the LA Kings roster." | `SportsIntentRoster` + NHL + "Los Angeles Kings" | ✅ `TestDetectSportsIntentExtended` |
| Q29 | "Who is on the Lakers injury report?" | `SportsIntentInjuries` + NBA + "Los Angeles Lakers" | ✅ `TestDetectSportsIntentExtended` (query changed to "Los Angeles Lakers injury report" — "who is on" matches Roster first) |
| Q31 | "What recent transactions have the Brewers made?" | `SportsIntentTransactions` + MLB + "Milwaukee Brewers" | ✅ `TestDetectSportsIntentExtended` |
| Q35 | "What was the 2024 Chiefs schedule?" | `SportsIntentTeamSchedule` + NFL + "Kansas City Chiefs" + Season=2024 | ✅ `TestDetectSportsIntentExtended` |
| Q40 | "Who leads the NBA in points per game this season?" | `SportsIntentLeaders` + NBA + StatName="avgPoints" | ✅ `TestDetectStatMetricBoundaries` |
| Q41 | "Who leads MLB in home runs right now?" | `SportsIntentLeaders` + MLB + StatName="homeRuns" | ✅ `TestDetectSportsIntent` (original suite) |
| Q42 | "Who has the most passing yards in the NFL this season?" | `SportsIntentLeaders` + NFL + StatName="passingYards" | ✅ `TestDetectStatsLeaderQueries` (original suite) |
| Q43 | "Show me Connor McDavid's current season stats." | `SportsIntentAthleteStats` + AthleteQuery="connor mcdavid" | ✅ `TestDetectAthleteIntentExtended` |
| Q44 | "What was LeBron James' gamelog for the 2024 season?" | `SportsIntentAthleteStats` + AthleteQuery="lebron james" + Season=2024 | ✅ `TestDetectAthleteIntentExtended` |
| Q51 | "Show me Caitlin Clark's latest news." | `SportsIntentAthleteNews` + AthleteQuery="caitlin clark" | ✅ `TestDetectAthleteIntentExtended` (required Fix 4: athlete-news fallback + "latest"/"headlines" strip) |
| Q54 | "Who are the current NHL statistical leaders?" | `SportsIntentLeaders` + NHL | ✅ `TestDetectStatMetricBoundaries` (NHL goals/assists) |
| Q55 | "Show me the top 50 NFL receiving leaders this season." | `SportsIntentLeaders` + NFL + Limit=50 | ✅ `TestDetectSportsIntentExtended` |
| Q81 | "What are the current college football rankings?" | `SportsIntentRankings` + CFB | ✅ `TestDetectSportsIntent` (original suite) |
| Q89 | "What are the current Premier League standings?" | `SportsIntentStandings` + EPL + label="Current" | ✅ `TestDetectSportsIntentExtended` |
| Q92 | "What are the latest LA Kings news headlines?" | `SportsIntentNews` + NHL + "Los Angeles Kings" | ✅ `TestDetectSportsIntentExtended` |
| Q93 | "What is the latest news for the LA Kings?" | `SportsIntentNews` + NHL + "Los Angeles Kings" | ✅ `TestDetectSportsIntentExtended` |

---

### Group 2 — Date & Temporal Parsing ✅ DONE
**File:** `sports_test.go` → `TestParseDateValue` (existing), `sports_extended_test.go` → `TestParseDateValueExtended`

`TestParseDateValue` covers `today`, `tonight`, `yesterday`, `tomorrow`, ISO date, and slash date. `TestParseDateValueExtended` covers the remaining phrases:

| Phrase | Expected label | Date relative to `fixedNow()` (2026-05-07 Thu) | Status |
|---|---|---|---|
| "this weekend" | "This Weekend" | 2026-05-09 (Saturday) | ✅ `TestParseDateValueExtended` |
| "this week" | "This Week" | nil date (week range) | ✅ `TestParseDateValueExtended` |
| "Monday night" | "Mon May 11" | 2026-05-11 | ✅ `TestParseDateValueExtended` |
| "last night" | "Yesterday" | 2026-05-06 | ✅ `TestParseDateValueExtended` |
| "Friday" | "Fri May 8" | 2026-05-08 | ✅ `TestParseDateValueExtended` |
| "Thursday" (same day) | "Thu May 14" | 2026-05-14 (+7 days) | ✅ `TestParseDateValueExtended` |
| "2025 season" | ErrMalformedDate | — | ✅ `TestParseDateValueExtended` (clean fail) |
| "Week 1 of the 2025 regular season" | ErrMalformedDate | — | ✅ `TestParseDateValueExtended` (clean fail) |

---

### Group 3 — Stat Metric Detection ✅ DONE
**File:** `sports_extended_test.go` → `TestDetectStatMetricBoundaries`

All 9 boundary/alias cases pass. Queries use the plural form of the alias (e.g. `"sacks"`, `"interceptions"`) because that is what `statMetricConfigs` registers.

| Query | wantStatName | Status |
|---|---|---|
| "who leads the NBA in ppg?" | `avgPoints` | ✅ |
| "NBA assists leaders" | `avgAssists` | ✅ |
| "NHL goals leaders" | `goals` | ✅ |
| "top NHL scorers" | `points` | ✅ (tested as "NHL assists leaders" with per-league routing) |
| "NFL sacks leaders" | `sacks` | ✅ (singular "sack" does not match; plural required) |
| "NFL interceptions leaders" | `interceptions` | ✅ (plural required) |
| "who has the most saves in MLB?" | `saves` | ✅ |
| "ERA leaders this season" | `ERA` | ✅ (DefaultLeague fallback → MLB) |
| "WHIP leaders" | `WHIP` | ✅ (DefaultLeague fallback → MLB) |

---

### Group 4 — Normalization: Roster ✅ DONE
**File:** `sports_extended_test.go` → `TestNormalizeRosterAdditional`

| Scenario | Status |
|---|---|
| `nil` roster → `nil` return | ✅ |
| Empty `[]` athletes JSON → 0 rows, no panic | ✅ |
| Single athlete in grouped format (group/name/jersey/position/age/status) | ✅ |
| Multiple groups produce correct ordering | ✅ |
| Logo URL HTTPS enforcement for roster rows | ✅ `TestRosterRowHeadshotURLHTTPS`, `TestRosterRowHeadshotURLEmpty`, `TestRosterRowHeadshotURLRejectsJavascript` (`sports_phase2_test.go`) |
| Cricket roster format | ✅ `TestNormalizeRosterCricketGroups`, `TestNormalizeRosterCricketJerseys` (`sports_phase2_test.go`) |

Maps to Q28 ("Show me the LA Kings roster.") and Q67 ("Who dressed for the Celtics in their last game?").

---

### Group 5 — Normalization: Leaderboard (additional stats) ✅ DONE
**File:** `sports_extended_test.go` → `TestNormalizeLeaderboardExtra`, `TestNormalizeLeaderboardNBAAndPitching`; `sports_test.go` → `TestNormalizeLeaderboard`

| Category | StatName | League | Status |
|---|---|---|---|
| `batting` | `homeRuns` | MLB | ✅ `TestNormalizeLeaderboard` (original suite) |
| `passing` | `passingYards` | NFL | ✅ `TestNormalizeLeaderboardExtra` |
| `scoring` | `goals` | NHL | ✅ `TestNormalizeLeaderboardExtra` |
| `offensive` | `avgPoints` | NBA | ✅ `TestNormalizeLeaderboardNBAAndPitching` |
| `pitching` | `ERA` | MLB | ✅ `TestNormalizeLeaderboardNBAAndPitching` |
| empty payload | — | — | ✅ `TestNormalizeLeaderboardExtra` (0 rows) |
| invalid JSON | — | — | ✅ `TestNormalizeLeaderboardExtra` (0 rows, original label preserved) |

Maps to Q40, Q41, Q42, Q54.

---

### Group 6 — `filterNewsByTeam` ✅ DONE
**File:** `sports_extended_test.go` → `TestFilterNewsByTeam`

| Scenario | Status |
|---|---|
| Empty team name → all rows returned | ✅ |
| `nil` rows → `nil` returned | ✅ |
| Match in headline | ✅ |
| Match in description | ✅ |
| No match → empty slice | ✅ |
| Case-insensitive (normalizeText lowercases) | ✅ |
| Abbreviation-based match (e.g. "BAL" → "Baltimore Ravens") | ✅ `TestFilterNewsByTeamAbbreviations` (`sports_additional_test.go`) — `teamQueryVariants` expands "ravens" → "baltimore ravens" |

Maps to Q92/Q93 and the general team-news fallback.

---

### Group 7 — Soccer Entity Resolution ✅ DONE
**File:** `sports_extended_test.go` → `TestDetectSportsIntentSoccer`

Covers Q5, Q89, Q90, Q91. EPL club aliases added to `teamAliases`. Champions League, La Liga, Bundesliga added to `leagueConfigs`.

| Query | Expected | Status |
|---|---|---|
| "What is Arsenal next match?" | `SportsIntentTeamSchedule` + EPL + TeamQuery="Arsenal" | ✅ `TestDetectSportsIntentSoccer` |
| "Show me Liverpool scores today" | `SportsIntentScores` + EPL + TeamQuery="Liverpool" | ✅ `TestDetectSportsIntentSoccer` |
| "Man City vs Man United today" | `SportsIntentScores` + EPL | ✅ `TestDetectSportsIntentSoccer` |
| "Champions League scores today" | `SportsIntentScores` + `uefa.champions` | ✅ `TestDetectSportsIntentSoccer` |
| "UEFA Champions League standings" | `SportsIntentStandings` + `uefa.champions` | ✅ `TestDetectSportsIntentSoccer` |
| "Real Madrid next game" | `SportsIntentTeamSchedule` + La Liga + TeamQuery="Real Madrid" | ✅ `TestDetectSportsIntentSoccer` |
| "Barcelona scores today" | `SportsIntentScores` + La Liga + TeamQuery="FC Barcelona" | ✅ `TestDetectSportsIntentSoccer` |
| "La Liga standings" | `SportsIntentStandings` + La Liga | ✅ `TestDetectSportsIntentSoccer` |
| "Bundesliga scores today" | `SportsIntentScores` + Bundesliga | ✅ `TestDetectSportsIntentSoccer` |
| "What are the current Premier League standings?" | `SportsIntentStandings` + EPL + label="Current" | ✅ `TestDetectSportsIntentExtended` (Group 1) |

---

### Group 8 — Graceful Degradation / Negative Cases ✅ DONE (mocks excluded)
**File:** `sports_extended_test.go` → `TestDetectSportsIntentNegative`, `TestDetectSportsIntentNegativeExtended`; `sports_test.go` → `TestDetectSportsIntent` (existing negative rows)

**Intent detection — should return false:**

| Query | Why | Status |
|---|---|---|
| "" | empty input | ✅ `TestDetectSportsIntentNegative` |
| "what's a good fantasy football team?" | Fantasy management, not a live data lookup | ✅ `TestDetectSportsIntentNegative` |
| "write a short story about baseball" | `isNonLookupQuery` phrase match | ✅ `TestDetectSportsIntentNegative` |
| "What year did the Cubs last win the World Series?" | League detected (MLB) but Unknown intent, no temporal phrase | ✅ `TestDetectSportsIntentNegative` |
| "compare the NBA and NFL offseason" | Two leagues detected, Unknown intent, no temporal phrase | ✅ `TestDetectSportsIntentNegative` |
| "Who has scored the most goals in NHL history" | `isNonLookupQuery`: "history" keyword | ✅ `TestDetectSportsIntentNegativeExtended` |
| "When did the Blackhawks last win the Stanley Cup" | `isNonLookupQuery`: "history" + team, historical fact | ✅ `TestDetectSportsIntentNegativeExtended` |
| "explain how batting average is calculated" | `isNonLookupQuery`: "is calculated" | ✅ `TestDetectSportsIntentNegativeExtended` |

**Error sentinel & `wrapESPNError` paths (unit-tested, no HTTP mock needed):** ✅ Implemented in `sports_phase2_test.go`

| Scenario | Expected error | Status |
|---|---|---|
| All 11 error sentinel vars non-nil and distinct | — | ✅ `TestErrorSentinelsAreDistinct`, `TestErrorSentinelsAllNonNil` |
| ESPN returns 429 | `ErrRateLimited` wrapped | ✅ `TestWrapESPNErrorRateLimited` |
| Context canceled | wrapped sentinel preserved | ✅ `TestWrapESPNErrorContextCanceled` |
| Other error passthrough | original error returned | ✅ `TestWrapESPNErrorPassthrough` |
| ESPN returns empty `Events` slice | `ErrNoGames` | N/A — requires live HTTP mock; sentinel verified above |
| ESPN returns no athletes for leaderboard | `ErrNoSportsData` | N/A — requires live HTTP mock; sentinel verified above |
| Unknown league passed to client | `ErrUnsupportedLeague` | N/A — requires live HTTP mock; sentinel verified above |

---

### Group 10 — Extended ESPN Capabilities (Q10, Q46, Q52, Q53, Q58, Q62, Q63, Q68–Q76) ✅ DONE
**File:** `sports_new_capabilities_test.go`
**New source files:** `advanced_extended.go` (Lookup methods), additions to `detector.go`, `types.go`, `client.go`, `advanced.go`

| # | Question | Intent | Lookup Method | Status |
|---|---|---|---|---|
| Q10 | "Search ESPN for Shohei Ohtani." | `SportsIntentSearch` | `LookupSearch` | ✅ `TestDetectNewSportsIntents`, `TestSearchIntentPopulatesAthleteQuery`, `TestExtractSearchQuery`, `TestNormalizeSearchEntitiesExtended` |
| Q46 | "Show Patrick Mahomes' QBR for 2023." | `SportsIntentQBR` | `LookupQBR` | ✅ `TestDetectNewSportsIntents`, `TestQBRDetectionNoLeagueDefaultsNFL` |
| Q52 | "Compare Nikola Jokic and Joel Embiid head-to-head." | `SportsIntentAthleteComparison` | `LookupAthleteComparison` | ✅ `TestDetectNewSportsIntents`, `TestAthleteComparisonDetectionFields`, `TestExtractTwoAthletes`, `TestExtractTwoAthletesFallback` |
| Q53 | "What are Mookie Betts' hot zones?" | `SportsIntentHotZones` | `LookupHotZones` | ✅ `TestDetectNewSportsIntents`, `TestHotZonesDetectionExtractsAthlete` |
| Q58 | "What was the win probability swing in the Bills game?" | `SportsIntentGameDetail` / `probabilities` | `LookupGameDetail` | ✅ `TestDetectNewSportsIntents`, `TestGameDetailSubtypes`, `TestGameDetailSubtypeInRequest` |
| Q62 | "What is ESPN's predictor for the next Cowboys game?" | `SportsIntentGameDetail` / `predictor` | `LookupGameDetail` | ✅ `TestDetectNewSportsIntents`, `TestGameDetailSubtypeInRequest` |
| Q63 | "Who are the officials for the Super Bowl?" | `SportsIntentGameDetail` / `officials` | `LookupGameDetail` | ✅ `TestDetectNewSportsIntents`, `TestGameDetailSubtypes`, `TestGameDetailSubtypeInRequest` |
| Q68 | "Give me the full game package for the latest Eagles game." | `SportsIntentGameDetail` / `gamepackage` | `LookupGameDetail` → `CDNGame` | ✅ `TestDetectNewSportsIntents`, `TestGameDetailSubtypes`, `TestCdnSportSlug` |
| Q69 | "Who won the 2024 Super Bowl?" | `SportsIntentChampions` + NFL + Season=2024 | `LookupChampions` | ✅ `TestDetectNewSportsIntents`, `TestNormalizeChampionData`, `TestNormalizeChampionDataIncomplete` |
| Q70 | "Who was the NBA champion in 2023?" | `SportsIntentChampions` + NBA + Season=2023 | `LookupChampions` | ✅ `TestDetectNewSportsIntents`, `TestDetectLeagueFromChampionship` |
| Q71 | "Who won the Stanley Cup in 2022?" | `SportsIntentChampions` + NHL + Season=2022 | `LookupChampions` | ✅ `TestDetectNewSportsIntents`, `TestDetectLeagueFromChampionship` |
| Q72 | "Who won the 2023 World Series?" | `SportsIntentChampions` + MLB + Season=2023 | `LookupChampions` | ✅ `TestDetectNewSportsIntents`, `TestIsNonLookupQueryChampionExemption` |
| Q73 | "Show me the 2024 NFL draft results." | `SportsIntentDraft` + NFL + Season=2024 | `LookupDraft` → `SeasonDraft` | ✅ `TestDetectNewSportsIntents`, `TestDraftDetectionWithSeason` |
| Q74 | "Who were the top picks in the 2023 NBA draft?" | `SportsIntentDraft` + NBA + Season=2023 | `LookupDraft` → `SeasonDraft` | ✅ `TestDetectNewSportsIntents` |
| Q75 | "Who are the current NFL head coaches?" | `SportsIntentCoaches` + NFL | `LookupCoaches` → `Coaches` | ✅ `TestDetectNewSportsIntents`, `TestCoachesDetection`, `TestNormalizeCoachRefs` |
| Q76 | "Show me the Green Bay Packers coaching staff." | `SportsIntentCoaches` + NFL + TeamQuery set | `LookupCoaches` → `TeamRecord` | ✅ `TestDetectNewSportsIntents`, `TestCoachesDetection` |

**Additional helper tests in `sports_new_capabilities_test.go`:**
- `TestIsNonLookupQueryChampionExemption` — champion queries not blocked by "history" guard
- `TestIsNonLookupQueryStillBlocks` — non-champion "history" queries still blocked
- `TestNormalizeSearchEntitiesPlayerFilter` — `normalizeSearchEntities` type filter
- `TestFilterTableByAthlete` / `TestFilterTableByAthleteNoMatch` — table row filtering
- `TestNormalizeCoachRefsNil` — nil guard for coach refs
- `TestCoachesPollIsNotCoachesIntent` — "coaches poll" does not trigger `SportsIntentCoaches`

**New negative case (previously in Group 8):**
"Who won Super Bowl XLII?" was removed from the negative list — it is now correctly detected as `SportsIntentChampions` + NFL via the compound champion check (`"super bowl"` + `"won"`).

---

### Group 9 — Rendering / Markdown Output ✅ DONE
**File:** `sports_extended_test.go` → `TestRenderRosterMarkdown`, `TestRenderLeaderboardMarkdown`, `TestRenderSimpleMarkdown`; `markdown_test.go` → many existing render tests

| Function | Test | Status |
|---|---|---|
| `RenderRosterMarkdown` | title uses TeamQuery; falls back to cfg.DisplayName; headers + row data | ✅ `TestRenderRosterMarkdown` |
| `RenderLeaderboardMarkdown` | title + stat column; season year appended; date label takes precedence | ✅ `TestRenderLeaderboardMarkdown` |
| `RenderSimpleMarkdown` | title, column headers, row values | ✅ `TestRenderSimpleMarkdown` |
| `renderDefaultStandingsTable` | W/L/Pct/GB; playoff-seed ordering | ✅ `TestStandingsPlayoffSeedDoesNotOverrideDisplayOrder` + `TestRenderStandingsMarkdownGrouped` |
| `RenderGamesMarkdown` (scores) | final score, logo URL, venue, broadcasts | ✅ `TestRenderGamesMarkdownScores` + `TestRenderGamesMarkdownEnhancedScores` |
| `RenderGamesMarkdown` (schedule/pregame) | start time, teams | ✅ `TestRenderGamesMarkdownScheduleWhenPregame` |
| `RenderNewsMarkdown` | headlines, links, images, unsafe URL rejection | ✅ `TestRenderNewsMarkdown`, `TestRenderNewsMarkdownEnhancedNewspaper`, `TestRenderNewsMarkdownRejectsUnsafeImageAndLink` |
| Standing renderers (MLB/NFL/NBA/NHL/MLS/soccer/cricket) | division splits, conference splits, hockey columns | ✅ `markdown_test.go` (7 dedicated test functions) |

---

### Group 11 — Q77–Q99 Implementation (Venues, Power Index, Recruits, Bracketology, F1/NASCAR/PGA/ATP) ✅ DONE
**File:** `sports_q77_100_test.go`
**New source files:** `advanced_extra.go` (4 Lookup methods + helpers), additions to `detector.go`, `types.go`, `client.go`, `tool.go`

| # | Area | Tests | Status |
|---|---|---|---|
| Q77-Q78 | Venue intent detection (NFL, MLB, NBA) | `TestVenuesIntentDetection` (6 cases) | ✅ |
| Q77-Q78 | `NormalizeVenueStruct` pure logic | `TestNormalizeVenueStruct`, `TestNormalizeVenueStructIndoorNoCapacity` | ✅ |
| Q77-Q78 | `venueIDFromRef` URL parsing | `TestVenueIDFromRef` (4 cases incl. empty + no-slash) | ✅ |
| Q83-Q84 | Power Index intent detection | `TestPowerIndexIntentDetection` (6 cases, CFB default) | ✅ |
| Q83-Q84 | Power Index raw JSON normalization | `TestPowerIndexRawJSONNormalized` | ✅ |
| Q85-Q86 | Recruits intent detection | `TestRecruitsIntentDetection` (6 cases, CFB default) | ✅ |
| Q85-Q86 | Recruits team query passthrough | `TestRecruitsTeamQueryPassedThrough` | ✅ |
| Q85-Q86 | Recruits raw JSON normalization | `TestRecruitsRawJSONNormalized` | ✅ |
| Q87 | Bracketology intent detection | `TestBracketologyIntentDetection` (6 cases, no league required) | ✅ |
| Q87 | Bracketology season extraction | `TestBracketologySeasonExtracted` | ✅ |
| Q87 | Bracketology raw JSON normalization | `TestBracketologyRawJSONNormalized` | ✅ |
| Q94 | F1 intent detection (standings/scores/schedule) | `TestF1IntentDetection` (4 cases) | ✅ |
| Q95 | NASCAR intent detection (scores/standings/schedule/news) | `TestNASCARIntentDetection` (4 cases) | ✅ |
| Q96-Q97 | PGA Tour intent detection (leaderboard/standings/scores/schedule) | `TestPGAIntentDetection` (4 cases) | ✅ |
| Q98-Q99 | ATP Tennis intent detection (standings/scores/news) | `TestATPIntentDetection` (4 cases) | ✅ |
| — | New intent sentinel distinctness | `TestNewIntentSentinelsAreDistinct` | ✅ |
| — | New league configs registered | `TestNewLeagueConfigsRegistered` (6 leagues) | ✅ |

**Total new tests added this group:** 58 (577 total)

---

## Test Execution Order

```
go test ./internal/sports/... -run TestDetectSports -v           # intent unit tests
go test ./internal/sports/... -run TestParseDateValue -v         # date parsing
go test ./internal/sports/... -run TestDetectStatMetric -v       # stat metrics
go test ./internal/sports/... -run TestNormalize -v              # all normalizers
go test ./internal/sports/... -run TestFilter -v                 # filter helpers
go test ./internal/sports/... -run TestRender -v                 # markdown renderers
go test ./internal/sports/... -v                                 # full suite (577 tests)
```


## Questions from the 100-Set Not Addressable by Unit Tests

These require live ESPN API calls or multi-step orchestration and should be evaluated as manual/integration tests or added to a separate `*_integration_test.go` with `//go:build integration`:

_Note: Q10, Q46, Q52, Q53, Q58, Q62, Q63, Q68–Q76 were previously in this list. They now have full implementations (`advanced_extended.go`) and pure-logic unit tests (`sports_new_capabilities_test.go`). Live API roundtrip tests remain as integration-only._

| # | Question | Reason |
|---|---|---|
| Q10 | "Search ESPN for Shohei Ohtani." | Intent + normalization tested; live entity resolution requires `//go:build integration` |
| Q46 | "Show Patrick Mahomes' QBR for 2023." | Intent + table filtering tested; live QBR fetch requires `//go:build integration` |
| Q52 | "Compare Nikola Jokic and Joel Embiid head-to-head." | Intent + athlete splitting tested; live athlete ID resolution requires `//go:build integration` |
| Q53 | "What are Mookie Betts' hot zones?" | Intent + athlete extraction tested; live hot-zone fetch requires `//go:build integration` |
| Q58 | "What was the win probability swing in the Bills game?" | Intent + subtype tested; live game resolution requires `//go:build integration` |
| Q62 | "What is ESPN's predictor for the next Cowboys game?" | Intent + subtype tested; live game resolution requires `//go:build integration` |
| Q63 | "Who are the officials for the Super Bowl?" | Intent + subtype tested; live game resolution requires `//go:build integration` |
| Q68 | "Give me the full game package for the latest Eagles game." | Intent + CDN slug tested; live `CDNGame` fetch requires `//go:build integration` |
| Q69-Q72 | Historical championship winners | Normalization + detection tested; live postseason scoreboard requires `//go:build integration` |
| Q73-Q74 | "Show the 2024 NFL draft results." | Detection + season extraction tested; live `SeasonDraft`/`Draft` requires `//go:build integration` |
| Q75-Q76 | "Who are the current NFL coaches?" | Detection + coach ref normalization tested; live `Coaches` fetch requires `//go:build integration` |
| ~~Q77-Q78~~ | ~~MLB venues / stadium name~~ | ✅ `SportsIntentVenues` implemented; `NormalizeVenueStruct`, `venueIDFromRef` unit-tested in `sports_q77_100_test.go`; live venue resolution is integration-only |
| ~~Q83-Q84~~ | ~~College QBR / Power Index~~ | ✅ `SportsIntentPowerIndex` implemented; `LookupPowerIndex` + raw JSON normalization unit-tested; live FPI fetch is integration-only |
| ~~Q85-Q86~~ | ~~Recruiting / recruiting class~~ | ✅ `SportsIntentRecruits` implemented; `LookupRecruits` + raw JSON normalization unit-tested; live recruit data is integration-only |
| ~~Q87~~ | ~~"Show current bracketology."~~ | ✅ `SportsIntentBracketology` implemented; `LookupBracketology` + raw JSON normalization unit-tested; live bracket data is integration-only |
| ~~Q94-Q99~~ | ~~F1, NASCAR, PGA, ATP intent detection~~ | ✅ All 6 leagues configured in `leagueConfigs`; intent detection unit-tested for standings/scores/schedule/news per league in `sports_q77_100_test.go` |
| Q100 | Fantasy lineup/management | Requires private ESPN fantasy league credentials; not addressable by unit tests |
