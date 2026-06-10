// api.ts is the central HTTP client for the OmniLLM-Studio frontend.
// It wraps `fetch` to automatically attach bearer tokens, handles error parsing,
// and implements SSE stream parsing for chunked LLM generation.

import type {
  Conversation,
  ConversationKind,
  CreateConversationRequest,
  UpdateConversationRequest,
  Message,
  SendMessageRequest,
  ProviderProfile,
  CreateProviderRequest,
  UpdateProviderRequest,
  WebSearchRequest,
  WebSearchResponse,
  WebSearchResult,
  FileSearchResult,
  FileSearchResponse,
  LibraryFile,
  FileLibrarySummaryResponse,
  FileLibraryCompareResponse,
  FileLibraryReindexResponse,
  ToolCall,
  AppSettings,
  ImageGenerateRequest,
  ImageGenerateResponse,
  FeatureFlag,
  DocumentChunk,
  ReindexResponse,
  IndexAttachmentResponse,
  ToolDefinition,
  ToolResult,
  UsageSummary,
  PricingRule,
  ExportRequest,
  ImportResult,
  ValidationReport,
  PromptTemplate,
  InterpolateResult,
  AgentRun,
  AgentRunWithSteps,
  StartAgentRunRequest,
  Branch,
  CreateBranchRequest,
  AuthResponse,
  AuthStatus,
  RegisterRequest,
  LoginRequest,
  User,
  WorkspaceMember,
  AddMemberRequest,
  UpdateMemberRoleRequest,
  InstalledPlugin,
  InstallPluginRequest,
  UpdatePluginRequest,
  EvalRun,
  RunEvalRequest,
  ImageSession,
  ImageSessionDetail,
  ImageNodeAsset,
  ImageEditGenerateRequest,
  ImageEditEditRequest,
  ImageEditGenerateResponse,
  ImagePromptEnhanceRequest,
  ImagePromptEnhanceResponse,
  ImageCapabilities,
  MCPServer,
  MCPAuditEvent,
  MCPTool,
  CreateMCPServerRequest,
  UpdateMCPServerRequest,
  ToolPolicy,
  BrowserSession,
  BrowserStatus,
  CrossoverTranslateRequest,
  CrossoverMusicToImageResponse,
  CrossoverImageToMusicResponse,
  CrossoverToVideoResponse,
  Attachment,
  RouterSuggestionsResponse,
  RouterTelemetry,
} from './types';
import type {
  GenerateMusicRequest,
  MusicAsset,
  MusicGenerationDetail,
  MusicGenerationDone,
  MusicGenerationError,
  MusicGenerationProgress,
  MusicModel,
  MusicProviderKey,
  MusicProvidersResponse,
  MusicSession,
  MusicSessionDetail,
} from './types/music';
import type {
  GenerateVideoRequest,
  VideoAsset,
  VideoAssistantRequest,
  VideoEditPlan,
  VideoExportSettings,
  VideoGenerationDetail,
  VideoGenerationValidationResult,
  VideoModel,
  VideoProject,
  VideoProjectDetail,
  VideoProviderInfo,
  VideoProviderKey,
  VideoRendererCapabilities,
  VideoRenderJob,
  VideoSocialVariant,
  VideoStoryboardResponse,
  VideoTimelineDetail,
  VideoTimelineDocument,
  VideoTimelineRecord,
} from './types/video';

// In the Wails desktop build the API runs on a real local HTTP server
// (required for SSE streaming). The Go App.GetAPIBase() binding returns
// the URL (e.g. "http://127.0.0.1:54321/v1").  In normal web mode we
// use the relative path which the Vite proxy forwards to the Go backend.
let BASE_URL = '/v1';

// Resolve a relative /v1/ path to an absolute URL when running inside
// the Wails desktop build (where the API lives on a different origin).
// In normal web mode this is a no-op (BASE_URL is already "/v1").
export function resolveApiUrl(path: string): string {
  if (path.startsWith('/v1/') || path === '/v1') {
    return `${BASE_URL}${path.slice(3)}`;
  }
  return path;
}

// Called once at startup from main.tsx (after Wails runtime is ready).
export async function initAPIBase(): Promise<void> {
  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const wailsGo = (window as any).go;
    if (wailsGo?.main?.App?.GetAPIBase) {
      const url: string = await wailsGo.main.App.GetAPIBase();
      if (url) BASE_URL = url;
    }
  } catch {
    // Not in desktop mode — keep default
  }
}

// Auth token management
let authToken: string | null = localStorage.getItem('omnillm_auth_token');

export function setAuthToken(token: string | null): void {
  authToken = token;
  if (token) {
    localStorage.setItem('omnillm_auth_token', token);
  } else {
    localStorage.removeItem('omnillm_auth_token');
  }
}

export function getAuthToken(): string | null {
  return authToken;
}

// Returns the full URL for downloading an attachment, respecting desktop BASE_URL.
export function attachmentUrl(attachmentId: string): string {
  return `${BASE_URL}/attachments/${attachmentId}/download`;
}

export function musicAssetUrl(assetId: string): string {
  return `${BASE_URL}/music/assets/${encodeURIComponent(assetId)}/download`;
}

export function videoAssetUrl(assetId: string): string {
  return `${BASE_URL}/video/assets/${encodeURIComponent(assetId)}/download`;
}

// Upload a file as an attachment scoped to a conversation.
export async function uploadAttachment(conversationId: string, file: File): Promise<{ id: string }> {
  const formData = new FormData();
  formData.append('file', file);
  const headers: Record<string, string> = {};
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }
  const res = await fetch(`${BASE_URL}/conversations/${encodeURIComponent(conversationId)}/attachments`, {
    method: 'POST',
    headers,
    body: formData,
  });
  if (!res.ok) throw new Error('Upload failed');
  return res.json();
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    // Do NOT set Content-Type for FormData — the browser must set it with the
    // multipart boundary. Only default to JSON for everything else.
    ...(options?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
    ...(options?.headers as Record<string, string>),
  };

  // Attach auth token if available
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `API error: ${res.status}`);
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}

// ---- Health ----

export const api = {
  health: () => apiFetch<{ ok: boolean }>('/health'),
  version: () => apiFetch<{ version: string; commit: string }>('/version'),

  // ---- Conversations ----

  listConversations: (includeArchived = false, workspaceId?: string | null, kind: ConversationKind = 'chat') => {
    let url = `/conversations?include_archived=${includeArchived}&kind=${encodeURIComponent(kind)}`;
    if (workspaceId) url += `&workspace_id=${encodeURIComponent(workspaceId)}`;
    return apiFetch<Conversation[]>(url);
  },

  getConversation: (id: string) =>
    apiFetch<Conversation>(`/conversations/${id}`),

  createConversation: (data: CreateConversationRequest = {}) =>
    apiFetch<Conversation>('/conversations', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateConversation: (id: string, data: UpdateConversationRequest) =>
    apiFetch<Conversation>(`/conversations/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteConversation: (id: string) =>
    apiFetch<void>(`/conversations/${id}`, { method: 'DELETE' }),

  searchConversations: (query: string, kind: ConversationKind = 'chat') =>
    apiFetch<Conversation[]>(`/conversations/search?q=${encodeURIComponent(query)}&kind=${encodeURIComponent(kind)}`),

  // ---- Messages ----

  listMessages: (conversationId: string) =>
    apiFetch<Message[]>(`/conversations/${conversationId}/messages`),

  sendMessage: (conversationId: string, data: SendMessageRequest) =>
    apiFetch<Message>(`/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  deleteMessage: (conversationId: string, messageId: string) =>
    apiFetch<void>(`/conversations/${conversationId}/messages/${messageId}`, {
      method: 'DELETE',
    }),

  editMessage: (conversationId: string, messageId: string, content: string) =>
    apiFetch<{ status: string }>(`/conversations/${conversationId}/messages/${messageId}`, {
      method: 'PATCH',
      body: JSON.stringify({ content }),
    }),

  // ---- Image Generation ----

  generateImage: (conversationId: string, data: ImageGenerateRequest) =>
    apiFetch<ImageGenerateResponse>(`/conversations/${conversationId}/messages/image`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  generateTitle: (conversationId: string) =>
    apiFetch<{ title: string }>(`/conversations/${conversationId}/title`, {
      method: 'POST',
    }),

  // SSE streaming: returns an EventSource-like interface
  streamMessage: (
    conversationId: string,
    data: SendMessageRequest,
    callbacks: {
      onToken: (content: string) => void;
      onThinking?: (content: string) => void;
      onStart?: (data: { message_id: string; user_message_id: string }) => void;
      onDone?: (data: { message_id: string; provider: string; model: string; latency_ms: number; content?: string; web_search?: boolean; sources?: WebSearchResult[]; file_search?: boolean; file_sources?: FileSearchResult[]; browser_tool?: boolean; browser_navigated_urls?: string[]; tool_calls?: ToolCall[]; browser_tool_results?: ToolResult[]; thinking?: string; cost?: number; image_generation?: boolean; router?: RouterTelemetry }) => void;
      onError?: (error: string) => void;
      onWebSearch?: (data: { tool_call: ToolCall; status: string }) => void;
      onWebSearchResults?: (data: { query: string; results: WebSearchResult[] }) => void;
      onFileSearch?: (data: { status: string; scope?: string; query?: string; count?: number }) => void;
      onFileSearchResults?: (data: { results: FileSearchResult[] }) => void;
      onImageGeneration?: (data: { status: string; prompt?: string; count?: number; error?: string }) => void;
      onRAGIndexing?: (data: { status: string; detail?: string }) => void;
      onURLContext?: (data: { status: string; url?: string; kind?: string; url_count?: number; source_count?: number; used_rag?: boolean }) => void;
      onBrowserDownloading?: (data: { progress_percent: number }) => void;
      onBrowserNavigating?: (data: { url: string; session_id?: string }) => void;
      onBrowserScreenshotDone?: (data: { url: string; session_id?: string }) => void;
      onBrowserInteractDone?: (data: { action: string; selector: string; session_id?: string }) => void;
      onURLContextBrowserFallback?: (data: { url: string }) => void;
    }
  ) => {
    const controller = new AbortController();
    const INACTIVITY_TIMEOUT_MS = 60_000; // 60s inactivity timeout

    fetch(`${BASE_URL}/conversations/${conversationId}/messages/stream`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          const body = await response.json().catch(() => ({ error: response.statusText }));
          callbacks.onError?.(body.error || `Stream error: ${response.status}`);
          return;
        }

        const reader = response.body?.getReader();
        if (!reader) {
          callbacks.onError?.('No readable stream');
          return;
        }

        const decoder = new TextDecoder();
        let buffer = '';
        let currentEvent = 'token';
        let receivedTerminal = false;
        let startMessageId = ''; // Track message_id from start event
        let inactivityTimer: ReturnType<typeof setTimeout> | null = null;

        const resetInactivityTimer = () => {
          if (inactivityTimer) clearTimeout(inactivityTimer);
          inactivityTimer = setTimeout(() => {
            if (!receivedTerminal) {
              callbacks.onError?.('Stream timed out — no data received for 60 seconds');
              controller.abort();
            }
          }, INACTIVITY_TIMEOUT_MS);
        };

        const processEvent = (eventType: string, dataStr: string) => {
          try {
            const payload = JSON.parse(dataStr);
            switch (eventType) {
              case 'start':
                startMessageId = payload.message_id || '';
                callbacks.onStart?.(payload);
                break;
              case 'token':
                callbacks.onToken(payload.content || '');
                break;
              case 'thinking':
                callbacks.onThinking?.(payload.content || '');
                break;
              case 'done':
                receivedTerminal = true;
                callbacks.onDone?.(payload);
                break;
              case 'web_search':
                callbacks.onWebSearch?.(payload);
                break;
              case 'web_search_results':
                callbacks.onWebSearchResults?.(payload);
                break;
              case 'file_search':
                callbacks.onFileSearch?.(payload);
                break;
              case 'file_search_results':
                callbacks.onFileSearchResults?.(payload);
                break;
              case 'image_generation':
                callbacks.onImageGeneration?.(payload);
                break;
              case 'url_context':
                callbacks.onURLContext?.(payload);
                break;
              case 'url_context_browser_fallback':
                callbacks.onURLContextBrowserFallback?.(payload);
                break;
              case 'browser_downloading':
                callbacks.onBrowserDownloading?.(payload);
                break;
              case 'browser_navigating':
                callbacks.onBrowserNavigating?.(payload);
                break;
              case 'browser_screenshot_done':
                callbacks.onBrowserScreenshotDone?.(payload);
                break;
              case 'browser_interact_done':
                callbacks.onBrowserInteractDone?.(payload);
                break;
              case 'rag_indexing':
                callbacks.onRAGIndexing?.(payload);
                break;
              case 'error':
                receivedTerminal = true;
                callbacks.onError?.(payload.error || 'Unknown error');
                break;
            }
          } catch {
            // skip malformed JSON
          }
        };

        try {
          resetInactivityTimer();

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            resetInactivityTimer();
            buffer += decoder.decode(value, { stream: true });

            // Parse SSE: split on double newlines for complete events
            const lines = buffer.split('\n');
            buffer = lines.pop() || '';

            let pendingData = '';

            for (const line of lines) {
              const trimmed = line.trim();
              if (trimmed === '') {
                // Empty line = end of event block, dispatch if we have data
                if (pendingData) {
                  processEvent(currentEvent, pendingData);
                  pendingData = '';
                  currentEvent = 'token'; // reset to default
                }
              } else if (trimmed.startsWith('event:')) {
                currentEvent = trimmed.slice(6).trim();
              } else if (trimmed.startsWith('data:')) {
                pendingData = trimmed.slice(5).trim();
              }
            }

            // Handle case where last event in buffer doesn't end with double newline
            if (pendingData) {
              processEvent(currentEvent, pendingData);
              pendingData = '';
              currentEvent = 'token';
            }
          }
        } finally {
          // Ensure terminal state — if stream closed without done/error, emit synthetic done
          if (inactivityTimer) clearTimeout(inactivityTimer);
          if (!receivedTerminal) {
            if (startMessageId) {
              // We received a start event — finalize with the known message_id
              callbacks.onDone?.({ message_id: startMessageId, provider: '', model: '', latency_ms: 0 });
            } else {
              // No start event — stream closed before any message was created; signal error
              callbacks.onError?.('Stream closed unexpectedly');
            }
          }
        }
      })
      .catch((err) => {
        if (err.name !== 'AbortError') {
          callbacks.onError?.(err.message);
        }
      });

    return { abort: () => controller.abort() };
  },

  // ---- Providers ----

  listProviders: () => apiFetch<ProviderProfile[]>('/providers'),

  createProvider: (data: CreateProviderRequest) =>
    apiFetch<ProviderProfile>('/providers', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateProvider: (id: string, data: UpdateProviderRequest) =>
    apiFetch<ProviderProfile>(`/providers/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteProvider: (id: string) =>
    apiFetch<void>(`/providers/${id}`, { method: 'DELETE' }),

  getProviderImageCapabilities: (providerId: string) =>
    apiFetch<ImageCapabilities>(`/providers/${providerId}/image-capabilities`),

  // ---- Web Search ----

  webSearch: (data: WebSearchRequest) =>
    apiFetch<WebSearchResponse>('/websearch', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  // ---- Settings ----

  getSettings: () => apiFetch<AppSettings>('/settings'),

  updateSettings: (data: Partial<AppSettings>) =>
    apiFetch<AppSettings>('/settings', {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  getRouterSuggestions: (provider: string) =>
    apiFetch<RouterSuggestionsResponse>(`/settings/router/suggestions?provider=${encodeURIComponent(provider)}`),

  // ---- Ollama ----

  /** Fetch installed models from an Ollama instance (proxied through backend
   *  so the Wails desktop build doesn't need cross-origin access to Ollama). */
  fetchOllamaModels: async (baseUrl?: string): Promise<string[]> => {
    const url = (baseUrl || 'http://localhost:11434').replace(/\/+$/, '');
    try {
      return await apiFetch<string[]>(`/providers/ollama/models?base_url=${encodeURIComponent(url)}`);
    } catch {
      return [];
    }
  },

  fetchOpenRouterModels: async (providerId: string): Promise<Array<{ id: string; name: string }>> => {
    try {
      return await apiFetch<Array<{ id: string; name: string }>>(`/providers/openrouter/models?provider_id=${encodeURIComponent(providerId)}`);
    } catch {
      return [];
    }
  },

  // ---- Attachments ----

  listAttachments: (conversationId: string) =>
    apiFetch<import('./types').Attachment[]>(`/conversations/${conversationId}/attachments`),

  uploadAttachment: async (conversationId: string, file: File): Promise<import('./types').Attachment> => {
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(`${BASE_URL}/conversations/${conversationId}/attachments`, {
      method: 'POST',
      body: form,
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(body.error || `Upload failed: ${res.status}`);
    }
    return res.json();
  },

  downloadAttachmentUrl: (attachmentId: string) =>
    `${BASE_URL}/attachments/${attachmentId}/download`,

  deleteAttachment: (attachmentId: string) =>
    apiFetch<void>(`/attachments/${attachmentId}`, { method: 'DELETE' }),

  // ---- Feature Flags ----

  listFeatures: () => apiFetch<FeatureFlag[]>('/features'),

  updateFeature: (key: string, enabled: boolean, metadata?: string) =>
    apiFetch<FeatureFlag[]>(`/features/${encodeURIComponent(key)}`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled, ...(metadata !== undefined && { metadata }) }),
    }),

  // ---- RAG ----

  listChunks: (conversationId: string) =>
    apiFetch<DocumentChunk[]>(`/conversations/${conversationId}/chunks`),

  listAttachmentChunks: (attachmentId: string) =>
    apiFetch<DocumentChunk[]>(`/attachments/${attachmentId}/chunks`),

  reindexConversation: (conversationId: string) =>
    apiFetch<ReindexResponse>(`/conversations/${conversationId}/reindex`, {
      method: 'POST',
    }),

  indexAttachment: (attachmentId: string) =>
    apiFetch<IndexAttachmentResponse>(`/attachments/${attachmentId}/index`, {
      method: 'POST',
    }),

  reindexAll: () =>
    apiFetch<{ conversations_dropped: number; note: string }>('/rag/reindex-all', {
      method: 'POST',
    }),

  // ---- Tools ----

  listTools: () =>
    apiFetch<ToolDefinition[]>('/tools'),

  updateToolPermission: (toolName: string, policy: string) =>
    apiFetch<{ tool_name: string; policy: string }>(`/tools/${toolName}/permission`, {
      method: 'PATCH',
      body: JSON.stringify({ policy }),
    }),

  executeTool: (name: string, args: Record<string, unknown>) =>
    apiFetch<ToolResult>('/tools/execute', {
      method: 'POST',
      body: JSON.stringify({ name, arguments: args }),
    }),

  // ---- Analytics ----

  getUsage: (period: string = 'month') =>
    apiFetch<UsageSummary>(`/analytics/usage?period=${period}`),

  getConversationUsage: (conversationId: string, period: string = 'all') =>
    apiFetch<UsageSummary>(`/analytics/conversations/${conversationId}/usage?period=${period}`),

  listPricing: () =>
    apiFetch<PricingRule[]>('/pricing'),

  upsertPricing: (rule: Omit<PricingRule, 'created_at'> & { id?: string }) =>
    apiFetch<PricingRule>('/pricing', {
      method: 'PUT',
      body: JSON.stringify(rule),
    }),

  deletePricing: (id: string) =>
    apiFetch<void>(`/pricing/${id}`, {
      method: 'DELETE',
    }),

  // --- Import/Export ---

  exportBundle: async (options: ExportRequest = {}): Promise<Blob> => {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (authToken) headers['Authorization'] = `Bearer ${authToken}`;
    const res = await fetch(`${BASE_URL}/export`, {
      method: 'POST',
      headers,
      body: JSON.stringify(options),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || 'Export failed');
    }
    return res.blob();
  },

  importBundle: async (file: File, strategy: 'skip' | 'overwrite' = 'skip'): Promise<ImportResult> => {
    const form = new FormData();
    form.append('file', file);
    form.append('strategy', strategy);
    const headers: Record<string, string> = {};
    if (authToken) headers['Authorization'] = `Bearer ${authToken}`;
    const res = await fetch(`${BASE_URL}/import`, {
      method: 'POST',
      headers,
      body: form,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || 'Import failed');
    }
    return res.json();
  },

  validateBundle: async (file: File): Promise<ValidationReport> => {
    const form = new FormData();
    form.append('file', file);
    const headers: Record<string, string> = {};
    if (authToken) headers['Authorization'] = `Bearer ${authToken}`;
    const res = await fetch(`${BASE_URL}/import/validate`, {
      method: 'POST',
      headers,
      body: form,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || 'Validation failed');
    }
    return res.json();
  },
};

// ── Prompt Templates ──────────────────────────────────────

export const templateApi = {
  list: (category?: string) => {
    const q = category ? `?category=${encodeURIComponent(category)}` : '';
    return apiFetch<PromptTemplate[]>(`/templates${q}`);
  },

  get: (id: string) =>
    apiFetch<PromptTemplate>(`/templates/${id}`),

  create: (data: Partial<PromptTemplate>) =>
    apiFetch<PromptTemplate>('/templates', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<PromptTemplate>) =>
    apiFetch<PromptTemplate>(`/templates/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    apiFetch<{ deleted: boolean }>(`/templates/${id}`, { method: 'DELETE' }),

  interpolate: (id: string, values: Record<string, string>) =>
    apiFetch<InterpolateResult>(`/templates/${id}/interpolate`, {
      method: 'POST',
      body: JSON.stringify({ values }),
    }),
};

// ── Phase 6: Agent Mode ──────────────────────────────

export const agentApi = {
  /**
   * Start an agent run and stream SSE events. Returns an AbortController so
   * the caller can cancel the in-flight request.
   */
  startRun: (
    conversationId: string,
    req: StartAgentRunRequest,
    onEvent: (event: { type: string; data: string }) => void,
  ): { promise: Promise<void>; abort: AbortController } => {
    const controller = new AbortController();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (authToken) {
      headers['Authorization'] = `Bearer ${authToken}`;
    }

    const promise = new Promise<void>((resolve, reject) => {
      fetch(`${BASE_URL}/conversations/${conversationId}/agent/run`, {
        method: 'POST',
        headers,
        body: JSON.stringify(req),
        signal: controller.signal,
      })
        .then((res) => {
          if (!res.ok) {
            return res.json().then((b) => reject(new Error(b.error || 'Agent run failed')));
          }
          const reader = res.body?.getReader();
          if (!reader) return reject(new Error('No response body'));
          const decoder = new TextDecoder();
          let buffer = '';

          function pump(): Promise<void> {
            return reader!.read().then(({ done, value }) => {
              if (done) { resolve(); return; }
              buffer += decoder.decode(value, { stream: true });
              const lines = buffer.split('\n');
              buffer = lines.pop() || '';
              let eventType = '';
              for (const line of lines) {
                if (line.startsWith('event: ')) eventType = line.slice(7).trim();
                else if (line.startsWith('data: ')) {
                  onEvent({ type: eventType, data: line.slice(6) });
                }
              }
              return pump();
            });
          }
          return pump();
        })
        .catch((err) => {
          if (err?.name === 'AbortError') {
            resolve(); // graceful abort — don't treat as error
          } else {
            reject(err);
          }
        });
    });

    return { promise, abort: controller };
  },

  listRuns: (conversationId: string) =>
    apiFetch<AgentRun[]>(`/conversations/${conversationId}/agent/runs`),

  getRun: (runId: string) =>
    apiFetch<AgentRunWithSteps>(`/agent/runs/${runId}`),

  approveStep: (runId: string, stepId: string, approved: boolean) =>
    apiFetch<{ ok: boolean }>(`/agent/runs/${runId}/approve/${stepId}`, {
      method: 'POST',
      body: JSON.stringify({ approved }),
    }),

  cancelRun: (runId: string) =>
    apiFetch<{ cancelled: boolean }>(`/agent/runs/${runId}/cancel`, {
      method: 'POST',
    }),

  resumeRun: (runId: string) =>
    apiFetch<{ resumed: boolean }>(`/agent/runs/${runId}/resume`, {
      method: 'POST',
    }),
};

// ── Phase 7: Conversation Branching ──────────────

export const branchApi = {
  list: (conversationId: string) =>
    apiFetch<Branch[]>(`/conversations/${conversationId}/branches`),

  create: (conversationId: string, req: CreateBranchRequest) =>
    apiFetch<Branch>(`/conversations/${conversationId}/branches`, {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  rename: (conversationId: string, branchId: string, name: string) =>
    apiFetch<{ ok: boolean }>(`/conversations/${conversationId}/branches/${branchId}`, {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    }),

  delete: (conversationId: string, branchId: string) =>
    apiFetch<{ deleted: boolean }>(`/conversations/${conversationId}/branches/${branchId}`, {
      method: 'DELETE',
    }),

  listMessages: (conversationId: string, branchId?: string) =>
    apiFetch<Message[]>(
      `/conversations/${conversationId}/messages/branch${branchId ? `?branch=${branchId}` : ''}`,
    ),
};

// ── Search ────────────────────────────────────────────────────────────────

export const searchApi = {
  search: (
    query: string,
    mode: import('./types').SearchMode = 'hybrid',
    limit = 20,
    conversationId?: string,
    kind: ConversationKind = 'chat',
  ) => {
    const params = new URLSearchParams({ q: query, mode, limit: String(limit) });
    if (conversationId) params.set('conversation_id', conversationId);
    params.set('kind', kind);
    return apiFetch<import('./types').SearchResponse>(`/search?${params}`);
  },

  reindex: () =>
    apiFetch<{ status: import('./types').ReindexStatus; error?: string }>('/search/reindex', {
      method: 'POST',
    }),
};

// ── File Library ──────────────────────────────────────────────────────────

export const fileLibraryApi = {
  list: (scope: 'conversation' | 'workspace' | 'global' | 'all' = 'all', query?: string) => {
    const params = new URLSearchParams();
    params.set('scope', scope);
    if (query && query.trim()) params.set('query', query.trim());
    return apiFetch<LibraryFile[]>(`/file-library/files?${params.toString()}`);
  },

  fetch: (fileId: string, includeFullText = false) =>
    apiFetch<{ file?: LibraryFile; full_text?: string }>(`/file-library/files/${fileId}?include_full_text=${includeFullText ? 'true' : 'false'}`),

  ingest: (data: {
    attachment_id: string;
    display_name?: string;
    scope?: 'conversation' | 'workspace' | 'global';
    conversation_id?: string;
    workspace_id?: string;
    metadata?: Record<string, unknown>;
  }) =>
    apiFetch<LibraryFile>('/file-library/files/ingest', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  search: (data: {
    query: string;
    scope?: 'auto' | 'conversation' | 'workspace' | 'global' | 'all';
    conversation_id?: string;
    workspace_id?: string;
    top_k?: number;
    file_type_filter?: string[];
    source_filter?: string[];
    require_citations?: boolean;
  }) =>
    apiFetch<FileSearchResponse>('/file-library/search', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  summarize: (data: {
    library_file_ids: string[];
    query?: string;
    summary_style?: 'brief' | 'detailed' | 'executive' | 'technical' | 'qa';
    max_chars_per_file?: number;
    conversation_id?: string;
  }) =>
    apiFetch<FileLibrarySummaryResponse>('/file-library/summarize', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  compare: (data: {
    library_file_ids: string[];
    comparison_goal?: string;
    output_format?: 'markdown' | 'table' | 'executive_summary';
    max_chars_per_file?: number;
    conversation_id?: string;
  }) =>
    apiFetch<FileLibraryCompareResponse>('/file-library/compare', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (fileId: string, data: { display_name?: string; scope?: 'conversation' | 'workspace' | 'global'; metadata?: Record<string, unknown> }) =>
    apiFetch<LibraryFile>(`/file-library/files/${fileId}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (fileId: string, hard = false) =>
    apiFetch<{ deleted: boolean; hard: boolean }>(`/file-library/files/${fileId}?hard=${hard ? 'true' : 'false'}`, {
      method: 'DELETE',
    }),

  reindex: (fileId: string) =>
    apiFetch<FileLibraryReindexResponse>(`/file-library/files/${fileId}/reindex`, {
      method: 'POST',
    }),
};

// ── Browser ───────────────────────────────────────────────────────────────

export const browserApi = {
  status: () => apiFetch<BrowserStatus>('/browser/status'),

  listSessions: () => apiFetch<BrowserSession[]>('/browser/sessions'),

  closeSession: (id: string) =>
    apiFetch<void>(`/browser/sessions/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),
};

// ── Workspaces ────────────────────────────────────────────────────────────

export const workspaceApi = {
  list: () => apiFetch<import('./types').Workspace[]>('/workspaces'),

  get: (id: string) => apiFetch<import('./types').Workspace>(`/workspaces/${id}`),

  create: (req: import('./types').CreateWorkspaceRequest) =>
    apiFetch<import('./types').Workspace>('/workspaces', {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  update: (id: string, req: import('./types').UpdateWorkspaceRequest) =>
    apiFetch<import('./types').Workspace>(`/workspaces/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(req),
    }),

  delete: (id: string) =>
    apiFetch<{ deleted: boolean }>(`/workspaces/${id}`, {
      method: 'DELETE',
    }),

  getStats: (id: string) =>
    apiFetch<import('./types').WorkspaceStats>(`/workspaces/${id}/stats`),
};

// ── Auth ──────────────────────────────────────────────────────────────────

export const authApi = {
  register: (req: RegisterRequest) =>
    apiFetch<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  login: (req: LoginRequest) =>
    apiFetch<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  logout: () =>
    apiFetch<{ ok: boolean }>('/auth/logout', {
      method: 'POST',
    }),

  status: () => apiFetch<AuthStatus>('/auth/status'),

  me: () => apiFetch<User>('/users/me'),
};

// ── Workspace Members ─────────────────────────────────────────────────────

export const workspaceMemberApi = {
  list: (workspaceId: string) =>
    apiFetch<WorkspaceMember[]>(`/workspaces/${workspaceId}/members`),

  add: (workspaceId: string, req: AddMemberRequest) =>
    apiFetch<WorkspaceMember>(`/workspaces/${workspaceId}/members`, {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  updateRole: (workspaceId: string, userId: string, req: UpdateMemberRoleRequest) =>
    apiFetch<WorkspaceMember>(`/workspaces/${workspaceId}/members/${userId}`, {
      method: 'PATCH',
      body: JSON.stringify(req),
    }),

  remove: (workspaceId: string, userId: string) =>
    apiFetch<void>(`/workspaces/${workspaceId}/members/${userId}`, {
      method: 'DELETE',
    }),
};

// ── Plugins ───────────────────────────────────────────────────────────────

export const pluginApi = {
  list: () => apiFetch<InstalledPlugin[]>('/plugins'),

  install: (req: InstallPluginRequest) =>
    apiFetch<InstalledPlugin>('/plugins', {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  update: (name: string, req: UpdatePluginRequest) =>
    apiFetch<InstalledPlugin>(`/plugins/${name}`, {
      method: 'PATCH',
      body: JSON.stringify(req),
    }),

  uninstall: (name: string) =>
    apiFetch<void>(`/plugins/${name}`, {
      method: 'DELETE',
    }),
};

// ── MCP Servers ───────────────────────────────────────────────────────────

export const mcpApi = {
  listAudit: (params: { serverId?: string; limit?: number } = {}) => {
    const search = new URLSearchParams();
    if (params.serverId) search.set('server_id', params.serverId);
    if (params.limit) search.set('limit', String(params.limit));
    const suffix = search.toString() ? `?${search.toString()}` : '';
    return apiFetch<MCPAuditEvent[]>(`/mcp/audit${suffix}`);
  },

  listServers: () =>
    apiFetch<MCPServer[]>('/mcp/servers'),

  getServer: (id: string) =>
    apiFetch<MCPServer>(`/mcp/servers/${id}`),

  createServer: (data: CreateMCPServerRequest) =>
    apiFetch<MCPServer>('/mcp/servers', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateServer: (id: string, data: UpdateMCPServerRequest) =>
    apiFetch<MCPServer>(`/mcp/servers/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteServer: (id: string) =>
    apiFetch<void>(`/mcp/servers/${id}`, {
      method: 'DELETE',
    }),

  testServer: (id: string) =>
    apiFetch<{ ok: boolean; tools?: MCPTool[]; error?: string }>(`/mcp/servers/${id}/test`, {
      method: 'POST',
    }),

  startServer: (id: string) =>
    apiFetch<MCPServer>(`/mcp/servers/${id}/start`, {
      method: 'POST',
    }),

  stopServer: (id: string) =>
    apiFetch<MCPServer>(`/mcp/servers/${id}/stop`, {
      method: 'POST',
    }),

  restartServer: (id: string) =>
    apiFetch<MCPServer>(`/mcp/servers/${id}/restart`, {
      method: 'POST',
    }),

  refreshTools: (id: string) =>
    apiFetch<MCPTool[]>(`/mcp/servers/${id}/refresh`, {
      method: 'POST',
    }),

  listTools: (id: string) =>
    apiFetch<MCPTool[]>(`/mcp/servers/${id}/tools`),

  updateToolPolicy: (serverId: string, toolName: string, policy: ToolPolicy) =>
    apiFetch<{ tool_name: string; policy: ToolPolicy }>(`/mcp/servers/${serverId}/tools/${toolName}`, {
      method: 'PATCH',
      body: JSON.stringify({ policy }),
    }),
};

// ── Evaluation Harness ────────────────────────────────────────────────────

export const evalApi = {
  run: (req: RunEvalRequest) =>
    apiFetch<EvalRun>('/eval/run', {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  listRuns: (suite?: string) =>
    apiFetch<EvalRun[]>(`/eval/runs${suite ? `?suite=${encodeURIComponent(suite)}` : ''}`),

  getRun: (id: string) => apiFetch<EvalRun>(`/eval/runs/${id}`),

  deleteRun: (id: string) =>
    apiFetch<void>(`/eval/runs/${id}`, {
      method: 'DELETE',
    }),
};

// ── Image Edit Sessions ────────────────────────────────────────────────────

export const imageSessionApi = {
  listAll: () =>
    apiFetch<ImageSession[]>('/images/sessions'),

  create: (data: { title?: string }) =>
    apiFetch<ImageSession>('/images/sessions', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  get: (conversationId: string, sessionId: string) =>
    apiFetch<ImageSessionDetail>(`/conversations/${conversationId}/images/sessions/${sessionId}`),

  list: (conversationId: string) =>
    apiFetch<ImageSession[]>(`/conversations/${conversationId}/images/sessions`),

  delete: (conversationId: string, sessionId: string) =>
    apiFetch<{ deleted: boolean }>(`/conversations/${conversationId}/images/sessions/${sessionId}`, {
      method: 'DELETE',
    }),

  rename: (conversationId: string, sessionId: string, title: string) =>
    apiFetch<ImageSession>(`/conversations/${conversationId}/images/sessions/${sessionId}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    }),

  generate: (conversationId: string, sessionId: string, data: ImageEditGenerateRequest) =>
    apiFetch<ImageEditGenerateResponse>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/generate`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  edit: (conversationId: string, sessionId: string, data: ImageEditEditRequest) =>
    apiFetch<ImageEditGenerateResponse>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/edit`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  enhancePrompt: (conversationId: string, sessionId: string, data: ImagePromptEnhanceRequest) =>
    apiFetch<ImagePromptEnhanceResponse>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/enhance-prompt`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  importAttachment: (
    conversationId: string,
    sessionId: string,
    data: { attachment_id: string; instruction?: string },
  ) =>
    apiFetch<ImageEditGenerateResponse>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/import`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  uploadMask: (conversationId: string, sessionId: string, data: { node_id: string; mask_data: string; stroke_json?: string }) =>
    apiFetch<{ mask_id: string; attachment_id: string }>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/mask`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  getAssets: (conversationId: string, sessionId: string, nodeId?: string) =>
    apiFetch<ImageNodeAsset[]>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/assets${nodeId ? `?node_id=${nodeId}` : ''}`,
    ),

  getSessionAssets: (conversationId: string, sessionId: string, opts?: { types?: string[]; sort?: 'created_at_asc' | 'created_at_desc' }) => {
    const params = new URLSearchParams({ scope: 'session' });
    if (opts?.types) opts.types.forEach((t) => params.append('type', t));
    if (opts?.sort) params.set('sort', opts.sort);
    return apiFetch<ImageNodeAsset[]>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/assets?${params.toString()}`,
    );
  },

  deleteAsset: (conversationId: string, sessionId: string, assetId: string) =>
    apiFetch<{ status: string }>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/assets/${assetId}`,
      { method: 'DELETE' },
    ),

  selectVariant: (conversationId: string, sessionId: string, nodeId: string, assetId: string) =>
    apiFetch<{ ok: boolean }>(
      `/conversations/${conversationId}/images/sessions/${sessionId}/nodes/${nodeId}/select`,
      { method: 'PUT', body: JSON.stringify({ asset_id: assetId }) },
    ),
};

// ── Music Studio ──────────────────────────────────────────────────────────

export const musicApi = {
  providers: () =>
    apiFetch<MusicProvidersResponse>('/music/providers'),

  listModels: (provider: MusicProviderKey) =>
    apiFetch<MusicModel[]>(`/music/models?provider=${encodeURIComponent(provider)}`),

  refreshModels: (provider: MusicProviderKey) =>
    apiFetch<MusicModel[]>(`/music/models/refresh?provider=${encodeURIComponent(provider)}`, {
      method: 'POST',
    }),

  listSessions: () =>
    apiFetch<MusicSession[]>('/music/sessions'),

  createSession: (data: { title?: string; provider?: MusicProviderKey; model?: string } = {}) =>
    apiFetch<MusicSession>('/music/sessions', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  getSession: (sessionId: string) =>
    apiFetch<MusicSessionDetail>(`/music/sessions/${encodeURIComponent(sessionId)}`),

  updateSession: (sessionId: string, data: { title?: string; provider?: MusicProviderKey; model?: string }) =>
    apiFetch<MusicSession>(`/music/sessions/${encodeURIComponent(sessionId)}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteSession: (sessionId: string) =>
    apiFetch<{ deleted: boolean }>(`/music/sessions/${encodeURIComponent(sessionId)}`, {
      method: 'DELETE',
    }),

  listGenerations: (sessionId: string) =>
    apiFetch<MusicGenerationDetail[]>(`/music/sessions/${encodeURIComponent(sessionId)}/generations`),

  getGeneration: (generationId: string) =>
    apiFetch<MusicGenerationDetail>(`/music/generations/${encodeURIComponent(generationId)}`),

  branchGeneration: (generationId: string) =>
    apiFetch<{
      parent_id: string;
      session_id: string;
      prompt: string;
      assembled_prompt: string;
      provider: MusicProviderKey;
      model: string;
    }>(`/music/generations/${encodeURIComponent(generationId)}/branch`, {
      method: 'POST',
    }),

  getAsset: (assetId: string) =>
    apiFetch<MusicAsset>(`/music/assets/${encodeURIComponent(assetId)}`),

  deleteAsset: (assetId: string) =>
    apiFetch<{ deleted: boolean }>(`/music/assets/${encodeURIComponent(assetId)}`, {
      method: 'DELETE',
    }),

  downloadUrl: (assetId: string) => musicAssetUrl(assetId),

  generate: (
    req: GenerateMusicRequest,
    callbacks: {
      onStarted?: (data: MusicGenerationProgress) => void;
      onProgress?: (data: MusicGenerationProgress) => void;
      onDone?: (data: MusicGenerationDone) => void;
      onError?: (data: MusicGenerationError) => void;
    },
  ) => {
    const controller = new AbortController();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (authToken) {
      headers.Authorization = `Bearer ${authToken}`;
    }

    fetch(`${BASE_URL}/music/generations`, {
      method: 'POST',
      headers,
      body: JSON.stringify(req),
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          const body = await response.json().catch(() => ({ error: response.statusText }));
          callbacks.onError?.({ error: body.error || `Music generation failed: ${response.status}` });
          return;
        }
        const reader = response.body?.getReader();
        if (!reader) {
          callbacks.onError?.({ error: 'No readable stream returned' });
          return;
        }
        const decoder = new TextDecoder();
        let buffer = '';
        let currentEvent = 'music_generation_progress';
        let pendingData = '';
        let terminalReceived = false;

        const dispatch = (eventType: string, dataStr: string) => {
          try {
            const payload = JSON.parse(dataStr);
            if (eventType === 'music_generation_started') {
              callbacks.onStarted?.(payload);
            } else if (eventType === 'music_generation_done') {
              terminalReceived = true;
              callbacks.onDone?.(payload);
            } else if (eventType === 'music_generation_error') {
              terminalReceived = true;
              callbacks.onError?.(payload);
            } else {
              callbacks.onProgress?.(payload);
            }
          } catch {
            // Ignore malformed SSE frames.
          }
        };

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';
          for (const rawLine of lines) {
            const line = rawLine.trim();
            if (!line) {
              if (pendingData) {
                dispatch(currentEvent, pendingData);
                pendingData = '';
                currentEvent = 'music_generation_progress';
              }
              continue;
            }
            if (line.startsWith('event:')) {
              currentEvent = line.slice(6).trim();
            } else if (line.startsWith('data:')) {
              pendingData += line.slice(5).trim();
            }
          }
        }
        if (pendingData) {
          dispatch(currentEvent, pendingData);
        }
        if (!terminalReceived) {
          callbacks.onError?.({ error: 'Music generation stream closed unexpectedly' });
        }
      })
      .catch((err) => {
        if (err?.name !== 'AbortError') {
          callbacks.onError?.({ error: err?.message || 'Music generation failed' });
        }
      });

    return { abort: () => controller.abort() };
  },

  /** Copy a music asset's bytes into a new attachment under the target conversation. */
  attachToConversation: (assetId: string, conversationId: string) =>
    apiFetch<Attachment>(
      `/music/assets/${encodeURIComponent(assetId)}/attach-to-conversation`,
      { method: 'POST', body: JSON.stringify({ conversation_id: conversationId }) },
    ),
};

// ── Video Studio ──────────────────────────────────────────────────────────

export const videoApi = {
  providers: () =>
    apiFetch<VideoProviderInfo[]>('/video/providers'),

  listModels: (provider: VideoProviderKey) =>
    apiFetch<VideoModel[]>(`/video/models?provider=${encodeURIComponent(provider)}`),

  refreshModels: (provider: VideoProviderKey) =>
    apiFetch<VideoModel[]>(`/video/models/refresh?provider=${encodeURIComponent(provider)}`, {
      method: 'POST',
    }),

  listProjects: () =>
    apiFetch<VideoProject[]>('/video/projects'),

  createProject: (data: { title?: string; provider?: VideoProviderKey; model?: string; width?: number; height?: number; fps?: number; aspect_ratio?: string } = {}) =>
    apiFetch<VideoProject>('/video/projects', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  getProject: (projectId: string) =>
    apiFetch<VideoProjectDetail>(`/video/projects/${encodeURIComponent(projectId)}`),

  updateProject: (projectId: string, data: { title?: string; provider?: VideoProviderKey; model?: string; width?: number; height?: number; fps?: number; aspect_ratio?: string }) =>
    apiFetch<VideoProject>(`/video/projects/${encodeURIComponent(projectId)}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteProject: (projectId: string) =>
    apiFetch<{ deleted: boolean }>(`/video/projects/${encodeURIComponent(projectId)}`, {
      method: 'DELETE',
    }),

  listGenerations: (projectId: string) =>
    apiFetch<VideoGenerationDetail[]>(`/video/projects/${encodeURIComponent(projectId)}/generations`),

  getGeneration: (generationId: string) =>
    apiFetch<VideoGenerationDetail>(`/video/generations/${encodeURIComponent(generationId)}`),

  branchGeneration: (generationId: string) =>
    apiFetch<{
      parent_id: string;
      project_id: string;
      prompt: string;
      enhanced_prompt?: string;
      negative_prompt?: string;
      provider: VideoProviderKey;
      model: string;
      settings_json?: string;
      input_asset_ids_json?: string;
      input_assets_json?: string;
    }>(`/video/generations/${encodeURIComponent(generationId)}/branch`, {
      method: 'POST',
    }),

  sendGenerationToTimeline: (generationId: string) =>
    apiFetch<{ project_id: string; asset_id?: string; timeline: VideoTimelineRecord; document: VideoTimelineDocument }>(
      `/video/generations/${encodeURIComponent(generationId)}/send-to-timeline`,
      { method: 'POST' },
    ),

  listAssets: (projectId: string) =>
    apiFetch<VideoAsset[]>(`/video/projects/${encodeURIComponent(projectId)}/assets`),

  importExternalAsset: (projectId: string, data: {
    source_studio: string;
    source_id: string;
    source_type?: string;
    kind?: VideoAsset['kind'];
    file_name?: string;
    mime_type?: string;
    size_bytes?: number;
    duration_ms?: number;
    width?: number;
    height?: number;
    fps?: number;
    metadata?: Record<string, unknown>;
  }) =>
    apiFetch<VideoAsset>(`/video/projects/${encodeURIComponent(projectId)}/assets/import`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  getAsset: (assetId: string) =>
    apiFetch<VideoAsset>(`/video/assets/${encodeURIComponent(assetId)}`),

  uploadAsset: (projectId: string, file: File): Promise<VideoAsset> => {
    const form = new FormData();
    form.append('file', file);
    return apiFetch<VideoAsset>(`/video/projects/${encodeURIComponent(projectId)}/assets/upload`, {
      method: 'POST',
      body: form,
    });
  },

  deleteAsset: (assetId: string) =>
    apiFetch<{ deleted: boolean }>(`/video/assets/${encodeURIComponent(assetId)}`, {
      method: 'DELETE',
    }),

  attachAssetToConversation: (assetId: string, conversationId: string) =>
    apiFetch<{ id: string; conversation_id: string; type: string; mime_type: string }>(
      `/video/assets/${encodeURIComponent(assetId)}/attach-to-conversation`,
      { method: 'POST', body: JSON.stringify({ conversation_id: conversationId }) },
    ),

  registerAssetInLibrary: (assetId: string) =>
    apiFetch<{ id: string; display_name: string; scope: string }>(
      `/video/assets/${encodeURIComponent(assetId)}/register-in-library`,
      { method: 'POST', body: JSON.stringify({}) },
    ),

  downloadUrl: (assetId: string) => videoAssetUrl(assetId),

  enhancePrompt: (data: { prompt: string; aspect_ratio?: string; duration_seconds?: number; negative_prompt?: string }) =>
    apiFetch<{ prompt: string }>('/video/enhance-prompt', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  getTimeline: (projectId: string) =>
    apiFetch<VideoTimelineDetail>(`/video/projects/${encodeURIComponent(projectId)}/timeline`),

  saveTimeline: (projectId: string, document: VideoTimelineDocument) =>
    apiFetch<VideoTimelineDetail>(`/video/projects/${encodeURIComponent(projectId)}/timeline`, {
      method: 'PUT',
      body: JSON.stringify(document),
    }),

  importAssetToTimeline: (projectId: string, data: { asset_id: string; track_id?: string; track_type?: string; start_ms?: number; duration_ms?: number }) =>
    apiFetch<VideoTimelineDetail>(`/video/projects/${encodeURIComponent(projectId)}/timeline/import-asset`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  renderTimeline: (projectId: string, data: VideoExportSettings) =>
    apiFetch<VideoRenderJob>(`/video/projects/${encodeURIComponent(projectId)}/render`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  getRenderJob: (jobId: string) =>
    apiFetch<VideoRenderJob>(`/video/render-jobs/${encodeURIComponent(jobId)}`),

  cancelRenderJob: (jobId: string) =>
    apiFetch<VideoRenderJob>(`/video/render-jobs/${encodeURIComponent(jobId)}/cancel`, {
      method: 'POST',
    }),

  rendererCapabilities: () =>
    apiFetch<VideoRendererCapabilities>('/video/render/capabilities'),

  updateAsset: (assetId: string, data: { file_name: string }) =>
    apiFetch<VideoAsset>(`/video/assets/${encodeURIComponent(assetId)}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  assistant: {
    storyboard: (projectId: string, data: VideoAssistantRequest) =>
      apiFetch<VideoStoryboardResponse>(`/video/projects/${encodeURIComponent(projectId)}/assistant/storyboard`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),

    timelinePlan: (projectId: string, data: VideoAssistantRequest) =>
      apiFetch<VideoEditPlan>(`/video/projects/${encodeURIComponent(projectId)}/assistant/timeline-plan`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),

    editPlan: (projectId: string, data: VideoAssistantRequest) =>
      apiFetch<VideoEditPlan>(`/video/projects/${encodeURIComponent(projectId)}/assistant/edit-plan`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),

    applyEditPlan: (projectId: string, data: VideoEditPlan) =>
      apiFetch<VideoTimelineDetail>(`/video/projects/${encodeURIComponent(projectId)}/assistant/apply-edit-plan`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),

    socialVariants: (projectId: string, data: VideoAssistantRequest) =>
      apiFetch<VideoSocialVariant[]>(`/video/projects/${encodeURIComponent(projectId)}/assistant/social-variants`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
  },

  /** Start async generation. Returns immediately with 202. Poll via getGeneration(). */
  generate: (req: GenerateVideoRequest) =>
    apiFetch<{ generation_id: string; project_id: string; status: string; generation: VideoGenerationDetail }>(
      '/video/generations',
      { method: 'POST', body: JSON.stringify(req) },
    ),

  cancelGeneration: (generationId: string) =>
    apiFetch<{ generation_id: string; status: string }>(
      `/video/generations/${encodeURIComponent(generationId)}/cancel`,
      { method: 'POST' },
    ),

  validateGeneration: (req: GenerateVideoRequest) =>
    apiFetch<VideoGenerationValidationResult>(
      '/video/generations/validate',
      { method: 'POST', body: JSON.stringify(req) },
    ),
};

// ── Crossover Translation ──────────────────────────────────────────────────

export const crossoverApi = {
  /** Translate a domain prompt between studios via LLM. */
  translate: {
    musicToImage: (content: CrossoverTranslateRequest['content']) =>
      apiFetch<CrossoverMusicToImageResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'music', target: 'image', content } satisfies CrossoverTranslateRequest),
      }),

    imageToMusic: (content: CrossoverTranslateRequest['content']) =>
      apiFetch<CrossoverImageToMusicResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'image', target: 'music', content } satisfies CrossoverTranslateRequest),
      }),

    /** Distill a raw LLM chat response into a clean Lyria music prompt with genre/mood/instruments. */
    chatToMusic: (content: { prompt: string }) =>
      apiFetch<CrossoverImageToMusicResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'chat', target: 'music', content }),
      }),

    imageToVideo: (content: CrossoverTranslateRequest['content']) =>
      apiFetch<CrossoverToVideoResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'image', target: 'video', content } satisfies CrossoverTranslateRequest),
      }),

    musicToVideo: (content: CrossoverTranslateRequest['content']) =>
      apiFetch<CrossoverToVideoResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'music', target: 'video', content } satisfies CrossoverTranslateRequest),
      }),

    chatToVideo: (content: { prompt: string }) =>
      apiFetch<CrossoverToVideoResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'chat', target: 'video', content } satisfies CrossoverTranslateRequest),
      }),

    videoToImage: (content: CrossoverTranslateRequest['content']) =>
      apiFetch<CrossoverMusicToImageResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'video', target: 'image', content } satisfies CrossoverTranslateRequest),
      }),

    videoToMusic: (content: CrossoverTranslateRequest['content']) =>
      apiFetch<CrossoverImageToMusicResponse>('/crossover/translate', {
        method: 'POST',
        body: JSON.stringify({ source: 'video', target: 'music', content } satisfies CrossoverTranslateRequest),
      }),
  },
};
