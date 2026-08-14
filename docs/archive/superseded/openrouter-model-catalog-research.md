> **Archived — time-sensitive research snapshot.** Provider/model catalog data is not planning authority and must be refreshed from a live provider source when needed.

# OpenRouter Model Catalog Research

> Date: 2026-05-08
> Source: `GET https://openrouter.ai/api/v1/models` (367 models returned)
> Source: `GET https://openrouter.ai/api/v1/models?output_modalities=text,image` (subset)

---

## Current Frontend Issues

### 1. Three Anthropic model IDs use wrong delimiter

The current `models.ts` uses **hyphens** where the API uses **dots**:

| Current (broken) | Correct API ID |
|---|---|
| `anthropic/claude-opus-4-7` | `anthropic/claude-opus-4.7` |
| `anthropic/claude-opus-4-6` | `anthropic/claude-opus-4.6` |
| `anthropic/claude-sonnet-4-6` | `anthropic/claude-sonnet-4.6` |

These model IDs return **400 "not a valid model ID"** errors. All other model IDs in the current list are valid.

### 2. `openai/gpt-4o` does NOT exist on OpenRouter

`openai/gpt-4o` is not listed in the OpenRouter API response. It should be replaced with `openai/gpt-4.1` (already in the list) or similar valid model.

### 3. Missing many high-profile models

The current chat list has only 20 models. A comprehensive list should have ~35-40 mostly non-free models plus ~10 free-tier models.

---

## Recommended Chat Model Catalog (~40 models)

Models grouped by provider family, sorted roughly by capability/cost. Pricing as of 2026-05-08.

### OpenAI (currently have 6 of 15+ available)

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Add? |
|---|---|---|---|---|
| `openai/gpt-5.5` | $5.00 | $30.00 | 1.05M | ✓ already |
| `openai/gpt-5.5-pro` | $30.00 | $180.00 | 1.05M | **ADD** |
| `openai/gpt-5.4` | $2.50 | $15.00 | 1.05M | ✓ already |
| `openai/gpt-5.4-pro` | $30.00 | $180.00 | 1.05M | ✓ already |
| `openai/gpt-5.4-mini` | $0.75 | $4.50 | 400K | ✓ already |
| `openai/gpt-5.4-nano` | $0.20 | $1.25 | 400K | **ADD** |
| `openai/gpt-5.2` | $1.75 | $14.00 | 400K | ✓ already |
| `openai/gpt-5.2-pro` | $21.00 | $168.00 | 400K | **ADD** |
| `openai/gpt-5` | $1.25 | $10.00 | 400K | **ADD** |
| `openai/gpt-5-mini` | $0.25 | $2.00 | 400K | **ADD** |
| `openai/gpt-5-nano` | $0.05 | $0.40 | 400K | **ADD** |
| `openai/gpt-5-pro` | $15.00 | $120.00 | 400K | **ADD** |
| `openai/gpt-5.1` | $1.25 | $10.00 | 400K | **ADD (behind gpt-5.2)** |
| `openai/gpt-4.1` | $2.00 | $8.00 | 1.05M | ✓ already |
| `openai/gpt-4.1-mini` | $0.40 | $1.60 | 1.05M | **ADD** |
| `openai/gpt-4.1-nano` | $0.10 | $0.40 | 1.05M | **ADD** |
| `openai/o3` | $2.00 | $8.00 | 200K | **ADD** |
| `openai/o3-mini` | $1.10 | $4.40 | 200K | **ADD** |
| `openai/o3-pro` | $20.00 | $80.00 | 200K | **ADD** |
| `openai/o4-mini` | $1.10 | $4.40 | 200K | **ADD** |

### Anthropic (currently have 3 — all with wrong IDs)

| Correct API ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Notes |
|---|---|---|---|---|
| `anthropic/claude-opus-4.7` | $5.00 | $25.00 | 1M | Latest flagship Opus |
| `anthropic/claude-opus-4.6` | $5.00 | $25.00 | 1M | Previous Opus |
| `anthropic/claude-opus-4.6-fast` | $30.00 | $150.00 | 1M | Faster variant |
| `anthropic/claude-sonnet-4.6` | $3.00 | $15.00 | 1M | Latest Sonnet |
| `anthropic/claude-sonnet-4.5` | $3.00 | $15.00 | 1M | |  
| `anthropic/claude-sonnet-4` | $3.00 | $15.00 | 1M | |
| `anthropic/claude-haiku-4.5` | $1.00 | $5.00 | 200K | Fast/cheap |

### Google / Gemini (some already present)

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Add? |
|---|---|---|---|---|
| `google/gemini-3.1-pro-preview` | $2.00 | $12.00 | 1M | ✓ already |
| `google/gemini-3.1-flash-lite-preview` | $0.25 | $1.50 | 1M | ✓ already |
| `google/gemini-3.1-flash-lite` | $0.25 | $1.50 | 1M | **ADD** (GA version) |
| `google/gemini-3-flash-preview` | $0.50 | $3.00 | 1M | **ADD** |
| `google/gemini-2.5-pro` | $1.25 | $10.00 | 1M | ✓ already |
| `google/gemini-2.5-flash` | $0.30 | $2.50 | 1M | ✓ already |
| `google/gemini-2.5-flash-lite` | $0.10 | $0.40 | 1M | **ADD** |

### DeepSeek

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Add? |
|---|---|---|---|---|
| `deepseek/deepseek-v4-pro` | $0.44 | $0.87 | 1M | **ADD** |
| `deepseek/deepseek-v4-flash` | $0.14 | $0.28 | 1M | **ADD** |
| `deepseek/deepseek-r1` | — | — | — | ✓ already |
| `deepseek/deepseek-chat` (V3) | $0.32 | $0.89 | 164K | **ADD** |

### Meta / Llama

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Add? |
|---|---|---|---|---|
| `meta-llama/llama-4-maverick` | $0.15 | $0.60 | 1M | ✓ already |
| `meta-llama/llama-4-scout` | $0.08 | $0.30 | 328K | **ADD** |
| `meta-llama/llama-3.3-70b-instruct` | $0.10 | $0.32 | 131K | ✓ already |

### Qwen

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Add? |
|---|---|---|---|---|
| `qwen/qwen3.6-plus` | $0.33 | $1.95 | 1M | **ADD** |
| `qwen/qwen3.6-flash` | $0.25 | $1.50 | 1M | **ADD** |
| `qwen/qwen3.5-plus-20260420` | $0.40 | $2.40 | 1M | **ADD** |
| `qwen/qwen3.5-flash-02-23` | $0.07 | $0.26 | 1M | **ADD** |
| `qwen/qwen3-235b-a22b` | $0.07 | $0.10 | 262K | ✓ already |
| `qwen/qwen3-coder` | $0.22 | $1.80 | 262K | **ADD** |
| `qwen/qwen3-max` | $0.78 | $3.90 | 262K | **ADD** |
| `qwen/qwen-plus` | $0.26 | $0.78 | 1M | **ADD** |

### xAI / Grok

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Add? |
|---|---|---|---|---|
| `x-ai/grok-4.20` | $1.25 | $2.50 | 2M | **ADD** |
| `x-ai/grok-4.20-multi-agent` | $2.00 | $6.00 | 2M | **ADD** |
| `x-ai/grok-4.3` | $1.25 | $2.50 | 1M | **ADD** |
| `x-ai/grok-4` | $3.00 | $15.00 | 256K | **ADD** |
| `x-ai/grok-4-fast` | $0.20 | $0.50 | 2M | **ADD** |

### Mistral

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Add? |
|---|---|---|---|---|
| `mistralai/mistral-large-2512` | $0.50 | $1.50 | 262K | ✓ already |
| `mistralai/mistral-medium-3-5` | $1.50 | $7.50 | 262K | ✓ already |
| `mistralai/mistral-small-2603` | $0.15 | $0.60 | 262K | **ADD** |
| `mistralai/codestral-2508` | $0.30 | $0.90 | 256K | **ADD** |

### Others worth adding

| Model ID | Prompt $/1M tok | Comp $/1M tok | Ctx | Notes |
|---|---|---|---|---|
| `cohere/command-a` | $2.50 | $10.00 | 256K | **ADD** |
| `amazon/nova-lite-v1` | $0.06 | $0.24 | 300K | **ADD** |
| `amazon/nova-pro-v1` | $0.80 | $3.20 | 300K | **ADD** |
| `amazon/nova-2-lite-v1` | $0.30 | $2.50 | 1M | **ADD** |
| `minimax/minimax-m2.5` | $0.15 | $1.15 | 197K | **ADD** |
| `minimax/minimax-m2.7` | $0.30 | $1.20 | 197K | **ADD** |
| `inclusionai/ling-2.6-1t` | $0.30 | $2.50 | 262K | **ADD** |
| `inclusionai/ling-2.6-flash` | $0.08 | $0.24 | 262K | **ADD** |
| `bytedance-seed/seed-2.0-lite` | $0.25 | $2.00 | 262K | **ADD** |
| `bytedance-seed/seed-2.0-mini` | $0.10 | $0.40 | 262K | **ADD** |
| `z-ai/glm-5.1` | $1.05 | $3.50 | 203K | **ADD** |
| `z-ai/glm-5` | $0.60 | $1.92 | 203K | **ADD** |
| `z-ai/glm-4.7` | $0.40 | $1.75 | 203K | **ADD** |
| `openrouter/auto` | auto-routed | auto-routed | 2M | **ADD** — auto-router |
| `openrouter/free` | $0 | $0 | 200K | **ADD** — routes to free models |

---

## Free Models

These models charge **$0 per token** via OpenRouter's free plan. Should be listed at the top of the dropdown with a visual FREE indicator.

| Model ID | Ctx | Name |
|---|---|---|
| `openrouter/owl-alpha` | 1.05M | Owl Alpha |
| `google/lyria-3-clip-preview` | 1.05M | Google: Lyria 3 Clip Preview |
| `google/lyria-3-pro-preview` | 1.05M | Google: Lyria 3 Pro Preview |
| `google/gemma-4-26b-a4b-it:free` | 262K | Google: Gemma 4 26B A4B (free) |
| `google/gemma-4-31b-it:free` | 262K | Google: Gemma 4 31B (free) |
| `nvidia/nemotron-3-super-120b-a12b:free` | 262K | NVIDIA: Nemotron 3 Super (free) |
| `nvidia/nemotron-3-nano-30b-a3b:free` | 256K | NVIDIA: Nemotron 3 Nano 30B A3B (free) |
| `qwen/qwen3-next-80b-a3b-instruct:free` | 262K | Qwen: Qwen3 Next 80B A3B Instruct (free) |
| `qwen/qwen3-coder:free` | 262K | Qwen: Qwen3 Coder 480B A35B (free) |
| `tencent/hy3-preview:free` | 262K | Tencent: Hy3 preview (free) |
| `openrouter/free` | 200K | Free Models Router |
| `minimax/minimax-m2.5:free` | 197K | MiniMax: MiniMax M2.5 (free) |
| `meta-llama/llama-3.3-70b-instruct:free` | 65K | Meta: Llama 3.3 70B Instruct (free) |
| `meta-llama/llama-3.2-3b-instruct:free` | 131K | Meta: Llama 3.2 3B Instruct (free) |
| `openai/gpt-oss-120b:free` | 131K | OpenAI: gpt-oss-120b (free) |
| `openai/gpt-oss-20b:free` | 131K | OpenAI: gpt-oss-20b (free) |

> **Note**: Free models use the `:free` suffix. When a model is selected without `:free`, it charges. The paid equivalents also exist (e.g. `meta-llama/llama-3.3-70b-instruct` is paid, `meta-llama/llama-3.3-70b-instruct:free` is free). Keep both in the dropdown or just the paid ones + `openrouter/free` router.

---

## Implementation Approach

For the `models.ts` `PROVIDER_MODEL_CATALOG`, the OpenRouter `chat` array should be expanded from 20 to ~40 entries. Recommended ordering:

1. **Special routers**: `openrouter/auto`, `openrouter/free`
2. **Free models** (top ones): `google/gemma-4-31b-it:free`, `nvidia/nemotron-3-super-120b-a12b:free`, `qwen/qwen3-coder:free`, `meta-llama/llama-3.3-70b-instruct:free`
3. **OpenAI**: flagship → pro → mini/nano (gpt-5.5, gpt-5.5-pro, gpt-5.4, gpt-5.4-pro, gpt-5.4-mini, gpt-5.4-nano, gpt-5.2, gpt-5.2-pro, gpt-5, gpt-5-mini, gpt-5-nano, gpt-5.1, gpt-4.1, gpt-4.1-mini, gpt-4.1-nano)
4. **Anthropic** (with dots!): claude-opus-4.7, claude-opus-4.6, claude-sonnet-4.6, claude-sonnet-4.5, claude-haiku-4.5
5. **Google**: gemini-3.1-pro-preview, gemini-3.1-flash-lite, gemini-2.5-pro, gemini-2.5-flash, gemini-2.5-flash-lite
6. **DeepSeek**: deepseek-v4-pro, deepseek-v4-flash, deepseek/deepseek-r1, deepseek/deepseek-chat
7. **Meta**: llama-4-maverick, llama-4-scout, llama-3.3-70b-instruct
8. **Qwen**: qwen3.6-plus, qwen3.6-flash, qwen3.5-flash, qwen3-235b-a22b, qwen3-coder, qwen3-max, qwen-plus
9. **xAI**: grok-4.20, grok-4.3, grok-4-fast
10. **Mistral**: mistral-large-2512, mistral-medium-3-5, mistral-small-2603, codestral-2508
11. **Others**: cohere/command-a, amazon/nova-pro-v1, amazon/nova-lite-v1, minimax/m2.5, minimax/m2.7, bytedance-seed/seed-2.0-lite, inclusionai/ling-2.6-flash

For **FREE marking**: Models with `:free` suffix indicate free tier. The `openrouter/free` router also routes to free models. For the frontend display, check for `:free` suffix or add a separate `free: boolean` metadata field in a future refactor.

### Comparison to Together.ai

For reference, the Together.ai catalog in `models.ts` has ~28 chat models. The OpenRouter catalog should be similar in size but spans multiple providers so ~40 is appropriate.

---

## Frontend Changes Needed

### `models.ts`

1. **Fix 3 Anthropic model IDs**: `claude-opus-4-7` → `claude-opus-4.7`, etc.
2. **Remove `openai/gpt-4o`** (doesn't exist on OpenRouter API)
3. **Expand from 20 to ~38-42 chat models**
4. **Keep image models as-is** (already updated in previous change)

### Display Enhancement (optional)

The `:free` suffix in model IDs could be visually styled in the model selector (e.g., a green "FREE" badge). This would require either:
- Parsing the model ID for `:free` suffix in the UI component
- Adding an extra `modelFreeMap: Record<string, boolean>` metadata object in `models.ts`
