# News Lookup Implementation Summary

## Overview

Added a new **non-sports news lookup capability** to OmniLLM-Studio that detects user questions about current events, routes them to the free Actually Relevant News API, and returns a polished newspaper-style response in Markdown.

## Files Changed

### New Files

| File | Purpose |
|---|---|
| `backend/internal/news/models.go` | Domain models: `Story`, `StoryPage`, `NewsIntent`, `LookupResult`, `EditionInput`, etc. |
| `backend/internal/news/config.go` | Configuration with env-var overrides (`NEWS_LOOKUP_*`) |
| `backend/internal/news/client.go` | HTTP client for Actually Relevant API (`/stories`, `/issues`, etc.) |
| `backend/internal/news/cache.go` | In-memory TTL cache for API responses (5 min default) |
| `backend/internal/news/intent.go` | Deterministic intent detector — sports exclusion, topic mapping, confidence scoring |
| `backend/internal/news/formatter.go` | Newspaper-style Markdown formatter with lead story, headlines, briefs |
| `backend/internal/news/service.go` | Orchestration: intent → API → cache → format → result |
| `backend/internal/news/news_test.go` | 57 unit tests covering intent, client, cache, formatter, config, service |

### Modified Files

| File | Change |
|---|---|
| `backend/internal/db/db.go` | Added V29 migration seeding `news_lookup_enabled` feature flag |
| `backend/internal/api/message_handler.go` | Added `newsSvc` field, `handleNewsLookupMessage()`, `newsLookupEnabled()`, wired into both streaming and non-streaming paths after sports lookup |
| `backend/internal/api/router.go` | Added `news` import, initializes `news.Service` with config, passes to `NewMessageHandler` |
| `frontend/src/components/SettingsPanel.tsx` | Added `NewsLookupCard` toggle component in General tab |
| `README.md` | Added news lookup to feature list, request flow, project structure, config docs, and external dependencies table |

## Architecture

```
User prompt
  → message stream endpoint
  → auth/context loading
  → local preflight checks
      1. sports lookup detector (ESPN) — unchanged, still first
      2. non-sports news detector (Actually Relevant) — NEW
      3. other local tools/enrichments
      4. LLM fallback
  → return formatted Markdown over existing message/SSE path
```

Sports prompts are explicitly rejected by the news detector. The sports path remains first in the routing order.

## Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `NEWS_LOOKUP_ENABLED` | `true` | Enable/disable news lookup |
| `NEWS_LOOKUP_BASE_URL` | `https://actually-relevant-api.onrender.com/api` | API base URL |
| `NEWS_LOOKUP_TIMEOUT_SECONDS` | `8` | HTTP client timeout |
| `NEWS_LOOKUP_CACHE_TTL_SECONDS` | `300` | In-memory cache TTL |
| `NEWS_LOOKUP_DEFAULT_PAGE_SIZE` | `8` | Default stories per request |
| `NEWS_LOOKUP_MAX_PAGE_SIZE` | `15` | Maximum stories per request |

Feature flag: `news_lookup_enabled` (seeded enabled by default in V29 migration).

## Intent Detection

- Deterministic (no LLM classifier needed)
- Rejects sports prompts using the same team/league terms as the sports detector
- Rejects creative/fictional newspaper prompts
- Maps topics to issue slugs: `science-technology`, `planet-climate`, `existential-threats`, `human-development`
- Confidence threshold: 0.65
- Detects presentation style: front page, brief, detailed, or "top N"

## Response Formatting

- Newspaper-style Markdown with `The OmniLLM Daily` / `The OmniLLM Front Page` header
- Lead story, More Headlines, News Briefs sections
- HTML escaping for all user/API-provided text
- Safe Markdown links (only `https://` URLs)
- Broadened search note when results are expanded
- Error messages when API is unreachable

## API Client

- Supports: `GET /stories`, `GET /stories/{slug}`, `GET /stories/{slug}/related`, `GET /stories/{slug}/cluster`, `GET /issues`
- Query parameters: `page`, `pageSize`, `issueSlug`, `search`, `emotionTags`
- Context-based timeouts, typed error handling, JSON decoding
- No API key required

## Tests

57 unit tests covering:

- **Intent detection** (20 tests): sports rejection, news detection, topic mapping, presentation style, non-news rejection
- **API client** (6 tests): URL construction, query params, non-2xx errors, timeouts, malformed JSON, missing optional fields
- **Cache** (4 tests): hit/miss, expiry, clear, stats
- **Formatter** (6 tests): full edition, no results, broadened note, HTML safety, link safety, issue display names
- **Config** (1 test): default values
- **Service** (3 tests): disabled, non-news prompt, sports prompt

## Limitations

1. **No LLM polish pass** — The formatter is fully deterministic. Future enhancement could add an optional LLM rewrite step.
2. **No web search fallback** — If the API fails, the error message suggests enabling web search but does not automatically fall through.
3. **Single API provider** — Only Actually Relevant is supported. Architecture is isolated for future providers.
4. **No tool registration** — Unlike sports, news is not registered as a tool in the tool framework. It only runs as a local preflight check.

## Future Enhancements

1. Optional LLM polish pass for story summaries
2. Web search fallback integration
3. Additional news API providers
4. Tool registration for agent mode
5. Frontend CSS styling for `.omni-news-edition` class