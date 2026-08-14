> **Archived — historical implementation prompt.** Current-information routing is documented in [PROVIDER_AWARE_SEARCH.md](../../PROVIDER_AWARE_SEARCH.md).

# GitHub Copilot Agent Prompt: Add Actually Relevant News API Routing to OmniLLM-Studio

## Role

You are GitHub Copilot Agent Mode working inside the `ajbergh/OmniLLM-Studio` repository.

Implement a new **non-sports news lookup capability** that detects user questions about current news, routes those questions to the free Actually Relevant News API, and returns a polished newspaper-style response in HTML/Markdown.

The project already has an ESPN-backed sports lookup path. This feature must **not** replace, bypass, or interfere with the existing sports lookup capability. Sports news, scores, standings, schedules, rosters, injuries, odds, and player/team stats must continue to route to the ESPN implementation.

---

## Goal

Add a local backend capability that handles prompts such as:

- "What are the latest headlines?"
- "What's happening in AI news?"
- "Show me the latest climate news."
- "Any major science and technology developments today?"
- "What are the top global news stories that matter?"
- "Latest news about nuclear risk."
- "What is going on with public health?"
- "Give me a newspaper-style front page of today's important stories."

The answer should be grounded in Actually Relevant API data and formatted like a small digital newspaper edition rather than a plain JSON dump or basic table.

---

## Required source references

Before coding, inspect the current repo implementation patterns, especially:

- `backend/internal/sports/`
- `backend/internal/tools/`
- `backend/internal/api/message_handler.go`
- `backend/internal/api/router.go`
- existing feature flag implementation
- existing SSE/message streaming path
- existing markdown rendering behavior in the frontend
- README sections for request lifecycle and sports lookup behavior

The current README describes the application as a Go backend + React frontend, with local preflight checks in the message lifecycle. High-confidence sports prompts are handled locally through ESPN before LLM fallback. Use that same architectural pattern for news.

Actually Relevant source URLs:

- Free API intro: `https://actuallyrelevant.news/free-api`
- Developer docs: `https://actuallyrelevant.news/developers`
- API host used by public links: `https://actually-relevant-api.onrender.com`
- OpenAPI route exposed by the public backend, based on the project source: `/api/docs/openapi.json`

Actually Relevant public API routes to support:

- `GET /api/stories`
- `GET /api/stories/{slug}`
- `GET /api/stories/{slug}/related`
- `GET /api/stories/{slug}/cluster`
- `GET /api/issues`
- optionally `GET /api/feed` and `GET /api/feed/{issueSlug}` for RSS fallback/diagnostics only

Primary JSON endpoint for this feature:

```text
https://actually-relevant-api.onrender.com/api/stories
```

Known query parameters for `/api/stories`:

```text
page        positive integer, default 1
pageSize    positive integer, max 100
issueSlug   optional issue slug
search      optional query string, 2-200 chars
emotionTags optional comma-delimited list
```

Known top-level issue areas:

```text
human-development
planet-climate
existential-threats
science-technology
```

Known story fields to tolerate and use when returned:

```text
id
slug
sourceUrl
sourceTitle
title
titleLabel
dateCrawled
datePublished
status
relevancePre
relevance
emotionTag
summary
quote
quoteAttribution
marketingBlurb
relevanceReasons
relevanceSummary
antifactors
issue.name
issue.slug
feed.id
feed.title
feed.displayTitle
feed.issue.name
feed.issue.slug
```

The response from `/api/stories` is expected to be a paginated JSON envelope with at least:

```text
data: Story[]
total
page
pageSize
totalPages
```

Implement the decoder defensively in case the API evolves.

---

## Non-goals

Do not do the following:

1. Do not route sports news through Actually Relevant.
2. Do not remove, weaken, or reorder the existing ESPN sports path in a way that causes sports prompts to be captured by the new news router.
3. Do not answer current-news questions from model memory.
4. Do not scrape arbitrary news websites.
5. Do not require an API key for Actually Relevant.
6. Do not introduce a heavyweight external dependency unless the repo already has an equivalent pattern.
7. Do not create a generic "web search replacement"; this is a focused curated-news integration.
8. Do not expose raw API JSON directly to the user except in debug logs/tests.
9. Do not over-quote source material. Use the API's quote field sparingly when available.

---

## Desired architecture

Add a new backend package parallel to the existing sports implementation:

```text
backend/internal/news/
  client.go
  formatter.go
  intent.go
  models.go
  service.go
  cache.go          optional if no existing reusable cache exists
  news_test.go     or split tests by file
```

Use the current project conventions if the actual repo structure suggests a better location.

The capability should sit in the same local-preflight layer as sports:

```text
User prompt
  -> message stream endpoint
  -> auth/context loading
  -> local preflight checks
      1. sports lookup detector first
      2. non-sports news detector second
      3. other local tools/enrichments
      4. LLM fallback
  -> return formatted Markdown/HTML over existing message/SSE path
```

Sports must remain first. News should only run if the sports detector did not handle the prompt.

---

## Feature flag and configuration

Add configuration values consistent with existing project style.

Recommended names:

```text
news_lookup_enabled                 default true
news_lookup_timeout_seconds          default 8
news_lookup_cache_ttl_seconds        default 300
news_lookup_default_page_size        default 8
news_lookup_max_page_size            default 15
news_lookup_base_url                 default https://actually-relevant-api.onrender.com/api
news_lookup_user_agent               default OmniLLM-Studio/NewsLookup
```

If the repo already uses environment-backed config naming, use that style. For example:

```text
NEWS_LOOKUP_ENABLED=true
NEWS_LOOKUP_BASE_URL=https://actually-relevant-api.onrender.com/api
NEWS_LOOKUP_TIMEOUT_SECONDS=8
NEWS_LOOKUP_CACHE_TTL_SECONDS=300
NEWS_LOOKUP_DEFAULT_PAGE_SIZE=8
NEWS_LOOKUP_MAX_PAGE_SIZE=15
```

Expose the feature flag through the existing feature flags API if sports uses one. Match the sports flag pattern.

---

## Intent detection requirements

Implement deterministic intent detection first. Only use an LLM-based classifier if the project already has a clean local classifier pattern and deterministic rules are not sufficient.

Create a `NewsIntent` model similar to:

```go
type NewsIntent struct {
    Handled       bool
    Confidence    float64
    Query         string
    IssueSlug     string
    PageSize      int
    WantsHTML     bool
    WantsBrief    bool
    WantsDetailed bool
    WantsFrontPage bool
    Reason        string
}
```

### Positive news indicators

Treat these as news/current-event signals:

```text
news
headlines
latest
breaking
today
this week
current events
developments
what happened
what is happening
what's going on
coverage
stories
front page
newspaper
article
press
global news
world news
top stories
major stories
important stories
```

### Topic mapping

Map obvious topics to Actually Relevant issue slugs:

```text
science-technology:
  science, technology, tech, AI, artificial intelligence, LLM, OpenAI, Anthropic,
  Google AI, chips, semiconductors, space, astronomy, research, robots,
  cybersecurity, software, biotech when the emphasis is scientific/technical

planet-climate:
  climate, planet, environment, emissions, fossil fuels, clean energy, renewable,
  carbon, biodiversity, conservation, pollution, oceans, deforestation,
  extreme weather when framed as climate news

existential-threats:
  existential, nuclear, nuclear risk, biosecurity, pandemic risk, AI safety,
  AI risk, catastrophic risk, war escalation, global security,
  autonomous weapons, great power conflict

human-development:
  health, education, poverty, food security, inequality, migration, human rights,
  governance, democracy, development, labor, public health, medicine,
  humanitarian, children, economy when framed around human/social outcomes
```

If no issue can be confidently inferred, call `/api/stories` without `issueSlug`.

If the user asks for a topic-specific query, pass a concise search query using `search`.

Examples:

```text
"latest climate news" ->
  issueSlug=planet-climate
  search empty or "climate" depending on current API result quality

"what's happening with AI regulation" ->
  issueSlug=science-technology
  search="AI regulation"

"top global headlines" ->
  issueSlug empty
  search empty

"latest pandemic risk news" ->
  issueSlug=existential-threats
  search="pandemic risk"
```

### Sports exclusion

The news detector must explicitly reject sports prompts. Do not let the word "news" alone capture sports.

Reject if prompt contains clear sports entities or sports request types, including but not limited to:

```text
sports
MLB
NBA
NFL
NHL
WNBA
MLS
EPL
Premier League
college football
college basketball
NCAA
Cubs
Bears
Packers
Bulls
Blackhawks
Brewers
Yankees
Dodgers
Lakers
Warriors
scores
standings
schedule
odds
spread
moneyline
roster
injury report
player stats
team stats
home runs
touchdowns
goals
assists
playoffs
draft
trade deadline
```

If the existing sports detector has its own team/league lists, reuse those. The correct behavior is:

```text
"latest Cubs news" -> sports route
"latest NBA news" -> sports route
"latest AI news" -> news route
"latest climate headlines" -> news route
```

### Confidence threshold

Only handle news locally when confidence is high enough.

Suggested threshold:

```text
confidence >= 0.65
```

Examples that should handle:

```text
"Give me today's top news"
"Latest science headlines"
"What is happening in climate news?"
"Show me important global stories"
```

Examples that should not handle:

```text
"Explain climate change"
"Write a fictional newspaper article"
"Summarize this article I pasted below"
"Tell me the history of newspapers"
"Create a news-style landing page for my event"
```

---

## API client requirements

Create an Actually Relevant API client using Go standard library unless a project-wide HTTP client abstraction already exists.

Suggested public interface:

```go
type Client struct {
    BaseURL    string
    HTTPClient *http.Client
    UserAgent  string
}

type StoryQuery struct {
    Page        int
    PageSize    int
    IssueSlug   string
    Search      string
    EmotionTags []string
}

func (c *Client) GetStories(ctx context.Context, q StoryQuery) (*StoryPage, error)
func (c *Client) GetStory(ctx context.Context, slug string) (*Story, error)
func (c *Client) GetRelated(ctx context.Context, slug string, limit int) ([]Story, error)
func (c *Client) GetCluster(ctx context.Context, slug string) (*ClusterResponse, error)
func (c *Client) GetIssues(ctx context.Context) ([]Issue, error)
```

### Request behavior

- Use `context.WithTimeout`.
- Set `Accept: application/json`.
- Set a clear `User-Agent`.
- URL-encode query parameters.
- Clamp `pageSize` to configured max.
- Treat non-2xx responses as typed errors with status code and truncated body.
- Include request ID in logs if the repo has request-scoped logging.
- Never panic on missing fields.
- Do not log full response bodies by default.

### Caching

Add small in-memory TTL caching for repeated news prompts.

Cache key should include:

```text
issueSlug
search
page
pageSize
emotionTags
```

Default TTL: 5 minutes.

Do not persist news results to SQLite unless the repo has a clear cache table pattern. This is a lookup capability, not durable content ingestion.

### Failure behavior

If the API fails:

Return a user-facing response like:

```markdown
# News lookup unavailable

I could not reach the Actually Relevant News API right now.

**What I tried:** latest non-sports news lookup  
**Fallback:** I am not going to answer from memory because this is a current-news request.
```

If existing web search is enabled and project conventions allow fallback, optionally add:

```markdown
You can enable web search fallback for current-news prompts in Settings.
```

But do not silently answer from model memory.

If no stories are found:

```markdown
# No matching curated stories found

I checked Actually Relevant for this topic, but it did not return matching published stories.

Try a broader query such as "latest technology news" or "top global headlines."
```

---

## Response formatting requirements

The user specifically wants output as nicely formatted HTML/Markdown in a **"Newspaper" like format**.

Create a deterministic formatter. Do not rely on the LLM to produce the layout unless the current system already has a controlled post-processing path. The formatter should be attractive, readable, and safe in the existing Markdown renderer.

### Preferred output style

Use Markdown with light semantic HTML if the frontend allows safe HTML. If HTML is sanitized or not rendered, use pure Markdown with horizontal rules and blockquotes.

Target structure:

```markdown
<div class="omni-news-edition">

# The OmniLLM Daily

**Non-sports news edition** · Actually Relevant News · Updated May 8, 2026

---

## Lead Story

### [Story Title](https://source-url.example)

*Science & Technology · Relevance 8/10 · Published May 8, 2026*

Summary paragraph.

> Short quote when available.

**Why it matters:** Relevance summary or relevance reasons.

---

## More Headlines

### [Second Story](https://source-url.example)
*Planet & Climate · Relevance 7/10*

One-sentence summary.

### [Third Story](https://source-url.example)
*Human Development · Relevance 7/10*

One-sentence summary.

---

## Source Notes

Stories are from Actually Relevant's curated public API. Links go to the original source URLs returned by the API.

</div>
```

### Layout logic

When at least 1 story exists:

- First story becomes `Lead Story`.
- Next 3-5 stories become `More Headlines`.
- If 6+ stories are returned, add a compact `News Briefs` section.
- Include source links using `sourceUrl`.
- Prefer `titleLabel`, then `title`, then `sourceTitle`.
- Prefer `datePublished`, then `dateCrawled`.
- Display issue name if available.
- Display relevance score if available.
- Display `emotionTag` only if it adds value; do not overemphasize emotional tone.
- Use `summary` as the deck/body.
- Use `relevanceSummary` or `relevanceReasons` for "Why it matters."
- Use `quote` + `quoteAttribution` only when short and available.
- Never invent fields.
- Never fabricate source names.

### HTML safety

If using HTML:

- Do not include scripts.
- Do not include external CSS.
- Do not include iframes.
- Do not include inline event handlers.
- Escape all user/API-provided text.
- Allow links only to `https://` URLs.
- If URL invalid, render title without a link.

### Optional frontend enhancement

If the existing Markdown renderer supports class names and CSS, add styling for `.omni-news-edition` to make it look newspaper-like:

- serif headline font if available
- subtle border
- muted dateline
- column-like headline sections on wide screens
- strong typographic hierarchy
- readable blockquotes
- no garish colors

If the renderer strips classes, keep the Markdown attractive without CSS.

Do not block the backend implementation on frontend styling.

---

## Suggested implementation phases

### Phase 1 — Codebase review and plan

Inspect the existing sports lookup implementation and message preflight routing.

Create a short internal implementation plan before editing code. Identify:

- where sports intent is detected
- where sports result formatting happens
- how direct local responses are returned to the chat stream
- how feature flags are loaded
- how tests are organized
- how Markdown/HTML is rendered

Do not ask the user for clarification. Make reasonable implementation choices consistent with the repo.

### Phase 2 — News domain models

Create models for:

```go
type StoryPage struct {
    Data       []Story `json:"data"`
    Total      int     `json:"total"`
    Page       int     `json:"page"`
    PageSize   int     `json:"pageSize"`
    TotalPages int     `json:"totalPages"`
}

type Story struct {
    ID                string      `json:"id"`
    Slug              string      `json:"slug"`
    SourceURL         string      `json:"sourceUrl"`
    SourceTitle       string      `json:"sourceTitle"`
    Title             string      `json:"title"`
    TitleLabel        string      `json:"titleLabel"`
    DateCrawled       *time.Time  `json:"dateCrawled"`
    DatePublished     *time.Time  `json:"datePublished"`
    Status            string      `json:"status"`
    RelevancePre      *int        `json:"relevancePre"`
    Relevance         *int        `json:"relevance"`
    EmotionTag        string      `json:"emotionTag"`
    Summary           string      `json:"summary"`
    Quote             string      `json:"quote"`
    QuoteAttribution  string      `json:"quoteAttribution"`
    MarketingBlurb    string      `json:"marketingBlurb"`
    RelevanceReasons  string      `json:"relevanceReasons"`
    RelevanceSummary  string      `json:"relevanceSummary"`
    Antifactors       string      `json:"antifactors"`
    Issue             *IssueRef   `json:"issue"`
    Feed              *FeedRef    `json:"feed"`
}

type IssueRef struct {
    Name string `json:"name"`
    Slug string `json:"slug"`
}

type FeedRef struct {
    ID           string    `json:"id"`
    Title        string    `json:"title"`
    DisplayTitle string    `json:"displayTitle"`
    Issue        *IssueRef `json:"issue"`
}
```

If the codebase uses pointer-free model conventions, adapt accordingly.

Make timestamp parsing robust. If the API returns strings that do not parse cleanly into `time.Time`, preserve the raw date string or omit the date rather than failing the entire response.

### Phase 3 — API client

Implement `client.go`.

Client must support:

```text
GET /stories?page=1&pageSize=8
GET /stories?issueSlug=science-technology&page=1&pageSize=8
GET /stories?search=AI%20regulation&issueSlug=science-technology&page=1&pageSize=8
GET /stories/{slug}
GET /stories/{slug}/related?limit=4
GET /stories/{slug}/cluster
GET /issues
```

Add unit tests using `httptest.Server`.

Test cases:

1. correct URL/path/query construction
2. successful decode
3. non-2xx error
4. timeout/context cancellation
5. malformed JSON
6. missing optional fields
7. pageSize clamping
8. cache hit/miss if cache implemented here

### Phase 4 — Intent detector

Implement `intent.go`.

Public function:

```go
func DetectNewsIntent(prompt string) NewsIntent
```

Or match the project's sports detector style.

Detector behavior:

- normalize whitespace and lowercase
- reject empty/very short prompts
- reject sports first
- detect news/current-events terms
- extract issue slug
- extract search query
- detect desired shape:
  - `front page`, `newspaper`, `edition` -> front page
  - `brief`, `quick`, `summary` -> brief
  - `detailed`, `deep dive`, `why it matters` -> detailed
  - `html`, `markdown` -> output preference
- infer page size:
  - "top 3" -> 3
  - "top 10" -> 10
  - default -> configured default
  - clamp -> configured max

Test cases:

```text
"latest Cubs news" -> not handled
"latest NBA news" -> not handled
"What are the latest MLB standings?" -> not handled
"latest AI news" -> handled, science-technology
"latest climate headlines" -> handled, planet-climate
"give me the top global headlines" -> handled, no issueSlug
"what is going on with nuclear risk?" -> handled, existential-threats
"explain AI safety" -> not handled unless current-news wording is present
"write a fake newspaper story" -> not handled
```

### Phase 5 — News service orchestration

Create `service.go`.

Responsibilities:

1. Accept user prompt/context.
2. Run `DetectNewsIntent`.
3. If not handled, return `(handled=false)`.
4. Build `StoryQuery`.
5. Call Actually Relevant client.
6. If no results:
   - optionally broaden search once:
     - if `search` + `issueSlug` returns no results, retry with only `issueSlug`
     - if `search` only returns no results, retry latest stories without search
   - mark response as broadened in a small note
7. Format the response.
8. Return direct assistant content in the same shape sports uses.

Suggested function:

```go
type LookupResult struct {
    Handled  bool
    Content  string
    Metadata map[string]any
}

func (s *Service) TryAnswer(ctx context.Context, prompt string) (*LookupResult, error)
```

Metadata should include useful non-sensitive fields:

```text
provider=actually_relevant
issueSlug
search
storyCount
fromCache
durationMs
```

### Phase 6 — Message routing integration

Wire the news service into the existing message path after sports.

Pseudo-flow:

```go
if sportsLookupEnabled {
    if result := sportsService.TryAnswer(ctx, prompt); result.Handled {
        streamDirectAssistantMessage(result.Content, result.Metadata)
        return
    }
}

if newsLookupEnabled {
    if result := newsService.TryAnswer(ctx, prompt); result.Handled {
        streamDirectAssistantMessage(result.Content, result.Metadata)
        return
    }
}

// existing LLM path
```

Important:

- preserve existing SSE event ordering
- preserve conversation persistence
- ensure the direct response is saved as an assistant message
- ensure errors are presented as assistant responses only when intent was definitely news
- if detector confidence is low, do not show API errors; simply fall through to normal LLM/tool flow

### Phase 7 — Formatter

Implement `formatter.go`.

Public function:

```go
func FormatNewspaperEdition(input EditionInput) string
```

Suggested `EditionInput`:

```go
type EditionInput struct {
    Prompt       string
    Intent       NewsIntent
    Stories      []Story
    Total        int
    Broadened    bool
    GeneratedAt  time.Time
}
```

Output must be Markdown-first and frontend-safe.

Formatter rules:

- Create a publication-style title:
  - `The OmniLLM Daily`
  - or `The OmniLLM Front Page`
  - include topic if available: `Science & Technology Edition`
- Add a dateline:
  - current local date/time if available
  - provider: `Actually Relevant News`
- Lead story:
  - title linked to `sourceUrl`
  - issue/relevance/date metadata
  - summary
  - quote if available
  - why it matters
- More headlines:
  - title linked to source
  - one concise summary
- News briefs:
  - compact list of additional stories
- Source note:
  - brief statement that links come from the API's source URLs
- If results were broadened:
  - add a small note: `No exact match was found, so I broadened the edition to the closest current curated stories.`

The formatter must escape:

- `<`
- `>`
- `&`
- untrusted quote text
- untrusted titles
- untrusted summaries

When creating Markdown links:

- only link if URL parses and scheme is `https`
- escape `]`, `[`, `(`, `)` in link text
- otherwise render plain text title

### Phase 8 — Optional frontend polish

Inspect the Markdown renderer.

If custom class names are preserved, add CSS for:

```css
.omni-news-edition
.omni-news-edition .dateline
.omni-news-edition .lead-story
.omni-news-edition blockquote
.omni-news-edition .news-briefs
```

Keep it subtle and professional.

If the renderer strips HTML/classes, do not fight it. Improve pure Markdown formatting instead.

### Phase 9 — Documentation

Update README or docs with a new section after sports lookup:

```markdown
### Ask Current News Questions

News lookup is enabled by default and does not require an API key. OmniLLM-Studio detects high-confidence non-sports news prompts, calls the Actually Relevant public API, and returns a newspaper-style Markdown/HTML edition.

Examples:
- "What are the latest headlines?"
- "Show me today's AI news."
- "Latest climate headlines."
- "Give me a front page of important global stories."
- "What's happening with nuclear risk?"

Sports prompts continue to use the ESPN-backed sports lookup.
```

Document config flags and fallback behavior.

### Phase 10 — Tests and verification

Run the existing test suite plus new tests.

Add or update tests for:

- sports prompt still routes to sports
- non-sports news prompt routes to news
- no false positives for creative writing/newsletter/newspaper-design prompts
- API client behavior
- formatter output
- config defaults
- feature flag disabled
- error responses
- no-result responses
- Markdown link safety
- HTML escaping

Manual smoke tests:

```text
"What are the latest headlines?"
"Latest AI news"
"Show me climate headlines in a newspaper format"
"What is happening with nuclear risk?"
"Latest Cubs news"
"What are the current MLB standings?"
"Write a fictional newspaper article about a Christmas market"
```

Expected manual outcomes:

- First four use Actually Relevant.
- Cubs/MLB use ESPN sports path.
- Fictional newspaper prompt goes to normal LLM path, not news lookup.

---

## Acceptance criteria

The implementation is complete when:

1. Non-sports current-news prompts are detected and answered locally.
2. Sports prompts continue to route to the existing ESPN sports capability.
3. Actually Relevant API calls are made server-side.
4. No Actually Relevant API key is required.
5. Responses are grounded only in returned API data.
6. Responses are saved into conversation history as assistant messages.
7. The response is attractive and newspaper-like, not a plain table.
8. API failures do not result in hallucinated current-news answers.
9. Config flags and defaults are documented.
10. Unit tests cover intent detection, client behavior, formatting, and routing priority.
11. The app builds and existing tests pass.

---

## Example target output

Use this as a style target, not as hardcoded content:

```markdown
<div class="omni-news-edition">

# The OmniLLM Daily

**Science & Technology Edition** · Actually Relevant News · May 8, 2026

---

## Lead Story

### [Big tech signs code as Europe demands source lists](https://example.com/original-source)

*Science & Technology · Relevance 8/10 · Published May 8, 2026*

New transparency requirements are changing how major technology companies document AI training data and copyright compliance.

> "Short source quote when available." — Source attribution

**Why it matters:** This affects how AI vendors disclose training data practices and may shape future compliance obligations for general-purpose AI systems.

---

## More Headlines

### [Startup releases software to inspect and change LLM behavior](https://example.com/original-source)
*Science & Technology · Relevance 7/10*

A new interpretability tool gives developers more visibility into model behavior and intervention points.

### [Another relevant story](https://example.com/original-source)
*Science & Technology · Relevance 7/10*

One concise sentence explaining the story.

---

## News Briefs

- **Story title:** one-sentence summary.
- **Story title:** one-sentence summary.

---

## Source Notes

This edition was generated from Actually Relevant's public curated news API. Links point to the original source URLs returned by the API.

</div>
```

---

## Implementation notes

- Prefer a deterministic formatter over LLM summarization for speed, cost, and grounding.
- If you add an LLM polish option later, it must be opt-in and must only rewrite returned story data without adding facts.
- Keep the API integration isolated so future providers can be added later.
- Consider a future `news` tool registration after the local preflight implementation works.
- Make this capability feel native to OmniLLM-Studio, similar to sports lookup.

---

## Final task for Copilot

Implement the feature end-to-end.

After implementation, create a new markdown file in the repo:

```text
docs/news-lookup-implementation-summary.md
```

Include:

1. files changed
2. architecture summary
3. config flags
4. examples tested
5. any limitations
6. future enhancements

Then run the relevant Go and frontend tests/build checks and report results.
