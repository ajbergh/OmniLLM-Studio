> **Archived — historical implementation export.** Retained for feature lineage; it is not active planning.

# ESPN Sports Lookup Tool Implementation Export

Last reviewed: 2026-05-17  
Primary code paths:

- `backend/internal/sports/`
- `backend/internal/router/`
- `backend/internal/api/message_handler.go`
- `backend/internal/tools/`

This document captures how OmniLLM-Studio's ESPN-backed sports lookup works so the design can be ported into a similar project. It covers the public tool API, the chat preflight path, the LLM router used for question interpretation, the deterministic fallback detector, ESPN API access through `espn-go`, graceful fallback behavior, result validation, Markdown rendering, and test strategy.

## Executive Summary

The sports feature is implemented as a deterministic backend tool around ESPN public APIs. It is exposed in two ways:

1. **Direct preflight in chat**: before calling the main conversation model, the message handler tries to classify sports questions and answer them directly with ESPN data.
2. **Registered tool calling**: the same capability is registered as `sports_lookup`, so supported LLM providers can call it during a normal streamed chat tool loop.

The app also has a small **LLM router** that can interpret a user question into structured sports parameters before the deterministic detector is used. The router does not answer the question. It only returns JSON such as route, confidence, intent, league, team, date, season, metric, and subtype. If that router is disabled, unavailable, low confidence, invalid, or says the message should go to the normal LLM, the system falls back to the local deterministic sports detector.

The core design principle is:

> Use ESPN data for current, factual, ESPN-backed sports questions; use the normal LLM for explanations, creative writing, subjective questions, and unsupported sports requests.

## Core Dependencies

The ESPN integration depends on:

```go
github.com/chinmaykhachane/espn-go v0.1.1
```

The app wraps this library in `backend/internal/sports/ESPNClient`.

`NewESPNClient()` configures:

```go
espn.New(
    espn.WithUserAgent("OmniLLM-Studio/1.0"),
    espn.WithTimeout(10*time.Second),
    espn.WithMaxRetries(3),
    espn.WithBackoff(250*time.Millisecond),
)
```

This gives every lookup:

- a clear user agent,
- a 10 second upstream timeout,
- retries for transient ESPN failures,
- backoff between retries.

## Main Concepts

### SportsRequest

`SportsRequest` is the internal request object used everywhere after interpretation:

```go
type SportsRequest struct {
    RawQuery           string
    Intent             SportsIntentType
    League             string
    Sport              string
    TeamQuery          string
    AthleteQuery       string
    SecondAthleteQuery string
    GameDetailSubtype  string
    StatCategory       string
    StatName           string
    StatLabel          string
    StatSort           string
    Date               *time.Time
    DateLabel          string
    Season             int
    Limit              int
    RenderMode         SportsRenderMode
    LeagueLogoURL      string
}
```

Important fields:

- `RawQuery`: original user text; kept for fallback parsing and metadata.
- `Intent`: normalized operation such as `scores`, `standings`, `news`, `leaders`, or `game_detail`.
- `League` and `Sport`: ESPN-compatible league/sport identifiers.
- `TeamQuery`: human team name used for team resolution and filtering.
- `AthleteQuery` and `SecondAthleteQuery`: used for athlete stats, news, comparisons, QBR filters, hot zones, etc.
- `GameDetailSubtype`: sub-operation for detail lookups, such as `pitching_matchups`, `broadcasts`, `officials`, `probabilities`, `predictor`, `plays`, or `gamepackage`.
- `StatCategory`, `StatName`, `StatLabel`, `StatSort`: normalized stat routing for leaderboards.
- `Date` and `DateLabel`: machine date plus user-facing label.
- `Season`: year-like season value, for historical schedules, leaders, drafts, QBR, etc.
- `Limit`: max rows/articles.
- `RenderMode`: `plain_markdown`, `enhanced_markdown`, or `html_markdown`.

### SportsLookupResult

Every successful lookup returns:

```go
type SportsLookupResult struct {
    Intent        SportsIntentType
    League        string
    LeagueName    string
    LeagueLogoURL string
    Sport         string
    DateLabel     string
    Markdown      string
    Source        string
    RetrievedAt   time.Time
    RenderMode    SportsRenderMode
}
```

The chat path uses `Markdown` as the assistant response. The tool-calling path serializes the same information to JSON and also puts the Markdown into metadata.

### Source Label

The source is deliberately explicit:

```go
const SourceESPN = "ESPN public API via espn-go"
```

This source is included in metadata and in rendered Markdown source lines.

## Supported Intents

The feature supports a large intent set. The most important production intents are:

| Intent | Purpose |
|---|---|
| `scores` | Current or dated scoreboard results. |
| `schedule` | Current, future, or dated game schedule. |
| `standings` | League standings. |
| `news` | League, team, or broad ESPN sports news. |
| `odds` | ESPN-provided betting odds from scoreboard payloads. |
| `roster` | Team roster. |
| `injuries` | Team or league injuries. |
| `transactions` | Team or league transactions. |
| `team_record` | Team record extracted from standings. |
| `team_schedule` | Team-specific schedule. |
| `leaders` | Player/stat leaderboard. |
| `athlete_stats` | Athlete profile/stats/gamelog/splits/bio style lookups. |
| `athlete_news` | Athlete-specific news. |
| `rankings` | College rankings. |
| `league_stats` | League/team statistics when a stat query is not athlete-specific. |
| `teams` | Team list for a league/season. |
| `team_history` | Team/franchise history. |
| `seasons` | Available seasons. |
| `calendar` | League calendar. |
| `scoreboard_header` | Cross-sport or broadcast-oriented scoreboard summaries. |
| `search` | ESPN search/entity lookup. |
| `qbr` | Total QBR leaderboard and athlete filtering. |
| `athlete_comparison` | Two-athlete comparison. |
| `hot_zones` | Athlete hot-zone/splits fallback. |
| `game_detail` | Summary, plays, probabilities, predictor, officials, game package, etc. |
| `champions` | Championship/historical winner style lookups. |
| `draft` | Draft results and team draft filtering. |
| `coaches` | Coach listings and coach record style lookups. |
| `venues` | Venue/stadium/arena lookup. |
| `power_index` | ESPN power indexes such as FPI/BPI style data. |
| `recruits` | College football recruit rankings/classes. |
| `bracketology` | NCAA bracketology projection endpoint. |
| `tournaments` | Golf/tennis tournament lists. |
| `fantasy` | ESPN fantasy league/player info paths. |

## League and Team Normalization

League configuration lives in `backend/internal/sports/detector.go`.

Each league has:

```go
type LeagueConfig struct {
    DisplayName string
    Sport       string
    League      string
    Aliases     []string
}
```

Examples:

- MLB maps to `espn.SportBaseball` and `espn.LeagueMLB`.
- NFL maps to `espn.SportFootball` and `espn.LeagueNFL`.
- NBA/WNBA map to `espn.SportBasketball`.
- NHL maps to `espn.SportHockey`.
- EPL, MLS, Champions League, La Liga, Bundesliga, Serie A, and Ligue 1 map to soccer ESPN slugs.
- F1 and NASCAR map to racing.
- PGA maps to golf.
- ATP maps to tennis.
- IPL is special: ESPN cricket exposes IPL as series `8048`, so the app defines:

```go
const LeagueIPL = "8048"
```

Team normalization uses a large `teamAliases` table. Each alias row has:

```go
type teamAlias struct {
    League    string
    TeamQuery string
    Aliases   []string
}
```

Examples:

- `cubs`, `chc` -> `Chicago Cubs`
- `chiefs`, `kc chiefs` -> `Kansas City Chiefs`
- `lakers`, `la lakers` -> `Los Angeles Lakers`
- `la kings` -> `Los Angeles Kings`
- `csk` -> `Chennai Super Kings`
- `man city` -> `Manchester City`

There is also a typo-tolerance fallback for single-word aliases. `detectTeamAliasFuzzy` compares user tokens to single-word aliases using Levenshtein distance `<= 1`, but only for tokens and aliases with enough length to avoid false positives. This catches examples like `seahaks` -> `Seahawks` and `knics` -> `Knicks`.

## User Request Flow

### Non-Streaming Chat Flow

In `MessageHandler.Create`, the backend:

1. Saves the user message.
2. Builds the normal LLM request.
3. Resolves URL context first; URL-specific prompts take precedence over sports.
4. Runs sports preflight if URL context did not handle the message:
   1. Try the LLM router through `tryRouterSportsLookup`.
   2. If router returns a valid `sports_lookup` decision, map it to `SportsRequest`.
   3. Execute ESPN lookup and save a direct assistant message.
   4. If router falls back and does not say to skip local sports, run deterministic `DetectSportsIntent`.
   5. If local detection succeeds, execute ESPN lookup and save a direct assistant message.
5. If sports does not handle the message, continue with file search, RAG, artifact directives, web search, and normal LLM completion.

### Streaming Chat Flow

`MessageHandler.Stream` does the same sports preflight, but emits SSE events:

- `router` when router trace display is enabled,
- `sports_lookup` when the lookup completes,
- `token` containing the rendered Markdown,
- `done` with provider/model metadata.

When sports handles the request, provider and model are set to:

```text
provider = sports_lookup
model    = espn-go
```

This makes sports answers visibly distinct from model-generated answers.

### Tool Calling Flow

The feature is also registered as a regular tool:

```go
toolRegistry.MustRegister(sports.NewSportsLookupTool(sports.NewESPNClient()))
```

The tool appears in `/v1/tools`, can have `allow`, `deny`, or `ask` permission policy, and can be manually executed through the tool API.

In streaming LLM chat, the backend injects enabled tools from the registry into `llmReq.Tools` for non-Gemini providers. If the model calls `sports_lookup`, the generic tool executor validates permissions, validates arguments, executes the sports tool, appends the result as a `tool` message, and lets the LLM produce the final answer from that tool output.

The direct sports preflight is separate from this generic tool loop. In practice, most high-confidence sports questions are answered before the main LLM is called.

## The `sports_lookup` Tool API

The tool type is `SportsLookupTool` in `backend/internal/sports/tool.go`.

### Tool Definition

Name:

```text
sports_lookup
```

Description:

```text
Fetch ESPN-backed sports scores, schedules, standings, news, betting odds,
rosters, injuries, transactions, team records, rankings, player stats,
league stats, and league leaders, including IPL cricket. Use this for current
or ESPN-specific sports questions instead of answering from model memory.
```

### Input Schema

The JSON schema accepts:

| Field | Required | Type | Meaning |
|---|---:|---|---|
| `query` | yes | string | Raw user request. |
| `intent` | no | string enum | Explicit intent override. |
| `league` | no | string | League alias or ESPN league string. |
| `date` | no | string | `YYYY-MM-DD`, `today`, `tomorrow`, or `yesterday`. |
| `limit` | no | integer | Row/article cap. |
| `render_mode` | no | string enum | `plain_markdown`, `enhanced_markdown`, `html_markdown`. |

The documented schema enum in `Definition()` lists the main high-value intents, while `Validate()` accepts the full extended intent set. If porting, keep the schema and validator aligned so LLM callers can discover every supported intent.

### Example Manual Tool Call

```json
{
  "name": "sports_lookup",
  "arguments": {
    "query": "Show me today's MLB scores",
    "intent": "scores",
    "league": "mlb",
    "date": "today",
    "limit": 25,
    "render_mode": "enhanced_markdown"
  }
}
```

### Tool Execution Steps

`SportsLookupTool.Execute` does the following:

1. Parse JSON args.
2. Run `DetectSportsIntent(query, now)`.
3. If detection fails, create a minimal `SportsRequest` with `Intent=unknown`.
4. Validate dates found inside the raw query with `ValidateDateInQuery`.
5. Apply explicit overrides:
   - `intent` overrides detected intent.
   - `league` resolves by alias or ESPN league string.
   - `date` resolves with `ParseDateValue`.
   - `limit` overrides detected/default limit.
   - `render_mode` overrides default rendering.
6. Call `ESPNClient.Lookup(ctx, req)`.
7. If ESPN lookup returns an error, return a `ToolResult` with:
   - `IsError: true`,
   - user-facing message from `UserFacingError`,
   - metadata including tool, source, and raw error.
8. If successful, marshal `sportsLookupOutput` JSON containing:
   - `intent`,
   - `league`,
   - `league_name`,
   - `league_logo_url`,
   - `markdown`,
   - `source`,
   - `retrieved_at`,
   - `render_mode`.

### Output Example

```json
{
  "intent": "scores",
  "league": "mlb",
  "league_name": "MLB",
  "league_logo_url": "https://a.espncdn.com/i/teamlogos/leagues/500/mlb.png",
  "markdown": "### MLB Scores - Today\n\n_Source: ESPN public API via espn-go ..._",
  "source": "ESPN public API via espn-go",
  "retrieved_at": "2026-05-17T18:45:00Z",
  "render_mode": "enhanced_markdown"
}
```

## LLM Router for Question Interpretation

The router lives in `backend/internal/router`.

It is intended to be a cheap, structured classification pass before the main LLM. It does not answer user questions.

### Router Settings

The router reads typed app settings from `SettingsRepo`. Key settings include:

- `RouterEnabled`
- `RouterMode`
- `RouterProvider`
- `RouterModel`
- `RouterTimeoutMS`
- `RouterMaxTokens`
- `RouterTemperature`
- `RouterStructuredOutputMode`
- `RouterConfidenceThreshold`
- `RouterFallbackBehavior`
- `RouterShowTrace`

For sports preflight, `MessageHandler.tryRouterSportsLookup` calls:

```go
h.routerSvc.Route(ctx, intentrouter.RouteRequest{
    UserMessage:     query,
    ConversationID:  conversationID,
    Mode:            intentrouter.RouterModeSportsOnly,
    AvailableRoutes: []intentrouter.RouteName{
        intentrouter.RouteSportsLookup,
        intentrouter.RouteNormalLLM,
        intentrouter.RouteClarify,
    },
})
```

### Router Prompt

The router system prompt tells the model:

- return only JSON matching the schema,
- never answer the user,
- classify and extract fields only,
- use `sports_lookup` only for ESPN-backed factual/current sports data,
- use `normal_llm` for explanations, definitions, creative writing, subjective analysis, logo/image requests, and sports questions that do not need ESPN current data,
- use `clarify` only when a short question can resolve a missing required route parameter.

It also includes special instructions:

- MLB pitching matchups/probable pitchers should route to `sports_lookup`, intent `schedule`, league `MLB`, and `game_detail_subtype="pitching_matchups"`.
- Normalize common leagues to MLB, NFL, NBA, WNBA, NHL, NCAAF, NCAAMB, NCAAWB, EPL, MLS, UCL, LALIGA, BUNDESLIGA, SERIEA, LIGUE1, IPL, F1, NASCAR, PGA, or ATP.

### Router Output Schema

The router response is a strict JSON object:

```go
type RouterDecision struct {
    Route                 RouteName          `json:"route"`
    Confidence            float64            `json:"confidence"`
    RequiresGenerationLLM bool               `json:"requires_generation_llm"`
    RewrittenQuery        string             `json:"rewritten_query,omitempty"`
    ClarifyingQuestion    string             `json:"clarifying_question,omitempty"`
    Reason                string             `json:"reason,omitempty"`
    Sports                *SportsRouteParams `json:"sports,omitempty"`
}
```

Sports params:

```go
type SportsRouteParams struct {
    Intent             string `json:"intent,omitempty"`
    League             string `json:"league,omitempty"`
    Sport              string `json:"sport,omitempty"`
    TeamQuery          string `json:"team_query,omitempty"`
    AthleteQuery       string `json:"athlete_query,omitempty"`
    SecondAthleteQuery string `json:"second_athlete_query,omitempty"`
    Metric             string `json:"metric,omitempty"`
    Date               string `json:"date,omitempty"`
    DateLabel          string `json:"date_label,omitempty"`
    Season             *int   `json:"season,omitempty"`
    Limit              *int   `json:"limit,omitempty"`
    GameDetailSubtype  string `json:"game_detail_subtype,omitempty"`
}
```

### Router Decision Example

For:

```text
What are the pitching matchups for today's MLB games?
```

Expected router JSON:

```json
{
  "route": "sports_lookup",
  "confidence": 0.95,
  "requires_generation_llm": false,
  "rewritten_query": "",
  "clarifying_question": "",
  "reason": "The user asks for ESPN-backed MLB schedule detail.",
  "sports": {
    "intent": "schedule",
    "league": "MLB",
    "sport": "baseball",
    "team_query": "",
    "athlete_query": "",
    "second_athlete_query": "",
    "metric": "",
    "date": "",
    "date_label": "today",
    "season": null,
    "limit": 25,
    "game_detail_subtype": "pitching_matchups"
  }
}
```

### Mapping Router Output to SportsRequest

`SportsRequestFromDecision` converts `RouterDecision` into `sports.SportsRequest`.

Responsibilities:

- validate route is `sports_lookup`,
- validate `sports` params exist,
- normalize intent strings to `SportsIntentType`,
- normalize league aliases to ESPN sport/league pair,
- copy team/athlete/subtype/date/season/limit,
- clamp limit to `1..50`,
- default limit to `25`,
- map metrics such as `goals`, `home runs`, `passing yards` to ESPN stat category/name/sort,
- parse ISO dates,
- resolve date labels `today`, `tomorrow`, `yesterday` relative to `now`,
- add `pitching_matchups` subtype if raw text asks for probable starters but router omitted the subtype.

### Router Validation and Fallback

The router fails open into fallback. It does not block normal chat unless configured to clarify.

Fallback is triggered when:

- router is disabled,
- router mode is off,
- provider or model is missing,
- LLM call errors,
- LLM output is invalid JSON,
- route is unsupported,
- confidence is below threshold,
- `sports_lookup` route lacks `sports` params,
- sports param mapping fails.

Telemetry records:

```go
type RouterTelemetry struct {
    Enabled              bool
    Mode                 RouterMode
    Provider             string
    Model                string
    LatencyMS            int
    Confidence           float64
    Route                RouteName
    Validated            bool
    FallbackUsed         bool
    FallbackReason       string
    StructuredOutputMode string
    Error                string
}
```

Fallback reasons include:

- `router_disabled`
- `missing_provider_or_model`
- `llm_error`
- `invalid_json`
- `low_confidence`
- `unsupported_route`
- `missing_sports_params`
- `sports_mapping_failed`
- `router_error`
- `local_detector`

## Deterministic Local Detector

The local fallback detector is `DetectSportsIntent(query, now)` in `backend/internal/sports/detector.go`.

It is intentionally conservative. It returns `ok=false` when a query might be better handled by the normal LLM.

### High-Level Detector Algorithm

```go
func DetectSportsIntent(query string, now time.Time) (*SportsRequest, bool) {
    raw := strings.TrimSpace(query)
    if raw == "" {
        return nil, false
    }

    norm := normalizeText(raw)
    if isNonLookupQuery(norm) {
        return nil, false
    }

    cfg, hasLeague := detectLeague(norm)
    teamQuery := detectTeamAliasOrFuzzy(norm)
    if team implies league and no explicit league {
        cfg = team league
        hasLeague = true
    }

    intent := detectIntent(norm)
    season := parseSeasonFromQuery(raw)
    limit := parseLimitFromQuery(norm, defaultLimitForIntent(intent))

    handle stat leader intent and metric mapping
    handle athlete stats/news extraction
    handle extended intent-specific request construction
    upgrade unknown+temporal+league to scores/schedule
    reject unsupported no-league cases
    parse date phrase

    return &SportsRequest{...}, true
}
```

### Normalization

`normalizeText` lowercases and keeps only letters/digits, replacing punctuation with spaces. This makes phrase matching robust against:

- apostrophes,
- punctuation,
- capitalization,
- common sentence formatting.

### Non-Lookup Guardrails

`isNonLookupQuery` blocks prompts such as:

- "write a story about baseball"
- "write a sports news article"
- "make a sports logo"
- "explain how betting odds work"
- "explain how standings work"
- "explain how batting average is calculated"
- subjective/all-time/history questions unless explicitly supported by an ESPN endpoint path.

This is critical. Without these guardrails, the sports detector would steal creative and explanatory requests from the main LLM.

### Intent Detection Priority

`detectIntent` checks specific/extended capabilities before broad generic phrases. This avoids collisions.

Examples:

- `bracketology` before generic tournament/standings handling.
- `recruiting class` before rankings.
- `power index`, `FPI`, `BPI` before generic rankings.
- venue/stadium/arena phrases before generic team info.
- champions/draft/coaches before generic scores or roster.
- game detail subtypes before scoreboard.
- `qbr`, `hot zones`, `athlete comparison`, and ESPN search before generic athlete stats.
- odds phrases before standings/schedule.
- leaders before athlete stats.
- standings before scores.
- scores before schedule.
- news last, because words like "latest" are broad.

### Temporal Parsing

Date parsing supports:

- `today`
- `tonight`
- `yesterday`
- `last night`
- `tomorrow`
- `current`
- `this weekend`
- `this week`
- named weekdays
- holidays:
  - Christmas Day
  - Thanksgiving
  - New Year's Eve
  - New Year's Day
  - Independence Day / Fourth of July
  - Super Bowl Sunday
- ISO dates: `YYYY-MM-DD`
- slash dates: `M/D/YYYY`

Important behavior:

- "current" maps to no single date for standings but maps to today for most other intents.
- "this weekend" resolves to the nearest upcoming Saturday.
- "this week" sets a label only and does not create a single date.
- named weekdays resolve to the next occurrence; same-day weekday means seven days later.
- explicit malformed dates are rejected by `ValidateDateInQuery`.

### Stat Metric Mapping

Stat leaders use `statMetricConfigs`, which map user phrases to ESPN statistics.

Examples:

| User phrase | League default | Category | Stat name | Sort |
|---|---|---|---|---|
| `home runs`, `hr`, `homers` | MLB | `batting` | `homeRuns` | `batting.homeRuns:desc` |
| `rbi`, `rbis` | MLB | `batting` | `RBIs` | `batting.RBIs:desc` |
| `era` | MLB | `pitching` | `ERA` | `pitching.ERA:asc` |
| `whip` | MLB | `pitching` | `WHIP` | `pitching.WHIP:asc` |
| `passing yards` | NFL | `passing` | `passingYards` | `passing.passingYards:desc` |
| `rushing yards` | NFL | `rushing` | `rushingYards` | `rushing.rushingYards:desc` |
| `receiving yards` | NFL | `receiving` | `receivingYards` | `receiving.receivingYards:desc` |
| `points per game`, `ppg` | NBA | `offensive` | `avgPoints` | `offensive.avgPoints:desc` |
| `rebounds` | NBA | `general` | `avgRebounds` | `general.avgRebounds:desc` |
| `assists` | NBA or NHL depending league | NBA `avgAssists` or NHL `assists` | league-specific | league-specific |
| `goals` | NHL | `scoring` | `goals` | `scoring.goals:desc` |

If no league is given and the metric has a default league, the detector can infer the league. Example: `WHIP leaders` defaults to MLB.

### Default Limits

```go
func defaultLimitForIntent(intent SportsIntentType) int {
    if intent == SportsIntentNews {
        return 10
    }
    if intent == SportsIntentLeaders {
        return 50
    }
    if intent == SportsIntentOdds {
        return 50
    }
    return 100
}
```

Router-mapped requests default to `25` and clamp explicit limits to `1..50`. Tool/direct detector requests may use intent-specific defaults.

## ESPN Client Lookup Dispatch

`ESPNClient.Lookup(ctx, req)` wraps `lookup(ctx, req)`.

The inner dispatch switches on `req.Intent`:

```go
switch req.Intent {
case SportsIntentStandings:
    return c.LookupStandings(ctx, req)
case SportsIntentScores, SportsIntentSchedule:
    return c.LookupScores(ctx, req)
case SportsIntentNews:
    return c.LookupNews(ctx, req)
case SportsIntentOdds:
    return c.LookupOdds(ctx, req)
case SportsIntentRoster:
    return c.LookupRoster(ctx, req)
case SportsIntentInjuries, SportsIntentTransactions, SportsIntentTeamSchedule,
     SportsIntentRankings, SportsIntentLeagueStats, SportsIntentCalendar:
    return c.LookupGeneric(ctx, req)
case SportsIntentTeamRecord:
    return c.LookupTeamRecord(ctx, req)
case SportsIntentTeams:
    return c.LookupTeams(ctx, req)
case SportsIntentLeaders:
    return c.LookupLeaders(ctx, req)
case SportsIntentAthleteStats, SportsIntentAthleteNews, ...:
    return c.LookupAthlete(ctx, req)
case SportsIntentSearch:
    return c.LookupSearch(ctx, req)
case SportsIntentQBR:
    return c.LookupQBR(ctx, req)
...
}
```

After calling `lookup`, `Lookup` catches "graceful" lookup errors and converts them to an empty Markdown result instead of surfacing a hard error.

## ESPN Endpoint Families Used

The implementation uses both typed `espn-go` methods and raw ESPN paths.

### Scoreboard / Scores / Schedule

Primary call:

```go
c.client.Scoreboard(ctx, req.Sport, req.League, opts)
```

Options:

- `SetDate(*req.Date)` when date is provided.
- `Limit` from request or default `100`.

Normalization:

- event date/time,
- status,
- home/away teams,
- scores,
- venue,
- broadcasts,
- period/quarter linescores,
- team logos,
- league logo.

For MLB pitching matchups, the app additionally calls the site API raw scoreboard endpoint:

```go
/apis/site/v2/sports/{sport}/{league}/scoreboard
```

with params:

```go
dates = YYYYMMDD
limit = req.Limit or 100
```

It extracts `competitors[].probables[]` and merges probable pitchers back into the normalized scoreboard rows.

### Standings

Primary call:

```go
c.client.Standings(ctx, req.Sport, req.League, req.Season)
```

Normalization recursively walks nested ESPN standings groups and extracts:

- group,
- rank,
- team identity,
- wins/losses/ties/draws/no result,
- percentage,
- games back,
- streak,
- last ten,
- points,
- games played,
- goal differential,
- net run rate,
- for/against,
- note/playoff seed context.

Rendering adapts columns by league type:

- MLB/NFL/NBA/NHL structured standings,
- soccer standings,
- cricket/IPL standings.

### News

Broad sports news uses raw ESPN Now API:

```go
c.client.GetRaw(ctx, espn.DomainNow, "/v1/sports/news", params)
```

League news uses:

```go
c.client.News(ctx, cfg.Sport, cfg.League, limit)
```

Team news tries three layers:

1. Resolve team through `Teams`.
2. Try team-specific endpoint:

```go
c.client.TeamNews(ctx, cfg.Sport, cfg.League, team.ID, limit)
```

3. If empty, call league site news with `team={teamID}`:

```go
/apis/site/v2/sports/{sport}/{league}/news?team={teamID}
```

4. If still empty, fetch more league news and filter by team aliases.

News normalization extracts:

- published time,
- headline,
- compact HTML-stripped description,
- byline,
- URL,
- first safe image URL,
- image alt text.

### Odds

Odds are derived from scoreboard events. There is no separate complex odds service in this implementation.

League odds:

```go
Scoreboard(sport, league, date/limit) -> normalizeOdds
```

Broad odds:

- iterates over every configured league,
- calls scoreboard odds rows for each,
- accumulates until limit.

Rows include:

- league,
- date/time/status,
- away/home teams,
- moneylines,
- spread,
- over/under,
- provider.

### Rosters

Process:

1. Resolve team from league teams.
2. Call:

```go
c.client.TeamRoster(ctx, cfg.Sport, cfg.League, team.ID)
```

3. Normalize grouped roster structures into:

```go
RosterRow{
    Group, Name, Position, Jersey, Age, Height, Weight, Status, HeadshotURL
}
```

The normalizer supports standard league rosters and cricket-style groups such as batters, bowlers, all-rounders, and wicket keepers.

### Leaders and League Stats

`LookupLeaders` has multiple strategies:

1. If a stat metric was detected, call league/team stat leaders.
2. If no stat metric is known and a team is present, try team core leaders.
3. If no normalized leaderboard path works, fall back to raw leader endpoints and render a generic `SimpleTable`.

Leaderboard rows:

```go
LeaderboardRow{
    Rank, Athlete, Team, Position, Value
}
```

### Athlete Lookups

`LookupAthlete` resolves an athlete entity first, then calls subtype-specific endpoints.

It supports:

- athlete stats,
- gamelog,
- splits,
- bio/profile,
- athlete news,
- awards,
- seasons,
- records,
- injuries.

Fallbacks:

- athlete news falls back to ESPN search/news when direct athlete news is empty,
- athlete stats fall back to generic stats/splits tables when the requested endpoint is missing.

### Generic Lookup

`LookupGeneric` is used for endpoint families that return varied JSON but can still be converted into a `SimpleTable`, such as:

- injuries,
- transactions,
- team schedule,
- rankings,
- league stats,
- calendar.

For team injuries and team transactions, it first tries team-specific helpers. If they fail, it falls back to league-level generic endpoints.

### Advanced and Extended Lookups

Additional files implement the extended capability set:

- `advanced_catalog.go`: scoreboard header, teams, team history, seasons, tournaments, fantasy.
- `advanced_extended.go`: ESPN search, QBR, athlete comparison, hot zones, game detail, champions, draft, coaches.
- `advanced_extra.go`: venues, power index, recruits, bracketology.

These methods often use raw ESPN JSON, normalize it into `SimpleTable`, and validate required shape before rendering.

## Validation Layer

Validation is in `backend/internal/sports/validation.go`.

The goal is to avoid answering a specific question with a generic or mismatched table. For example, if the user asks for pitching matchups, a generic MLB schedule without probable pitchers should not be treated as a correct answer.

### Validation Error Types

```go
var (
    ErrSportsResultMismatch        = errors.New("sports result does not match request")
    ErrSportsResultMissingRequired = errors.New("sports result missing required fields")
)
```

Validation errors include a code:

```go
type SportsValidationError struct {
    Code        string
    Message     string
    RetryHint   string
    Recoverable bool
    Err         error
}
```

Codes include:

- `missing_games`
- `missing_rows`
- `missing_team_match`
- `missing_standings`
- `missing_pitching_matchups`
- `missing_broadcasts`
- `missing_odds`
- `missing_required_columns`
- `wrong_stat_category`
- `wrong_result_type`
- `wrong_league_for_intent`
- `retry_raw_scoreboard_probables`

### Validation Examples

| Validator | Checks |
|---|---|
| `ValidateGameRows` | scores/schedule rows exist, team filter matches, pregame participants exist, broadcasts exist if requested. |
| `ValidateStandingsRows` | standings rows exist, league supports standings shape, each row has team plus relevant standing signal. |
| `ValidateNewsRows` | rows exist and at least one headline exists. |
| `ValidateOddsRows` | rows exist and include betting fields. |
| `ValidateRosterRows` | rows exist and names are populated. |
| `ValidateLeaderboardRows` | rows exist and include athlete/value. |
| `ValidateGameDetailTable` | subtype-specific columns are present for officials/probabilities/etc. |
| `ValidateSimpleTable` | extended endpoint tables have expected shape and league compatibility. |
| `ValidateSearchEntities` | search results match requested scope. |

## Graceful Fallback Behavior

There are two different meanings of "fallback" in this feature:

1. **Routing fallback**: LLM router fails or declines; local detector or normal LLM handles the request.
2. **Data fallback**: ESPN endpoint is missing/empty; the sports client tries alternative ESPN paths or returns a clear empty-state Markdown answer.

### Routing Fallback

`tryRouterSportsLookup` returns:

```go
(*models.Message, skipLocalSports bool, *RouterTelemetry)
```

Behavior:

- If router returns valid `sports_lookup`, execute ESPN and return a message.
- If router returns `normal_llm` or `none`, set `skipLocalSports=true`; the local detector is not allowed to override the router.
- If router returns `clarify` and fallback behavior is `clarify`, return the clarifying question as assistant output.
- If router fails validation or cannot map params, return telemetry and allow local detector fallback.
- If no router service or sports disabled, no router path is used.

### Local Detector Fallback

If local detector succeeds:

```go
req, ok := sports.DetectSportsIntent(query, time.Now())
```

then the message handler calls:

```go
sports.NewESPNClient().Lookup(ctx, *req)
```

If local detector fails, the request continues to normal LLM/web/RAG processing.

### ESPN Data Fallbacks

Examples:

- Team news:
  1. `TeamNews`
  2. league site news with team param
  3. league news filtered by aliases

- Athlete news:
  1. athlete news endpoint
  2. search/news fallback

- Athlete stats:
  1. requested endpoint
  2. generic athlete stats/splits fallback

- MLB pitching matchups:
  1. normal scoreboard rows
  2. raw site scoreboard with `probables`
  3. validation error if probable pitcher data is unavailable

- Racing:
  1. scoreboard events
  2. calendar fallback rows
  3. most recent completed race resolution for "who won most recent race"

- Leaders:
  1. typed stat leader path
  2. team core leaders
  3. generic raw table fallback

### Empty-State Result Fallback

`ESPNClient.Lookup` converts certain errors into a successful `SportsLookupResult` with a Markdown table that explains no data is available.

Graceful errors include:

- `ErrNoGames`
- `ErrNoMatchingGames`
- `ErrNoStandings`
- `ErrNoNews`
- `ErrNoOdds`
- `ErrNoSportsData`
- `ErrTeamNotFound`
- `ErrAthleteNotFound`
- `ErrSportsResultMismatch`
- `ErrSportsResultMissingRequired`

The generated fallback result includes:

- title such as `No Scheduled Events Listed`, `No Articles Listed`, `No Betting Lines Listed`, or `Requested Details Not Available`,
- table with `Status` and `Detail`,
- user-facing explanation from `UserFacingError`.

### Upstream Error Mapping

`wrapESPNError` maps:

- context cancellation/deadline -> context error,
- `espn.ErrNotFound` -> `ErrNoSportsData`,
- `espn.ErrRateLimited` -> `ErrRateLimited`,
- HTTP status 400/404 strings -> `ErrNoSportsData`,
- other errors -> passthrough.

### User-Facing Error Messages

`UserFacingError(req, err)` translates backend sentinels into conversational messages.

Examples:

- Unsupported league:
  - "I can retrieve ESPN-backed scores, schedules, standings, news..."
- Malformed date:
  - "I could not understand that sports date. Please use today, tomorrow, yesterday, or YYYY-MM-DD."
- Missing pitching matchups:
  - "I found the MLB schedule, but ESPN did not provide probable pitchers for those games yet."
- Missing broadcasts:
  - "I found the games, but ESPN did not provide broadcast information for them."
- Historical stats too old:
  - "ESPN's statistics API does not have [league] data for the [season] season..."
- Rate limit:
  - "ESPN is rate limiting sports lookups right now. Please try again shortly."

## Markdown Rendering

Rendering lives in `backend/internal/sports/markdown.go`.

Render modes:

```go
const (
    SportsRenderPlainMarkdown    SportsRenderMode = "plain_markdown"
    SportsRenderEnhancedMarkdown SportsRenderMode = "enhanced_markdown"
    SportsRenderHTMLMarkdown     SportsRenderMode = "html_markdown"
)
```

Default:

```go
const DefaultSportsRenderMode = SportsRenderEnhancedMarkdown
```

Renderers include:

- `RenderGamesMarkdown`
- `RenderStandingsMarkdown`
- `RenderNewsMarkdown`
- `RenderOddsMarkdown`
- `RenderRosterMarkdown`
- `RenderLeaderboardMarkdown`
- `RenderSimpleMarkdown`

Rendering responsibilities:

- write a title and source line,
- include retrieved timestamp,
- format tables with aligned headers,
- render logos/headshots/images only after URL sanitization,
- use specialized table columns by intent and league,
- avoid unsafe Markdown/HTML injection by escaping table cells and sanitizing URLs.

Image and logo safety is explicit:

- `http://` ESPN CDN URLs are promoted to `https://`,
- protocol-relative URLs become `https://`,
- empty/no-host URLs are rejected,
- embedded newline URLs are rejected,
- `javascript:` URLs are rejected.

## Feature Flag and Tool Permissions

The direct sports lookup is gated by:

```go
const sportsLookupFeatureFlag = "sports_lookup_enabled"
```

The database migration seeds this flag enabled by default:

```sql
INSERT INTO feature_flags (name, enabled, description)
VALUES ('sports_lookup_enabled', 1, 'Retrieve current sports scores, schedules, and standings from ESPN')
ON CONFLICT(name) DO NOTHING;
```

If the feature flag repo is unavailable, sports lookup defaults to enabled.

Tool permissions are handled by the generic tool framework:

- `allow`: execute normally,
- `deny`: return a denied error,
- `ask`: require runtime approval handler.

The sports preflight path checks the feature flag but does not use generic tool permission policy because it is not an LLM tool call; it is a backend preflight capability.

## API Surface in the App

### Tool Management API

`ToolHandler` exposes:

- list tools with permission policy,
- update a tool permission,
- execute a tool manually.

Manual tool execution expects:

```json
{
  "name": "sports_lookup",
  "arguments": {
    "query": "NBA odds today",
    "intent": "odds",
    "league": "nba",
    "date": "today"
  }
}
```

If successful, the HTTP response is a `ToolResult`:

```json
{
  "tool_call_id": "manual-sports_lookup",
  "content": "{\"intent\":\"odds\",...}",
  "is_error": false,
  "metadata": {
    "tool": "sports_lookup",
    "intent": "odds",
    "league": "nba",
    "league_name": "NBA",
    "source": "ESPN public API via espn-go",
    "retrieved_at": "2026-05-17T18:45:00Z",
    "render_mode": "enhanced_markdown",
    "markdown": "..."
  }
}
```

### Chat Message API

For direct preflight sports answers, the assistant message has:

- `Role`: `assistant`,
- `Content`: rendered sports Markdown or user-facing error,
- `Provider`: `sports_lookup`,
- `Model`: `espn-go`,
- `LatencyMs`: lookup latency,
- `MetadataJSON` including:
  - `sports_lookup: true`,
  - `tool: sports_lookup`,
  - `source`,
  - `intent`,
  - `league`,
  - optional router telemetry,
  - `league_name`,
  - `league_logo_url`,
  - `retrieved_at`,
  - `render_mode`,
  - `error` when relevant.

## Implementation Blueprint for Another Project

### Minimal Package Layout

For a port, use this package structure:

```text
internal/sports/
  types.go              // request/result structs, intent constants, sentinel errors
  detector.go           // deterministic NL detector, league/team aliases, date parsing
  client.go             // ESPN client wrapper, dispatch, scores/standings/news
  odds.go               // odds from scoreboard
  advanced.go           // roster, leaders, athlete, generic, team record
  advanced_catalog.go   // teams, team history, seasons, tournaments, fantasy
  advanced_extended.go  // search, QBR, game detail, champions, draft, coaches
  advanced_extra.go     // venues, power index, recruits, bracketology
  validation.go         // result-shape validators
  markdown.go           // Markdown renderers and sanitizers
  tool.go               // generic tool wrapper
```

Router package:

```text
internal/router/
  types.go
  schema.go
  prompts.go
  validator.go
  service.go
  sports_mapper.go
```

Integration points:

```text
internal/api/message_handler.go  // preflight chat handling
internal/tools/                  // registry/executor interfaces
```

### Minimal Sports Tool Interface

```go
type Tool interface {
    Definition() ToolDefinition
    Validate(args json.RawMessage) error
    Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error)
}
```

`sports_lookup` should implement that interface and internally call:

```go
req, ok := sports.DetectSportsIntent(args.Query, now)
result, err := sports.NewESPNClient().Lookup(ctx, *req)
```

### Minimal Preflight Handler

```go
func handleSportsPreflight(ctx context.Context, userText string) (*AssistantMessage, bool) {
    if !sportsLookupEnabled() {
        return nil, false
    }

    // Optional: LLM router first.
    if routerEnabled() {
        resp := router.Route(ctx, userText)
        if resp.Valid && resp.Decision.Route == router.RouteSportsLookup {
            req, err := router.SportsRequestFromDecision(userText, resp.Decision, time.Now())
            if err == nil {
                return executeSports(ctx, req, resp.Telemetry), true
            }
        }
        if resp.Valid && resp.Decision.Route == router.RouteNormalLLM {
            return nil, false
        }
    }

    req, ok := sports.DetectSportsIntent(userText, time.Now())
    if !ok {
        return nil, false
    }
    return executeSports(ctx, req, nil), true
}
```

### Minimal Execute Function

```go
func executeSports(ctx context.Context, req *sports.SportsRequest, telemetry any) *AssistantMessage {
    start := time.Now()
    err := sports.ValidateDateInQuery(req.RawQuery, start)
    var result *sports.SportsLookupResult
    if err == nil {
        result, err = sports.NewESPNClient().Lookup(ctx, *req)
    }

    metadata := map[string]any{
        "sports_lookup": true,
        "tool": "sports_lookup",
        "source": sports.SourceESPN,
        "intent": req.Intent,
        "league": req.League,
        "router": telemetry,
    }

    content := ""
    if err != nil {
        content = sports.UserFacingError(*req, err)
        metadata["error"] = err.Error()
    } else {
        content = result.Markdown
        metadata["league_name"] = result.LeagueName
        metadata["league_logo_url"] = result.LeagueLogoURL
        metadata["retrieved_at"] = result.RetrievedAt.Format(time.RFC3339)
        metadata["render_mode"] = result.RenderMode
    }

    return &AssistantMessage{
        Content: content,
        Provider: "sports_lookup",
        Model: "espn-go",
        LatencyMS: int(time.Since(start).Milliseconds()),
        Metadata: metadata,
    }
}
```

## Test Strategy

The tests were built as a layered safety net. The goal is not only to test happy-path ESPN calls. The tests also lock down routing boundaries, normalization, validation, rendering, and graceful failure.

### Test Source Documents

Two planning/reference docs exist:

- `docs/internal_docs/espn_go_nl_question_test_set.md`
- `docs/internal_docs/espn_go_test_plan.md`

The 100-question set drives coverage across:

- intent classification,
- entity resolution,
- temporal parsing,
- endpoint routing,
- multi-call orchestration,
- graceful degradation,
- output formatting.

### Unit Test Categories

#### 1. Intent Detection

Files:

- `sports_test.go`
- `sports_extended_test.go`
- `sports_new_capabilities_test.go`
- `sports_phase2_test.go`
- `sports_q77_100_test.go`

Coverage:

- scores,
- schedule,
- standings,
- news,
- odds,
- rosters,
- injuries,
- transactions,
- team record,
- team schedule,
- leaders,
- athlete stats/news,
- rankings,
- league stats,
- search,
- QBR,
- athlete comparison,
- hot zones,
- game detail,
- champions,
- draft,
- coaches,
- venues,
- power index,
- recruits,
- bracketology,
- tournaments,
- fantasy,
- F1/NASCAR/PGA/ATP.

Example test style:

```go
got, ok := DetectSportsIntent("top 50 home run leaders for the 2025 MLB season", fixedNow())
if !ok {
    t.Fatal("expected sports lookup")
}
if got.Intent != SportsIntentLeaders || got.League != espn.LeagueMLB {
    t.Fatalf("intent/league = %s/%s", got.Intent, got.League)
}
if got.Season != 2025 || got.Limit != 50 {
    t.Fatalf("season/limit = %d/%d", got.Season, got.Limit)
}
if got.StatName != "homeRuns" {
    t.Fatalf("stat = %q", got.StatName)
}
```

#### 2. Negative Intent Cases

These verify that the sports tool does not capture normal LLM prompts.

Examples expected to return `ok=false`:

- "write a short story about baseball"
- "write a sports news article"
- "explain how betting odds work"
- "explain how MLB standings work"
- "who is the greatest baseball player ever"
- "make a sports logo"
- "compare football and baseball"
- "latest sports movies"
- "explain how batting average is calculated"

This is one of the most important test categories for app quality.

#### 3. Date Parsing

Tests cover:

- today,
- tonight,
- yesterday,
- tomorrow,
- last night,
- ISO date,
- slash date,
- this weekend,
- this week,
- Monday-Friday style weekdays,
- holiday rollover,
- malformed exact date detection.

The tests use a fixed clock:

```go
func fixedNow() time.Time {
    return time.Date(2026, 5, 7, 18, 45, 0, 0, time.Local)
}
```

Use fixed clocks in ports. Do not test relative date behavior against wall-clock time.

#### 4. League and Team Alias Coverage

Tests verify:

- all major league aliases,
- soccer league aliases,
- F1/NASCAR/PGA/ATP aliases,
- team detection across MLB/NFL/NBA/NHL/IPL/soccer,
- typo tolerance,
- no false positives in fuzzy matching.

#### 5. Normalization Tests

Normalization tests construct ESPN-like structs or JSON fixtures without live network calls.

Covered normalizers:

- scoreboard rows,
- linescore rows,
- broadcast names,
- standings rows,
- cricket standings,
- news feed,
- ESPN Now payloads,
- odds,
- roster,
- cricket roster,
- leaderboards,
- venue structs,
- search entities,
- champions,
- QBR table,
- power index,
- recruits,
- bracketology.

Example scoreboard normalization checks:

- home/away split,
- score extraction,
- venue extraction,
- status type,
- broadcast list,
- logo HTTPS promotion,
- team filtering.

#### 6. Validation Tests

Validation tests verify the app refuses mismatched results.

Examples:

- pitching matchup query fails when generic schedule rows have no `PitchingMatchup`,
- broadcasts subtype fails when no broadcast info exists,
- team query fails when returned rows do not match the requested team,
- soccer standings fail when rows lack standings signals,
- odds fail without betting fields,
- roster fails without names,
- leaderboard fails without athlete/value,
- game detail subtype rejects generic summary fallback,
- QBR requires NFL/CFB-compatible league,
- power index/recruits/draft/bracketology tables require expected shape.

#### 7. Markdown Rendering Tests

Rendering tests verify:

- Markdown escaping,
- HTML escaping,
- URL sanitization,
- enhanced score cards,
- schedule vs scores rendering,
- standings grouping,
- soccer/cricket standings columns,
- news images,
- unsafe news image/link rejection,
- odds tables,
- roster tables,
- leaderboard tables,
- fallback/simple tables.

#### 8. Router Tests

Router tests cover:

- valid JSON parsing,
- invalid JSON errors,
- low confidence validation,
- sports-only mode rejecting future/non-allowed routes,
- mapping decisions to `SportsRequest`,
- metric mapping from router output,
- bad date mapping to `ErrMalformedDate`,
- pitching matchup subtype inference.

These tests are essential because LLM output is untrusted even when structured output is enabled.

#### 9. Error Sentinel Tests

Tests verify:

- all sentinel errors are non-nil,
- sentinel errors are distinct,
- ESPN rate limit maps to `ErrRateLimited`,
- context cancellation is preserved,
- passthrough errors remain passthrough.

#### 10. Live Integration Tests

Live ESPN tests are behind integration build tags and/or env flags.

Examples:

```powershell
go test ./internal/sports/... -tags=integration -run TestIntegration_NBAScoreboard -count=1 -v
```

The 100-question live audit is enabled with:

```powershell
$env:ESPN_NL_AUDIT='1'
go test ./internal/sports/... -tags=integration -run TestIntegration_ESPNNLQuestionSetAudit -count=1 -v
```

Audit artifacts are written to:

```text
output/espn_nl_audit/espn_100_live_results.json
output/espn_nl_audit/espn_100_live_results.md
```

As of the existing audit note, 65/100 questions returned substantive live ESPN-backed responses, with the rest mostly returning empty-state ESPN responses. The strict audit counts empty-state responses as "needs review" rather than pass.

## PostgreSQL Historical ESPN Stats Warehouse

The existing ESPN integration can be extended into a local historical sports warehouse. The right model is not a single uncontrolled scrape of every ESPN endpoint. The better approach is a scoped, resumable, throttled backfill system that uses ESPN as the source of truth, stores normalized data in PostgreSQL, and also preserves raw ESPN payloads so the warehouse can be reprocessed as schemas improve.

For a major-sports export, assume one PostgreSQL database per sport:

| Sport | PostgreSQL database |
|---|---|
| MLB | `mlb_stats_db` |
| NFL | `nfl_stats_db` |
| NBA | `nba_stats_db` |
| NHL | `nhl_stats_db` |

Each database should use the same schema shape where possible. Keeping schema names and table names consistent lets the application share ingestion code and query builders across sports while still physically isolating each sport's data volume, refresh cadence, and sport-specific extensions.

### Why PostgreSQL Is The Right Store

PostgreSQL is a good fit because the data has both relational and semi-structured characteristics:

- leagues, seasons, teams, athletes, events, competitions, and stats are relational;
- ESPN payloads differ by endpoint and season, so raw JSON should also be retained;
- historical queries need indexes, joins, grouping, ranking, and aggregation;
- current/live data needs upsert semantics and freshness metadata;
- JSONB can preserve endpoint-specific fields without blocking ingestion when ESPN changes response shape.

The recommended design is:

1. **Normalized tables** for common query paths.
2. **Raw JSONB payload archive** for replay, debugging, and future extraction.
3. **Sync-run tables** for resumability, auditing, rate limit handling, and error recovery.
4. **Materialized views** for common leaderboard and season-summary questions.

### High-Level Architecture

Add a warehouse layer beside the current sports client:

```text
backend/internal/sports/
  warehouse/
    config.go           // PostgreSQL connection settings per sport
    store.go            // normalized read/write repository
    raw_store.go        // raw JSONB payload storage
    sync.go             // backfill orchestration
    jobs.go             // job definitions by endpoint/season
    checkpoints.go      // resumable sync state
    queries.go          // local historical query helpers
```

The request flow becomes:

```text
User question
  -> router/local detector
  -> SportsRequest
  -> local warehouse read path for historical/static data
  -> ESPN live lookup when local data is missing/stale/current-live
  -> persist ESPN response to PostgreSQL
  -> normalize/render Markdown
```

The app should not replace ESPN lookups entirely. Instead:

- historical completed data should come from PostgreSQL first;
- live/current games should use short-lived cache rows and refresh from ESPN;
- missing local rows should trigger ESPN fetch and write-through;
- ESPN failures can serve stale PostgreSQL data with explicit metadata.

### Database Strategy

Use four databases:

```text
mlb_stats_db
nfl_stats_db
nba_stats_db
nhl_stats_db
```

Each database should contain:

- shared canonical tables,
- sport-specific stat tables where needed,
- raw payload table,
- sync metadata tables.

Recommended PostgreSQL extensions:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gin;
```

`pgcrypto` provides `gen_random_uuid()`. `btree_gin` helps with mixed JSONB and scalar indexing patterns if needed.

### Shared Canonical Schema

The following tables should exist in each of `mlb_stats_db`, `nfl_stats_db`, `nba_stats_db`, and `nhl_stats_db`.

#### `sports_leagues`

Stores ESPN sport/league identity.

```sql
CREATE TABLE sports_leagues (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source TEXT NOT NULL DEFAULT 'espn',
  sport TEXT NOT NULL,
  league TEXT NOT NULL,
  display_name TEXT NOT NULL,
  abbreviation TEXT,
  logo_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (source, sport, league)
);
```

Examples:

- MLB: `sport=baseball`, `league=mlb`
- NFL: `sport=football`, `league=nfl`
- NBA: `sport=basketball`, `league=nba`
- NHL: `sport=hockey`, `league=nhl`

#### `sports_seasons`

Stores season metadata.

```sql
CREATE TABLE sports_seasons (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  season INTEGER NOT NULL,
  display_name TEXT,
  start_date DATE,
  end_date DATE,
  is_current BOOLEAN NOT NULL DEFAULT false,
  status TEXT,
  raw_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, season)
);
```

#### `sports_teams`

Stores teams as ESPN identities.

```sql
CREATE TABLE sports_teams (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  espn_team_id TEXT NOT NULL,
  slug TEXT,
  display_name TEXT NOT NULL,
  short_name TEXT,
  abbreviation TEXT,
  location TEXT,
  nickname TEXT,
  color TEXT,
  alternate_color TEXT,
  logo_url TEXT,
  dark_logo_url TEXT,
  active BOOLEAN NOT NULL DEFAULT true,
  first_seen_season INTEGER,
  last_seen_season INTEGER,
  raw_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, espn_team_id)
);

CREATE INDEX idx_sports_teams_abbr ON sports_teams (league_id, abbreviation);
CREATE INDEX idx_sports_teams_name_trgm_like ON sports_teams (league_id, lower(display_name));
```

#### `sports_venues`

Stores stadiums, arenas, and ballparks.

```sql
CREATE TABLE sports_venues (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID REFERENCES sports_leagues(id),
  espn_venue_id TEXT,
  full_name TEXT NOT NULL,
  short_name TEXT,
  city TEXT,
  state TEXT,
  country TEXT,
  indoor BOOLEAN,
  capacity INTEGER,
  grass BOOLEAN,
  raw_ref TEXT,
  raw_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, espn_venue_id)
);
```

#### `sports_athletes`

Stores athlete identity independent of roster membership.

```sql
CREATE TABLE sports_athletes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  espn_athlete_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  full_name TEXT,
  first_name TEXT,
  last_name TEXT,
  short_name TEXT,
  position TEXT,
  position_abbr TEXT,
  jersey TEXT,
  height TEXT,
  weight TEXT,
  age INTEGER,
  birth_date DATE,
  birth_place TEXT,
  headshot_url TEXT,
  active BOOLEAN,
  raw_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, espn_athlete_id)
);

CREATE INDEX idx_sports_athletes_name ON sports_athletes (league_id, lower(display_name));
```

#### `sports_events`

Stores ESPN events/games.

```sql
CREATE TABLE sports_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  season_id UUID REFERENCES sports_seasons(id),
  espn_event_id TEXT NOT NULL,
  uid TEXT,
  guid TEXT,
  name TEXT,
  short_name TEXT,
  event_date TIMESTAMPTZ,
  event_date_local DATE,
  season INTEGER,
  season_type INTEGER,
  week INTEGER,
  status_state TEXT,
  status_name TEXT,
  status_detail TEXT,
  status_short_detail TEXT,
  completed BOOLEAN NOT NULL DEFAULT false,
  neutral_site BOOLEAN,
  conference_competition BOOLEAN,
  venue_id UUID REFERENCES sports_venues(id),
  raw_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, espn_event_id)
);

CREATE INDEX idx_sports_events_season_date ON sports_events (league_id, season, event_date);
CREATE INDEX idx_sports_events_completed ON sports_events (league_id, completed, event_date);
```

#### `sports_competitions`

ESPN events can contain competitions. Most major US sports have one competition per event, but storing this separately mirrors ESPN's shape.

```sql
CREATE TABLE sports_competitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id UUID NOT NULL REFERENCES sports_events(id) ON DELETE CASCADE,
  espn_competition_id TEXT NOT NULL,
  date TIMESTAMPTZ,
  attendance INTEGER,
  time_valid BOOLEAN,
  neutral_site BOOLEAN,
  division_competition BOOLEAN,
  conference_competition BOOLEAN,
  boxscore_available BOOLEAN,
  play_by_play_available BOOLEAN,
  status_state TEXT,
  status_detail TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (event_id, espn_competition_id)
);
```

#### `sports_competitors`

Stores home/away competitors and final/live score fields.

```sql
CREATE TABLE sports_competitors (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  competition_id UUID NOT NULL REFERENCES sports_competitions(id) ON DELETE CASCADE,
  team_id UUID REFERENCES sports_teams(id),
  espn_team_id TEXT,
  home_away TEXT NOT NULL CHECK (home_away IN ('home', 'away', 'unknown')),
  winner BOOLEAN,
  score INTEGER,
  score_display TEXT,
  record_summary TEXT,
  rank INTEGER,
  curated_rank INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (competition_id, home_away)
);

CREATE INDEX idx_sports_competitors_team ON sports_competitors (team_id);
```

#### `sports_linescores`

Stores period/quarter/inning scores.

```sql
CREATE TABLE sports_linescores (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  competitor_id UUID NOT NULL REFERENCES sports_competitors(id) ON DELETE CASCADE,
  period INTEGER NOT NULL,
  display_value TEXT,
  value NUMERIC,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (competitor_id, period)
);
```

Sport-specific interpretation:

- MLB: inning.
- NFL/NBA: quarter or overtime period.
- NHL: period or overtime/shootout representation as ESPN exposes it.

#### `sports_standings`

Stores normalized standings rows by season and group.

```sql
CREATE TABLE sports_standings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  season_id UUID REFERENCES sports_seasons(id),
  season INTEGER NOT NULL,
  season_type INTEGER,
  group_name TEXT,
  conference TEXT,
  division TEXT,
  team_id UUID REFERENCES sports_teams(id),
  rank INTEGER,
  wins NUMERIC,
  losses NUMERIC,
  ties NUMERIC,
  overtime_losses NUMERIC,
  pct NUMERIC,
  games_back TEXT,
  streak TEXT,
  last_ten TEXT,
  points NUMERIC,
  games_played NUMERIC,
  goal_differential NUMERIC,
  note TEXT,
  raw_stats JSONB,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, season, season_type, group_name, team_id)
);

CREATE INDEX idx_sports_standings_team_season ON sports_standings (team_id, season);
CREATE INDEX idx_sports_standings_rank ON sports_standings (league_id, season, group_name, rank);
```

#### `sports_rosters`

Stores season/team roster membership.

```sql
CREATE TABLE sports_rosters (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  season_id UUID REFERENCES sports_seasons(id),
  season INTEGER,
  team_id UUID NOT NULL REFERENCES sports_teams(id),
  athlete_id UUID REFERENCES sports_athletes(id),
  espn_athlete_id TEXT,
  group_name TEXT,
  position TEXT,
  jersey TEXT,
  status TEXT,
  depth_order INTEGER,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, season, espn_athlete_id)
);

CREATE INDEX idx_sports_rosters_team_season ON sports_rosters (team_id, season);
CREATE INDEX idx_sports_rosters_athlete ON sports_rosters (athlete_id);
```

#### `sports_leaders`

Stores normalized leaderboard results.

```sql
CREATE TABLE sports_leaders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  season_id UUID REFERENCES sports_seasons(id),
  season INTEGER,
  season_type INTEGER,
  team_id UUID REFERENCES sports_teams(id),
  athlete_id UUID REFERENCES sports_athletes(id),
  espn_athlete_id TEXT,
  stat_category TEXT NOT NULL,
  stat_name TEXT NOT NULL,
  stat_label TEXT,
  rank INTEGER,
  value NUMERIC,
  display_value TEXT,
  athlete_name TEXT,
  team_abbreviation TEXT,
  position TEXT,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  raw_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, season, season_type, team_id, stat_category, stat_name, espn_athlete_id)
);

CREATE INDEX idx_sports_leaders_stat ON sports_leaders (league_id, season, stat_category, stat_name, rank);
CREATE INDEX idx_sports_leaders_athlete ON sports_leaders (athlete_id, season);
```

#### `sports_athlete_stats`

Stores player stats in flexible JSONB plus common scalar fields.

```sql
CREATE TABLE sports_athlete_stats (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  season_id UUID REFERENCES sports_seasons(id),
  season INTEGER,
  season_type INTEGER,
  athlete_id UUID REFERENCES sports_athletes(id),
  team_id UUID REFERENCES sports_teams(id),
  espn_athlete_id TEXT NOT NULL,
  stat_scope TEXT NOT NULL, -- season, game_log, split, career, bio, qbr
  stat_category TEXT,
  stats JSONB NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, season, season_type, espn_athlete_id, stat_scope, stat_category)
);

CREATE INDEX idx_sports_athlete_stats_lookup ON sports_athlete_stats (league_id, season, espn_athlete_id, stat_scope);
CREATE INDEX idx_sports_athlete_stats_json ON sports_athlete_stats USING GIN (stats);
```

#### `sports_game_stats`

Stores box score/team/player stats by event.

```sql
CREATE TABLE sports_game_stats (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  event_id UUID NOT NULL REFERENCES sports_events(id) ON DELETE CASCADE,
  competition_id UUID REFERENCES sports_competitions(id) ON DELETE CASCADE,
  team_id UUID REFERENCES sports_teams(id),
  athlete_id UUID REFERENCES sports_athletes(id),
  espn_athlete_id TEXT,
  stat_subject TEXT NOT NULL, -- team, athlete
  stat_category TEXT,
  stats JSONB NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sports_game_stats_event ON sports_game_stats (event_id, stat_subject);
CREATE INDEX idx_sports_game_stats_team ON sports_game_stats (team_id);
CREATE INDEX idx_sports_game_stats_athlete ON sports_game_stats (athlete_id);
CREATE INDEX idx_sports_game_stats_json ON sports_game_stats USING GIN (stats);
```

#### `sports_odds`

Stores odds snapshots. Even historical odds can change before game start, so preserve `fetched_at`.

```sql
CREATE TABLE sports_odds (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID NOT NULL REFERENCES sports_leagues(id),
  event_id UUID REFERENCES sports_events(id),
  competition_id UUID REFERENCES sports_competitions(id),
  provider TEXT,
  away_team_id UUID REFERENCES sports_teams(id),
  home_team_id UUID REFERENCES sports_teams(id),
  away_moneyline INTEGER,
  home_moneyline INTEGER,
  spread NUMERIC,
  spread_display TEXT,
  over_under NUMERIC,
  over_under_display TEXT,
  details TEXT,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  raw_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sports_odds_event_time ON sports_odds (event_id, fetched_at DESC);
```

#### `sports_news`

News archival is optional. If included, avoid assuming ESPN news is complete historically.

```sql
CREATE TABLE sports_news (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  league_id UUID REFERENCES sports_leagues(id),
  team_id UUID REFERENCES sports_teams(id),
  athlete_id UUID REFERENCES sports_athletes(id),
  espn_article_id TEXT,
  headline TEXT NOT NULL,
  description TEXT,
  byline TEXT,
  url TEXT,
  image_url TEXT,
  published_at TIMESTAMPTZ,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  raw_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (league_id, url)
);

CREATE INDEX idx_sports_news_published ON sports_news (league_id, published_at DESC);
```

### Raw Payload Archive

Every ESPN API response used by a sync job should be stored in `sports_raw_payloads`. This makes the warehouse replayable.

```sql
CREATE TABLE sports_raw_payloads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source TEXT NOT NULL DEFAULT 'espn',
  sport TEXT NOT NULL,
  league TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  cache_key TEXT NOT NULL UNIQUE,
  request_url TEXT,
  request_params JSONB,
  season INTEGER,
  season_type INTEGER,
  week INTEGER,
  event_id TEXT,
  competition_id TEXT,
  team_id TEXT,
  athlete_id TEXT,
  payload_json JSONB NOT NULL,
  payload_hash TEXT NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ,
  status_code INTEGER,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_raw_payloads_endpoint ON sports_raw_payloads (league, endpoint, season, fetched_at DESC);
CREATE INDEX idx_raw_payloads_event ON sports_raw_payloads (league, event_id);
CREATE INDEX idx_raw_payloads_team ON sports_raw_payloads (league, team_id);
CREATE INDEX idx_raw_payloads_athlete ON sports_raw_payloads (league, athlete_id);
CREATE INDEX idx_raw_payloads_json ON sports_raw_payloads USING GIN (payload_json);
```

Use `payload_hash` to detect changes and avoid reprocessing unchanged payloads:

```text
payload_hash = sha256(canonical_json(payload_json))
```

### Sync Metadata Tables

#### `sports_sync_runs`

Tracks each backfill or refresh run.

```sql
CREATE TABLE sports_sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source TEXT NOT NULL DEFAULT 'espn',
  sport TEXT NOT NULL,
  league TEXT NOT NULL,
  job_name TEXT NOT NULL,
  status TEXT NOT NULL, -- running, completed, failed, canceled
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ,
  requested_start_season INTEGER,
  requested_end_season INTEGER,
  fetched_count INTEGER NOT NULL DEFAULT 0,
  inserted_count INTEGER NOT NULL DEFAULT 0,
  updated_count INTEGER NOT NULL DEFAULT 0,
  skipped_count INTEGER NOT NULL DEFAULT 0,
  error_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  metadata JSONB
);

CREATE INDEX idx_sync_runs_job ON sports_sync_runs (league, job_name, started_at DESC);
```

#### `sports_sync_checkpoints`

Allows jobs to resume safely.

```sql
CREATE TABLE sports_sync_checkpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sport TEXT NOT NULL,
  league TEXT NOT NULL,
  job_name TEXT NOT NULL,
  checkpoint_key TEXT NOT NULL,
  checkpoint_value JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (sport, league, job_name, checkpoint_key)
);
```

Example checkpoint values:

```json
{
  "season": 2021,
  "date": "2021-08-14",
  "event_id": "401326315",
  "page": 2
}
```

### PostgreSQL Connection Configuration

Use one connection pool per database.

Example environment variables:

```text
ESPN_MLB_DATABASE_URL=postgres://user:pass@localhost:5432/mlb_stats_db?sslmode=disable
ESPN_NFL_DATABASE_URL=postgres://user:pass@localhost:5432/nfl_stats_db?sslmode=disable
ESPN_NBA_DATABASE_URL=postgres://user:pass@localhost:5432/nba_stats_db?sslmode=disable
ESPN_NHL_DATABASE_URL=postgres://user:pass@localhost:5432/nhl_stats_db?sslmode=disable
```

Sport-to-database mapping:

```go
var ESPNWarehouseDatabases = map[string]string{
    "mlb": "mlb_stats_db",
    "nfl": "nfl_stats_db",
    "nba": "nba_stats_db",
    "nhl": "nhl_stats_db",
}
```

In Go, keep the app-level mapping based on ESPN league constants:

```go
func databaseForLeague(league string) string {
    switch league {
    case espn.LeagueMLB:
        return "mlb_stats_db"
    case espn.LeagueNFL:
        return "nfl_stats_db"
    case espn.LeagueNBA:
        return "nba_stats_db"
    case espn.LeagueNHL:
        return "nhl_stats_db"
    default:
        return ""
    }
}
```

### Backfill Scope By Sport

Do not assume every ESPN endpoint has complete historical depth. Start with endpoints that are most stable and valuable.

#### MLB: `mlb_stats_db`

Recommended backfill:

- seasons,
- teams by season,
- schedule/scoreboard by date or season,
- event summaries for completed games,
- standings by season,
- rosters by team/season where ESPN exposes them,
- stat leaders by season for common batting/pitching metrics:
  - home runs,
  - RBI,
  - hits,
  - stolen bases,
  - batting average,
  - ERA,
  - WHIP,
  - strikeouts,
  - saves,
- athlete stats for players discovered from rosters/leaders,
- venues.

#### NFL: `nfl_stats_db`

Recommended backfill:

- seasons,
- teams by season,
- schedule by season/week,
- scoreboard/event summaries,
- standings by season,
- rosters by team/season,
- leaders:
  - passing yards,
  - passing touchdowns,
  - rushing yards,
  - rushing touchdowns,
  - receiving yards,
  - receiving touchdowns,
  - receptions,
  - sacks,
  - interceptions,
- QBR by season,
- draft by season,
- coaches if needed,
- venues.

#### NBA: `nba_stats_db`

Recommended backfill:

- seasons,
- teams by season,
- schedule/scoreboard,
- event summaries and box scores,
- standings by season,
- rosters,
- leaders:
  - points per game,
  - rebounds,
  - assists,
  - steals,
  - blocks,
- athlete season stats,
- draft by season,
- venues.

#### NHL: `nhl_stats_db`

Recommended backfill:

- seasons,
- teams by season,
- schedule/scoreboard,
- event summaries,
- standings by season,
- rosters,
- leaders:
  - goals,
  - assists,
  - points,
  - saves if goalie data is exposed,
- athlete season stats,
- venues.

### Backfill Job Design

Use small, resumable jobs instead of one giant import.

Recommended job names:

```text
seed_league
backfill_seasons
backfill_teams
backfill_schedule
backfill_event_summaries
backfill_standings
backfill_rosters
backfill_leaders
backfill_athletes
backfill_venues
refresh_current_scoreboard
refresh_current_standings
refresh_current_odds
```

Each job should:

1. Create a `sports_sync_runs` row.
2. Load its latest checkpoint.
3. Fetch one bounded batch from ESPN.
4. Store raw payload.
5. Normalize into canonical tables.
6. Upsert rows idempotently.
7. Update checkpoint.
8. Sleep/throttle.
9. Mark run complete or failed.

### Backfill Order

For each database:

```text
1. seed_league
2. backfill_seasons
3. backfill_teams
4. backfill_venues
5. backfill_schedule
6. backfill_event_summaries
7. backfill_standings
8. backfill_rosters
9. backfill_leaders
10. backfill_athletes
11. create/refresh materialized views
```

Teams should be loaded before events so `sports_competitors.team_id` can reference canonical team rows. Athletes can be loaded from rosters, leaders, box scores, search results, and athlete stat endpoints.

### Example Sync Job Skeleton

```go
type WarehouseSyncJob struct {
    Sport      string
    League     string
    DBName     string
    JobName    string
    StartYear  int
    EndYear    int
    ESPNClient *sports.ESPNClient
    Store      *WarehouseStore
    Throttle   time.Duration
}

func (j *WarehouseSyncJob) BackfillStandings(ctx context.Context) error {
    runID, err := j.Store.StartRun(ctx, j.Sport, j.League, j.JobName, j.StartYear, j.EndYear)
    if err != nil {
        return err
    }
    defer j.Store.FinishRun(ctx, runID)

    for season := j.StartYear; season <= j.EndYear; season++ {
        if err := ctx.Err(); err != nil {
            return err
        }

        req := sports.SportsRequest{
            Intent: sports.SportsIntentStandings,
            Sport:  j.Sport,
            League: j.League,
            Season: season,
            Limit:  100,
        }

        result, err := j.ESPNClient.Lookup(ctx, req)
        if err != nil {
            j.Store.RecordRunError(ctx, runID, season, err)
            continue
        }

        rawKey := fmt.Sprintf("%s:%s:standings:%d", j.Sport, j.League, season)
        _ = j.Store.SaveRenderedResult(ctx, rawKey, req, result)

        // Prefer storing raw endpoint payloads inside lookup methods or a lower-level fetch wrapper.
        // Then normalize from raw JSON into sports_standings.

        j.Store.SaveCheckpoint(ctx, j.Sport, j.League, j.JobName, "season", map[string]any{
            "season": season,
        })

        time.Sleep(j.Throttle)
    }
    return nil
}
```

For a production implementation, prefer backfilling from lower-level raw ESPN calls rather than only from rendered `SportsLookupResult`, because rendered Markdown loses structure.

### Write-Through Cache From Existing Lookup

The easiest first step is a write-through cache at `ESPNClient.Lookup`.

Pseudo-flow:

```go
func (c *ESPNClient) Lookup(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
    if c.warehouse != nil && shouldReadWarehouse(req) {
        if result, hit := c.warehouse.ReadLookupResult(ctx, req); hit {
            result.Source = SourceESPN + " (local PostgreSQL cache)"
            return result, nil
        }
    }

    result, err := c.lookup(ctx, req)
    if err != nil {
        if c.warehouse != nil {
            if stale, hit := c.warehouse.ReadStaleLookupResult(ctx, req); hit {
                stale.Source = SourceESPN + " (stale local PostgreSQL cache)"
                return stale, nil
            }
        }
        if isGracefulLookupError(err) {
            return c.emptyLookupResult(req, err), nil
        }
        return nil, err
    }

    if c.warehouse != nil && shouldWriteWarehouse(req, result) {
        _ = c.warehouse.WriteLookupResult(ctx, req, result)
    }
    return result, nil
}
```

This is simple, but it stores rendered results. The better long-term design is to cache raw payloads and normalized rows below the renderer.

### Cache Key Strategy

Every warehouse lookup and raw payload should use a deterministic cache key.

Recommended key fields:

- source: `espn`
- sport
- league
- endpoint
- intent
- season
- season type
- week
- date
- team ID or normalized team query
- athlete ID or normalized athlete query
- event ID
- stat category/name/sort
- limit
- subtype

Example:

```go
type ESPNCacheKey struct {
    Source       string `json:"source"`
    Sport        string `json:"sport"`
    League       string `json:"league"`
    Endpoint     string `json:"endpoint"`
    Intent       string `json:"intent"`
    Season       int    `json:"season,omitempty"`
    SeasonType   int    `json:"season_type,omitempty"`
    Week         int    `json:"week,omitempty"`
    Date         string `json:"date,omitempty"`
    TeamID       string `json:"team_id,omitempty"`
    AthleteID    string `json:"athlete_id,omitempty"`
    EventID      string `json:"event_id,omitempty"`
    StatCategory string `json:"stat_category,omitempty"`
    StatName     string `json:"stat_name,omitempty"`
    StatSort     string `json:"stat_sort,omitempty"`
    Limit        int    `json:"limit,omitempty"`
    Subtype      string `json:"subtype,omitempty"`
}
```

Canonicalize it to stable JSON, then hash:

```go
cacheKey := sha256Hex(canonicalJSON(key))
```

### TTL and Freshness Policy

PostgreSQL rows should distinguish historical permanence from current/live volatility.

Recommended TTLs:

| Data type | TTL |
|---|---:|
| Live scores | 15-30 seconds |
| Current day's schedule | 1-5 minutes |
| Future schedule | 15-60 minutes |
| Completed historical games | effectively permanent |
| Historical standings for completed seasons | effectively permanent |
| Current standings | 15-60 minutes |
| Rosters | 12-24 hours during season |
| Historical rosters | effectively permanent once season is old |
| Teams/venues/seasons | 24 hours to permanent |
| News | 2-5 minutes, optional archival |
| Odds | 15-60 seconds before game start |
| Draft/champions | permanent |
| Athlete historical stats | permanent or multi-day |

Add freshness metadata either per table or in `sports_raw_payloads.expires_at`.

For normalized tables, use `fetched_at` and infer staleness by data type.

### Local Query Examples

#### MLB home run leaders from `mlb_stats_db`

```sql
SELECT
  rank,
  athlete_name,
  team_abbreviation,
  display_value
FROM sports_leaders
WHERE season = 2025
  AND stat_category = 'batting'
  AND stat_name = 'homeRuns'
ORDER BY rank
LIMIT 50;
```

#### NFL passing leaders from `nfl_stats_db`

```sql
SELECT
  rank,
  athlete_name,
  team_abbreviation,
  display_value
FROM sports_leaders
WHERE season = 2025
  AND stat_category = 'passing'
  AND stat_name = 'passingYards'
ORDER BY rank
LIMIT 25;
```

#### NBA current standings from `nba_stats_db`

```sql
SELECT
  s.group_name,
  s.rank,
  t.display_name,
  s.wins,
  s.losses,
  s.pct,
  s.games_back,
  s.streak
FROM sports_standings s
JOIN sports_teams t ON t.id = s.team_id
WHERE s.season = 2025
ORDER BY s.group_name, s.rank;
```

#### NHL team schedule from `nhl_stats_db`

```sql
SELECT
  e.event_date,
  e.status_detail,
  away.display_name AS away_team,
  away_comp.score AS away_score,
  home.display_name AS home_team,
  home_comp.score AS home_score
FROM sports_events e
JOIN sports_competitions c ON c.event_id = e.id
JOIN sports_competitors away_comp ON away_comp.competition_id = c.id AND away_comp.home_away = 'away'
JOIN sports_competitors home_comp ON home_comp.competition_id = c.id AND home_comp.home_away = 'home'
JOIN sports_teams away ON away.id = away_comp.team_id
JOIN sports_teams home ON home.id = home_comp.team_id
WHERE e.season = 2025
  AND (
    away.abbreviation = 'BOS'
    OR home.abbreviation = 'BOS'
  )
ORDER BY e.event_date;
```

### Materialized Views

For common historical questions, create materialized views.

Example season team records:

```sql
CREATE MATERIALIZED VIEW mv_team_season_records AS
SELECT
  l.league,
  s.season,
  t.espn_team_id,
  t.display_name,
  st.group_name,
  st.rank,
  st.wins,
  st.losses,
  st.ties,
  st.overtime_losses,
  st.pct,
  st.points,
  st.streak
FROM sports_standings st
JOIN sports_leagues l ON l.id = st.league_id
JOIN sports_teams t ON t.id = st.team_id
JOIN sports_seasons s ON s.id = st.season_id;

CREATE INDEX idx_mv_team_season_records_lookup
ON mv_team_season_records (league, season, espn_team_id);
```

Refresh after sync jobs:

```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_team_season_records;
```

For concurrent refresh, add a unique index on the materialized view.

### Integration With Question Answering

The local warehouse should become a retrieval source inside `ESPNClient`, not a separate user-visible tool at first.

Recommended decision rules:

```go
func shouldReadWarehouse(req sports.SportsRequest) bool {
    if req.League is not MLB/NFL/NBA/NHL {
        return false
    }
    if req.Intent is News or Odds and current/future {
        return false // use live ESPN unless fresh local row exists
    }
    if req.Season > 0 && req.Season < currentSeason(req.League) {
        return true
    }
    if req.Date != nil && date is before today {
        return true
    }
    if req.Intent is Teams, Venues, Seasons, Champions, Draft {
        return true
    }
    return false
}
```

For current/live queries:

```go
func shouldUseFreshCache(req sports.SportsRequest) bool {
    return req.Intent == Scores || req.Intent == Schedule || req.Intent == Standings || req.Intent == Odds
}
```

Response metadata should disclose cache usage:

```json
{
  "sports_lookup": true,
  "source": "ESPN public API via espn-go",
  "warehouse": {
    "database": "mlb_stats_db",
    "cache_hit": true,
    "stale": false,
    "fetched_at": "2026-05-17T20:00:00Z",
    "cache_key": "..."
  }
}
```

### Recommended CLI Commands

Add a backend command for warehouse sync:

```text
backend/cmd/sportssync/main.go
```

Example usage:

```powershell
go run ./cmd/sportssync --sport mlb --db mlb_stats_db --job backfill_schedule --start-season 2002 --end-season 2026
go run ./cmd/sportssync --sport nfl --db nfl_stats_db --job backfill_leaders --start-season 2002 --end-season 2026
go run ./cmd/sportssync --sport nba --db nba_stats_db --job backfill_standings --start-season 2002 --end-season 2026
go run ./cmd/sportssync --sport nhl --db nhl_stats_db --job backfill_rosters --start-season 2002 --end-season 2026
```

Include a dry-run mode:

```powershell
go run ./cmd/sportssync --sport mlb --db mlb_stats_db --job backfill_schedule --start-season 2025 --end-season 2025 --dry-run
```

Include a resume mode:

```powershell
go run ./cmd/sportssync --sport mlb --db mlb_stats_db --job backfill_schedule --resume
```

### Rate Limiting and Operational Safety

Historical backfill must be polite and resumable.

Recommended controls:

- global ESPN request rate limit,
- per-job concurrency limit,
- exponential backoff on HTTP 429/5xx,
- checkpoint after every season/date/team/event batch,
- skip unchanged payload hashes,
- stop after repeated rate limit responses,
- log every failed endpoint with request params,
- store partial raw payloads only when valid JSON is returned,
- run large backfills outside normal chat request handling.

Conservative starting limits:

```text
1-2 concurrent ESPN requests per sport
250-1000 ms delay between requests
stop job for 10-30 minutes after repeated 429 responses
```

### Data Completeness Caveats

Do not promise complete historical data until audited endpoint by endpoint.

Known risks:

- ESPN historical depth varies by endpoint, league, and season.
- Some endpoints return 400/404 for older seasons.
- Some current endpoints expose rich data that older endpoints do not.
- Team IDs, franchise moves, abbreviations, and league structures change over time.
- Rosters and athlete stats may not be complete for older seasons.
- Odds and news should not be treated as complete historical archives.

The warehouse should track missing data explicitly rather than hiding it. For example, `sports_sync_runs.error_count`, `sports_raw_payloads.error`, and empty-state rows can distinguish "not fetched yet" from "ESPN does not expose this."

### Warehouse Testing Strategy

Add tests separate from live ESPN integration:

1. **Schema migration tests**
   - create all tables in an ephemeral PostgreSQL database,
   - verify indexes and constraints.

2. **Repository tests**
   - upsert league/team/event rows idempotently,
   - verify duplicate ESPN IDs do not create duplicate rows,
   - verify updates change mutable fields.

3. **Raw payload tests**
   - canonical cache key generation,
   - payload hash stability,
   - JSONB insert/read round trip.

4. **Normalizer replay tests**
   - load saved ESPN fixture JSON,
   - normalize into canonical tables,
   - assert expected rows.

5. **Checkpoint tests**
   - save checkpoint,
   - resume job,
   - verify already-processed batches are skipped.

6. **Warehouse query tests**
   - standings lookup from SQL,
   - team schedule lookup from SQL,
   - leaderboard lookup from SQL,
   - stale cache fallback.

7. **Live smoke tests**
   - one current-season fetch per sport,
   - one historical-season fetch per sport,
   - verify raw payload plus normalized rows are written.

### Phased Implementation Plan

Phase 1: Raw payload archive

- Add PostgreSQL connection mapping for four databases.
- Add `sports_raw_payloads`, `sports_sync_runs`, and `sports_sync_checkpoints`.
- Wrap ESPN raw/typed calls so every response can be persisted as JSONB.
- Add a CLI with dry-run and one simple job.

Phase 2: Canonical schedule/teams/events

- Add leagues, seasons, teams, venues, events, competitions, competitors, linescores.
- Backfill schedules for one recent season per sport.
- Add local SQL read path for completed historical scores/schedules.

Phase 3: Standings and leaders

- Add standings and leaders tables.
- Backfill 5-10 seasons per sport.
- Add local SQL read path for standings and leaderboard questions.

Phase 4: Rosters and athletes

- Add athletes, rosters, athlete stats.
- Resolve athlete IDs from rosters/leaders.
- Backfill current and recent historical rosters.

Phase 5: Summaries and game stats

- Add event summary/box score ingestion.
- Populate `sports_game_stats`.
- Support richer historical game detail questions locally.

Phase 6: Production refresh

- Add scheduled jobs:
  - live scoreboard refresh,
  - current standings refresh,
  - current leaders refresh,
  - daily roster refresh,
  - periodic historical repair jobs.
- Add warehouse metadata to chat responses.

## PostgreSQL Warehouse Porting Checklist

1. Create `mlb_stats_db`, `nfl_stats_db`, `nba_stats_db`, and `nhl_stats_db`.
2. Apply identical base migrations to all four databases.
3. Seed `sports_leagues` in each database with the correct ESPN sport/league pair.
4. Implement per-sport connection pool selection.
5. Add raw JSONB payload storage before normalizing anything.
6. Add sync run and checkpoint tables.
7. Build one job at a time; start with seasons, teams, and schedules.
8. Add idempotent upsert helpers for all canonical tables.
9. Normalize from raw ESPN payloads into canonical rows.
10. Add local warehouse read paths for historical completed queries.
11. Add stale-cache fallback for ESPN failures.
12. Add cache metadata to assistant/tool responses.
13. Add materialized views only after base tables are stable.
14. Run small live smoke tests before large backfills.
15. Keep backfills throttled, resumable, and auditable.

## Porting Checklist

1. Add `espn-go` dependency.
2. Copy or recreate `SportsRequest`, `SportsLookupResult`, intent constants, render modes, and sentinel errors.
3. Implement league configs and team aliases for your target leagues.
4. Implement deterministic detector with non-lookup guardrails.
5. Implement date parser with fixed-clock tests.
6. Implement stat metric mappings by league.
7. Wrap `espn-go` in an `ESPNClient` with timeout, retries, and user agent.
8. Implement lookup dispatch by intent.
9. Normalize ESPN typed structs and raw JSON into stable internal row structs.
10. Validate result shape before rendering.
11. Render Markdown from internal row structs, not raw ESPN JSON.
12. Add user-facing error mapping.
13. Add direct chat preflight before normal LLM calls.
14. Optionally add LLM router with strict JSON schema and confidence threshold.
15. Add generic tool interface wrapper if your app supports LLM function calling.
16. Add feature flag and tool permission controls.
17. Build tests in layers: detector, parser, normalizer, validator, renderer, router, error mapping, live integration.

## Practical Design Lessons

- Keep the LLM router advisory, not authoritative. Validate and map every field.
- Keep a deterministic detector even with a router. It handles provider outages and avoids requiring an LLM for obvious sports lookups.
- Put non-lookup guardrails early. Creative/explanatory prompts must not be captured.
- Normalize ESPN data into stable structs before rendering. ESPN shapes differ between typed endpoints and raw JSON endpoints.
- Validate that the returned rows answer the specific request. Generic data is not enough for subtype-specific questions.
- Treat empty ESPN data as a first-class outcome. A clear empty-state table is better than a hard failure.
- Use fixed clocks in tests. Relative sports queries are otherwise brittle.
- Sanitize all external URLs before embedding in Markdown.
- Keep live ESPN audits separate from unit tests. Public ESPN data changes constantly.
