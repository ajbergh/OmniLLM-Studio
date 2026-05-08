// Shared model catalog grouped by use case so chat and image selectors stay in sync.
// Ollama models are always fetched dynamically.

type ProviderModelCatalog = Record<string, { chat: string[]; image?: string[] }>;

const PROVIDER_MODEL_CATALOG: ProviderModelCatalog = {
  openai: {
    chat: [
      'gpt-5.5',
      'gpt-5.4',
      'gpt-5.4-mini',
      'gpt-5.4-nano',
      'gpt-5.4-pro',
      'gpt-5.2',
      'gpt-5.2-pro',
      'gpt-5.1',
      'gpt-5',
      'gpt-5-pro',
      'gpt-5-mini',
      'gpt-5-nano',
      'gpt-4.1',
      'gpt-4.1-mini',
      'gpt-4.1-nano',
      'gpt-4o',
      'gpt-4o-mini',
      'o3-pro',
      'o4-mini',
      'o3',
      'o3-mini',
      'o1',
      'o1-mini',
    ],
    image: ['gpt-image-2', 'gpt-image-1.5', 'chatgpt-image-latest', 'gpt-image-1', 'gpt-image-1-mini', 'dall-e-3', 'dall-e-2'],
  },
  anthropic: {
    chat: [
      'claude-opus-4-7',
      'claude-opus-4-6',
      'claude-sonnet-4-6',
      'claude-haiku-4-5',
      'claude-sonnet-4-20250514',
      'claude-3-7-sonnet-20250219',
      'claude-3-5-sonnet-20241022',
      'claude-3-5-haiku-20241022',
      'claude-3-opus-20240229',
    ],
  },
  gemini: {
    chat: [
      'gemini-3.1-pro-preview',
      'gemini-3.1-flash-lite',
      'gemini-3.1-flash-lite-preview',
      'gemini-3-flash-preview',
      'gemini-2.5-pro',
      'gemini-2.5-pro-preview-06-05',
      'gemini-2.5-flash',
      'gemini-2.5-flash-preview-05-20',
      'gemini-2.5-flash-lite',
      'gemini-2.0-flash',
      'gemini-2.0-flash-lite',
      'gemini-1.5-pro',
      'gemini-1.5-flash',
    ],
    image: [
      'gemini-3.1-flash-image-preview',
      'gemini-3-pro-image-preview',
      'gemini-2.5-flash-image',
      'imagen-4.0-generate-001',
      'imagen-4.0-ultra-generate-001',
      'imagen-4.0-fast-generate-001',
      'imagen-3.0-generate-002',
      'imagen-3.0-fast-generate-001',
    ],
  },
  ollama: { chat: [] },
  openrouter: {
    chat: [
      'openai/gpt-5.5',
      'openai/gpt-5.4',
      'openai/gpt-5.4-mini',
      'openai/gpt-5.4-pro',
      'openai/gpt-5.2',
      'openai/gpt-4.1',
      'openai/gpt-4o',
      'anthropic/claude-opus-4-7',
      'anthropic/claude-opus-4-6',
      'anthropic/claude-sonnet-4-6',
      'google/gemini-3.1-pro-preview',
      'google/gemini-3.1-flash-lite-preview',
      'google/gemini-2.5-pro',
      'google/gemini-2.5-flash',
      'meta-llama/llama-4-maverick',
      'meta-llama/llama-3.3-70b-instruct',
      'deepseek/deepseek-r1',
      'qwen/qwen3-235b-a22b',
      'mistralai/mistral-medium-3-5',
      'mistralai/mistral-large-2512',
    ],
    image: [
      // Google / Gemini (text+image output)
      'google/gemini-2.5-flash-image',
      'google/gemini-3.1-flash-image-preview',
      'google/gemini-3-pro-image-preview',
      // OpenAI (text+image output)
      'openai/gpt-5.4-image-2',
      'openai/gpt-5-image',
      'openai/gpt-5-image-mini',
      // Black Forest Labs / FLUX (image-only; note: dot notation in IDs)
      'black-forest-labs/flux.2-pro',
      'black-forest-labs/flux.2-max',
      'black-forest-labs/flux.2-flex',
      'black-forest-labs/flux.2-klein-4b',
      // Recraft (image-only)
      'recraft/recraft-v3',
      'recraft/recraft-v4',
      'recraft/recraft-v4-pro',
      // Sourceful (image-only)
      'sourceful/riverflow-v2-fast',
      'sourceful/riverflow-v2-fast-preview',
      'sourceful/riverflow-v2-pro',
      'sourceful/riverflow-v2-max-preview',
      'sourceful/riverflow-v2-standard-preview',
      // ByteDance (image-only)
      'bytedance-seed/seedream-4.5',
    ],
  },
  groq: {
    chat: [
      'openai/gpt-oss-120b',
      'openai/gpt-oss-20b',
      'groq/compound',
      'groq/compound-mini',
      'llama-3.3-70b-versatile',
      'llama-3.1-8b-instant',
      'meta-llama/llama-4-scout-17b-16e-instruct',
      'moonshotai/kimi-k2-instruct-0905',
      'deepseek-r1-distill-llama-70b',
      'qwen/qwen3-32b',
      'qwen-qwq-32b',
      'mistral-saba-24b',
      'gemma2-9b-it',
    ],
  },
  together: {
    chat: [
      'MiniMaxAI/MiniMax-M2.5',
      'moonshotai/Kimi-K2.5',
      'moonshotai/Kimi-K2-Instruct-0905',
      'moonshotai/Kimi-K2-Thinking',
      'zai-org/GLM-5',
      'zai-org/GLM-4.7',
      'zai-org/GLM-4.5-Air-FP8',
      'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8',
      'meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo',
      'meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo',
      'Qwen/Qwen3-Coder-Next-FP8',
      'Qwen/Qwen3-Next-80B-A3B-Instruct',
      'Qwen/Qwen3-235B-A22B-Thinking-2507',
      'Qwen/Qwen3-Coder-480B-A35B-Instruct-FP8',
      'Qwen/Qwen3-235B-A22B-Instruct-2507-tput',
      'Qwen/Qwen3.5-397B-A17B',
      'Qwen/Qwen2.5-72B-Instruct-Turbo',
      'deepseek-ai/DeepSeek-V3.1',
      'deepseek-ai/DeepSeek-R1',
      'deepseek-ai/DeepSeek-V3',
      'openai/gpt-oss-120b',
      'google/gemma-2-27b-it',
      'mistralai/Mistral-Small-24B-Instruct-2501',
      'mistralai/Mixtral-8x22B-Instruct-v0.1',
    ],
    image: [
      'google/imagen-4.0-preview',
      'google/imagen-4.0-fast',
      'google/imagen-4.0-ultra',
      'google/flash-image-2.5',
      'google/gemini-3-pro-image',
      'black-forest-labs/FLUX.1-schnell-Free',
      'black-forest-labs/FLUX.1-schnell',
      'black-forest-labs/FLUX.1.1-pro',
      'black-forest-labs/FLUX.1-kontext-pro',
      'black-forest-labs/FLUX.1-kontext-max',
      'black-forest-labs/FLUX.1-krea-dev',
      'black-forest-labs/FLUX.2-pro',
      'black-forest-labs/FLUX.2-dev',
      'black-forest-labs/FLUX.2-flex',
      'ByteDance-Seed/Seedream-3.0',
      'ByteDance-Seed/Seedream-4.0',
      'Qwen/Qwen-Image',
      'RunDiffusion/Juggernaut-pro-flux',
      'Rundiffusion/Juggernaut-Lightning-Flux',
      'HiDream-ai/HiDream-I1-Full',
      'HiDream-ai/HiDream-I1-Dev',
      'HiDream-ai/HiDream-I1-Fast',
      'ideogram/ideogram-3.0',
      'Lykon/DreamShaper',
      'stabilityai/stable-diffusion-3-medium',
      'stabilityai/stable-diffusion-xl-base-1.0',
    ],
  },
  mistral: {
    chat: [
      'mistral-medium-3-5',
      'mistral-medium-latest',
      'mistral-small-2603',
      'mistral-small-latest',
      'mistral-large-2512',
      'mistral-large-latest',
      'mistral-medium-2508',
      'mistral-medium-3-1',
      'magistral-medium-2509',
      'magistral-small-2509',
      'devstral-2512',
      'codestral-2508',
      'codestral-latest',
      'open-mistral-nemo',
      'pixtral-large-latest',
    ],
  },
};

export const KNOWN_MODELS: Record<string, string[]> = Object.fromEntries(
  Object.entries(PROVIDER_MODEL_CATALOG).map(([provider, catalog]) => [
    provider,
    [...catalog.chat, ...(catalog.image || [])],
  ])
) as Record<string, string[]>;

export const KNOWN_IMAGE_MODELS: Record<string, string[]> = Object.fromEntries(
  Object.entries(PROVIDER_MODEL_CATALOG)
    .filter(([, catalog]) => (catalog.image || []).length > 0)
    .map(([provider, catalog]) => [provider, [...(catalog.image || [])]])
) as Record<string, string[]>;

export function getKnownChatModels(providerType: string): string[] {
  return [...(PROVIDER_MODEL_CATALOG[providerType.toLowerCase()]?.chat || [])];
}

export function getKnownImageModels(providerType: string): string[] {
  return [...(PROVIDER_MODEL_CATALOG[providerType.toLowerCase()]?.image || [])];
}

// ---------------------------------------------------------------------------
// Reasoning effort support
// ---------------------------------------------------------------------------

/** Ordered effort levels from least to most reasoning compute. */
export const REASONING_EFFORT_LEVELS = ['low', 'medium', 'high'] as const;
export type ReasoningEffortLevel = (typeof REASONING_EFFORT_LEVELS)[number];

/**
 * Models per provider that accept a `reasoning_effort` parameter.
 * OpenAI o-series and gpt-5.x / gpt-4.x support this natively.
 * Anthropic Claude 3.7+ and 4.x support extended thinking (mapped to budget_tokens server-side).
 * Groq compound systems support it.
 */
const REASONING_EFFORT_MODELS: Record<string, string[]> = {
  openai: [
    'gpt-5.5', 'gpt-5.4', 'gpt-5.4-mini', 'gpt-5.4-nano', 'gpt-5.4-pro',
    'gpt-5.2', 'gpt-5.2-pro', 'gpt-5.1', 'gpt-5', 'gpt-5-pro', 'gpt-5-mini', 'gpt-5-nano',
    'gpt-4.1', 'gpt-4.1-mini', 'gpt-4.1-nano',
    'gpt-4o', 'gpt-4o-mini',
    'o3-pro', 'o4-mini', 'o3', 'o3-mini', 'o1', 'o1-mini',
  ],
  anthropic: [
    'claude-opus-4-7', 'claude-opus-4-6',
    'claude-sonnet-4-6', 'claude-haiku-4-5',
    'claude-sonnet-4-20250514', 'claude-3-7-sonnet-20250219',
  ],
  openrouter: [
    'openai/gpt-5.5', 'openai/gpt-5.4', 'openai/gpt-5.4-mini', 'openai/gpt-5.4-pro',
    'openai/gpt-5.2', 'openai/gpt-4.1', 'openai/gpt-4o',
    'anthropic/claude-opus-4-7', 'anthropic/claude-opus-4-6',
    'anthropic/claude-sonnet-4-6',
  ],
  groq: [
    'openai/gpt-oss-120b', 'openai/gpt-oss-20b',
    'groq/compound', 'groq/compound-mini',
  ],
};

/**
 * Returns the supported reasoning effort levels for the given provider+model,
 * or null if the model does not support reasoning effort.
 */
export function getModelReasoningLevels(
  providerType: string,
  model: string
): ReasoningEffortLevel[] | null {
  const supported = REASONING_EFFORT_MODELS[providerType.toLowerCase()];
  if (!supported) return null;
  if (supported.includes(model)) return [...REASONING_EFFORT_LEVELS];
  return null;
}
