import { create } from 'zustand';
import { toast } from 'sonner';
import { api } from '../api';
import type { Conversation, Message, ProviderProfile, WebSearchResult, FeatureFlag } from '../types';

// ---- Conversation Store ----

interface ConversationState {
  conversations: Conversation[];
  activeId: string | null;
  loading: boolean;
  error: string | null;
  searchQuery: string;
  showArchived: boolean;

  fetchConversations: (includeArchived?: boolean, workspaceId?: string | null) => Promise<void>;
  setShowArchived: (show: boolean) => void;
  createConversation: (title?: string) => Promise<Conversation>;
  selectConversation: (id: string) => void;
  updateConversation: (id: string, data: Partial<Conversation>) => Promise<void>;
  deleteConversation: (id: string) => Promise<void>;
  setSearchQuery: (query: string) => void;
  searchConversations: (query: string) => Promise<void>;
}

export const useConversationStore = create<ConversationState>((set, get) => ({
  conversations: [],
  activeId: null,
  loading: false,
  error: null,
  searchQuery: '',

  showArchived: false,

  fetchConversations: async (includeArchived?: boolean, workspaceId?: string | null) => {
    const include = includeArchived ?? get().showArchived;
    set({ loading: true, error: null });
    try {
      const conversations = await api.listConversations(include, workspaceId);
      set({ conversations, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },

  setShowArchived: (show: boolean) => {
    set({ showArchived: show });
    get().fetchConversations(show);
  },

  createConversation: async (title?: string) => {
    const convo = await api.createConversation({ title: title || 'New Conversation' });
    set((s) => ({
      conversations: [convo, ...s.conversations],
      activeId: convo.id,
    }));
    return convo;
  },

  selectConversation: (id: string) => {
    set({ activeId: id });
  },

  updateConversation: async (id: string, data: Partial<Conversation>) => {
    const updated = await api.updateConversation(id, data);
    set((s) => ({
      conversations: s.conversations.map((c) => (c.id === id ? updated : c)),
    }));
  },

  deleteConversation: async (id: string) => {
    await api.deleteConversation(id);
    set((s) => ({
      conversations: s.conversations.filter((c) => c.id !== id),
      activeId: s.activeId === id ? null : s.activeId,
    }));
  },

  setSearchQuery: (query: string) => set({ searchQuery: query }),

  searchConversations: async (query: string) => {
    if (!query.trim()) {
      return get().fetchConversations();
    }
    set({ loading: true });
    try {
      const conversations = await api.searchConversations(query);
      set({ conversations, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },
}));

// ---- Message Store ----

// Counter to prevent stale fetchMessages responses from overwriting state on rapid conversation switching
let fetchMessageCounter = 0;

interface MessageState {
  messages: Message[];
  streaming: boolean;
  streamingContent: string;
  streamingThinking: string;
  streamingConversationId: string | null;
  loading: boolean;
  error: string | null;
  abortStream: (() => void) | null;
  webSearching: boolean;
  webSearchResults: WebSearchResult[] | null;
  webSearchQuery: string | null;

  fetchMessages: (conversationId: string) => Promise<void>;
  sendMessage: (conversationId: string, content: string, override?: { provider?: string; model?: string }, attachmentIds?: string[], webSearch?: boolean, think?: boolean) => void;
  generateImage: (conversationId: string, prompt: string, override?: { provider?: string; model?: string }, options?: { size?: string; quality?: string; referenceImageId?: string }) => Promise<void>;
  regenerateLastMessage: (conversationId: string) => Promise<void>;
  editAndResend: (conversationId: string, messageId: string, newContent: string) => Promise<void>;
  deleteMessage: (conversationId: string, messageId: string) => Promise<void>;
  clearMessages: () => void;
  stopStreaming: () => void;
}

export const useMessageStore = create<MessageState>((set, get) => ({
  messages: [],
  streaming: false,
  streamingContent: '',
  streamingThinking: '',
  streamingConversationId: null,
  loading: false,
  error: null,
  abortStream: null,
  webSearching: false,
  webSearchResults: null,
  webSearchQuery: null,

  fetchMessages: async (conversationId: string) => {
    // Increment counter to detect stale responses from rapid switching
    const fetchToken = ++fetchMessageCounter;
    set({ loading: true, error: null });
    try {
      const messages = await api.listMessages(conversationId);
      // Only commit if this is still the latest fetch request
      if (fetchToken === fetchMessageCounter) {
        set({ messages, loading: false });
      }
    } catch (err) {
      if (fetchToken === fetchMessageCounter) {
        set({ error: (err as Error).message, loading: false });
      }
    }
  },

  sendMessage: (conversationId: string, content: string, override?: { provider?: string; model?: string }, attachmentIds?: string[], webSearch?: boolean, think?: boolean) => {
    set({ streaming: true, streamingContent: '', streamingThinking: '', streamingConversationId: conversationId, error: null, webSearching: false, webSearchResults: null, webSearchQuery: null });

    const reqBody: { content: string; override?: { provider?: string; model?: string }; attachment_ids?: string[]; web_search?: boolean; think?: boolean } = { content };
    if (override) reqBody.override = override;
    if (attachmentIds && attachmentIds.length > 0) reqBody.attachment_ids = attachmentIds;
    if (webSearch !== undefined) reqBody.web_search = webSearch;
    if (think !== undefined) reqBody.think = think;

    const { abort } = api.streamMessage(
      conversationId,
      reqBody,
      {
        onStart: (data) => {
          // Add user message optimistically
          const userMsg: Message = {
            id: data.user_message_id,
            conversation_id: conversationId,
            role: 'user',
            content,
            created_at: new Date().toISOString(),
          };
          set((s) => ({ messages: [...s.messages, userMsg] }));
        },
        onToken: (tokenContent) => {
          set((s) => ({ streamingContent: s.streamingContent + tokenContent, webSearching: false }));
        },
        onThinking: (thinkingContent) => {
          set((s) => ({ streamingThinking: s.streamingThinking + thinkingContent }));
        },
        onWebSearch: (data) => {
          set({ webSearching: true, webSearchQuery: data?.tool_call?.arguments?.query || null });
        },
        onWebSearchResults: (data) => {
          set({ webSearchResults: data.results, webSearching: false, webSearchQuery: data.query || null });
        },
        onDone: (data) => {
          // Guard: skip if no valid message_id (phantom prevention)
          if (!data.message_id) {
            set({ streaming: false, streamingContent: '', streamingThinking: '', streamingConversationId: null, abortStream: null, webSearching: false });
            return;
          }
          const metadata: Record<string, unknown> = {};
          if (data.web_search) {
            metadata.web_search = true;
            metadata.tool = 'web_search';
            metadata.sources = data.sources || get().webSearchResults;
          }
          if (data.thinking) {
            metadata.thinking = data.thinking;
          }
          const assistantMsg: Message = {
            id: data.message_id,
            conversation_id: conversationId,
            role: 'assistant',
            content: get().streamingContent,
            created_at: new Date().toISOString(),
            provider: data.provider,
            model: data.model,
            latency_ms: data.latency_ms,
            metadata_json: Object.keys(metadata).length > 0 ? JSON.stringify(metadata) : undefined,
          };
          set((s) => ({
            messages: [...s.messages, assistantMsg],
            streaming: false,
            streamingContent: '',
            streamingThinking: '',
            streamingConversationId: null,
            abortStream: null,
            webSearching: false,
          }));
        },
        onError: (error) => {
          set({ error, streaming: false, streamingContent: '', streamingThinking: '', streamingConversationId: null, abortStream: null, webSearching: false, webSearchResults: null, webSearchQuery: null });
        },
      }
    );

    set({ abortStream: abort });
  },

  clearMessages: () => set({ messages: [], streamingContent: '', streamingThinking: '', streaming: false, streamingConversationId: null, webSearching: false, webSearchResults: null, webSearchQuery: null }),

  stopStreaming: () => {
    const { abortStream, streamingContent, streamingConversationId } = get();
    abortStream?.();
    if (streamingContent) {
      // Keep whatever was streamed so far as a partial message
      const partialMsg: Message = {
        id: `partial-${Date.now()}`,
        conversation_id: streamingConversationId || '',
        role: 'assistant',
        content: streamingContent + '\n\n*[Generation stopped]*',
        created_at: new Date().toISOString(),
      };
      set((s) => ({
        messages: [...s.messages, partialMsg],
        streaming: false,
        streamingContent: '',
        streamingConversationId: null,
        abortStream: null,
        webSearching: false,
        webSearchResults: null,
        webSearchQuery: null,
      }));
    } else {
      set({ streaming: false, streamingContent: '', streamingConversationId: null, abortStream: null, webSearching: false, webSearchResults: null, webSearchQuery: null });
    }
  },

  deleteMessage: async (conversationId: string, messageId: string) => {
    await api.deleteMessage(conversationId, messageId);
    set((s) => ({
      messages: s.messages.filter((m) => m.id !== messageId),
    }));
  },

  generateImage: async (conversationId: string, prompt: string, override?: { provider?: string; model?: string }, options?: { size?: string; quality?: string; referenceImageId?: string }) => {
    set({ streaming: true, streamingContent: '', error: null });
    try {
      const result = await api.generateImage(conversationId, {
        prompt,
        size: options?.size,
        quality: options?.quality,
        reference_image_id: options?.referenceImageId,
        override,
      });
      // Add both messages to state
      set((s) => ({
        messages: [...s.messages, result.user_message, result.assistant_message],
        streaming: false,
        streamingContent: '',
      }));
    } catch (err) {
      set({ streaming: false, error: (err as Error).message });
      toast.error(`Image generation failed: ${(err as Error).message}`);
      // Resync messages — backend may have stored then cleaned up the user prompt
      if (conversationId) get().fetchMessages(conversationId);
    }
  },

  regenerateLastMessage: async (conversationId: string) => {
    const { messages } = get();
    // Find the last user message
    const lastUserMsg = [...messages].reverse().find((m) => m.role === 'user');
    if (!lastUserMsg) return;

    // Remove the last assistant message from backend first
    const lastAssistant = [...messages].reverse().find((m) => m.role === 'assistant');
    if (lastAssistant) {
      try {
        await api.deleteMessage(conversationId, lastAssistant.id);
      } catch (err) {
        toast.error(`Failed to delete message: ${(err as Error).message}`);
        get().fetchMessages(conversationId);
        return;
      }
    }

    // Remove last assistant message from local state, then re-send
    const filtered = lastAssistant
      ? messages.filter((m) => m.id !== lastAssistant.id)
      : messages;
    set({ messages: filtered });

    // Re-send the last user message (remove it from state too since sendMessage will re-add it)
    const withoutLastUser = filtered.filter((m) => m.id !== lastUserMsg.id);
    set({ messages: withoutLastUser });

    get().sendMessage(conversationId, lastUserMsg.content);
  },

  editAndResend: async (conversationId: string, messageId: string, newContent: string) => {
    // Delete from messageId onward on backend — await confirmation
    try {
      await api.editMessage(conversationId, messageId, newContent);
    } catch (err) {
      toast.error(`Failed to edit message: ${(err as Error).message}`);
      get().fetchMessages(conversationId);
      return;
    }

    // Remove that message and everything after it from local state
    const { messages } = get();
    const idx = messages.findIndex((m) => m.id === messageId);
    if (idx === -1) return;
    const trimmed = messages.slice(0, idx);
    set({ messages: trimmed });

    // Re-send with the new content
    get().sendMessage(conversationId, newContent);
  },
}));

// ---- Provider Store ----

interface ProviderState {
  providers: ProviderProfile[];
  loading: boolean;
  error: string | null;

  fetchProviders: () => Promise<void>;
  createProvider: (data: { name: string; type: string; base_url?: string; default_model?: string; default_image_model?: string; api_key?: string }) => Promise<void>;
  updateProvider: (id: string, data: Partial<ProviderProfile> & { api_key?: string }) => Promise<void>;
  deleteProvider: (id: string) => Promise<void>;
}

export const useProviderStore = create<ProviderState>((set) => ({
  providers: [],
  loading: false,
  error: null,

  fetchProviders: async () => {
    set({ loading: true, error: null });
    try {
      const providers = await api.listProviders();
      set({ providers, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },

  createProvider: async (data) => {
    const provider = await api.createProvider(data);
    set((s) => ({ providers: [...s.providers, provider] }));
  },

  updateProvider: async (id, data) => {
    const updated = await api.updateProvider(id, data);
    set((s) => ({
      providers: s.providers.map((p) => (p.id === id ? updated : p)),
    }));
  },

  deleteProvider: async (id) => {
    await api.deleteProvider(id);
    set((s) => ({
      providers: s.providers.filter((p) => p.id !== id),
    }));
  },
}));

// ---- Settings Store ----

interface SettingsState {
  settings: import('../types').AppSettings;
  loading: boolean;
  sidebarOpen: boolean;
  settingsOpen: boolean;
  appMode: 'chat' | 'image';

  fetchSettings: () => Promise<void>;
  updateSettings: (data: Partial<import('../types').AppSettings>) => Promise<void>;
  toggleSidebar: () => void;
  toggleSettings: () => void;
  setAppMode: (mode: 'chat' | 'image') => void;
}

const defaultSettings: import('../types').AppSettings = {
  web_search_provider: 'auto',
  brave_api_key: '',
  jina_reader_enabled: true,
  jina_reader_max_len: 3000,
  rag_enabled: false,
  rag_embedding_model: 'text-embedding-3-small',
  rag_chunk_size: 512,
  rag_chunk_overlap: 64,
  rag_top_k: 5,
};

function getInitialSidebarOpen(): boolean {
  if (typeof window === 'undefined') return true;
  const saved = window.localStorage.getItem('omnillm_sidebar_open');
  if (saved == null) return true;
  return saved === 'true';
}

function getInitialAppMode(): 'chat' | 'image' {
  if (typeof window === 'undefined') return 'chat';
  const saved = window.localStorage.getItem('omnillm_app_mode');
  if (saved === 'image') return 'image';
  return 'chat';
}

export const useSettingsStore = create<SettingsState>((set) => ({
  settings: defaultSettings,
  loading: false,
  sidebarOpen: getInitialSidebarOpen(),
  settingsOpen: false,
  appMode: getInitialAppMode(),

  fetchSettings: async () => {
    set({ loading: true });
    try {
      const settings = await api.getSettings();
      set({ settings, loading: false });
    } catch {
      set({ loading: false });
    }
  },

  updateSettings: async (data) => {
    const settings = await api.updateSettings(data);
    set({ settings });
  },

  toggleSidebar: () => set((s) => {
    const next = !s.sidebarOpen;
    if (typeof window !== 'undefined') {
      window.localStorage.setItem('omnillm_sidebar_open', String(next));
    }
    return { sidebarOpen: next };
  }),
  toggleSettings: () => set((s) => ({ settingsOpen: !s.settingsOpen })),

  setAppMode: (mode) => set(() => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem('omnillm_app_mode', mode);
    }
    return { appMode: mode };
  }),
}));

// ---- Feature Flag Store ----

interface FeatureFlagState {
  features: FeatureFlag[];
  loading: boolean;

  fetchFeatures: () => Promise<void>;
  isEnabled: (key: string) => boolean;
  updateFeature: (key: string, enabled: boolean) => Promise<void>;
}

export const useFeatureFlagStore = create<FeatureFlagState>((set, get) => ({
  features: [],
  loading: false,

  fetchFeatures: async () => {
    set({ loading: true });
    try {
      const features = await api.listFeatures();
      set({ features: features || [], loading: false });
    } catch {
      set({ loading: false });
    }
  },

  isEnabled: (key: string) => {
    const flag = get().features.find((f) => f.key === key);
    return flag?.enabled ?? false;
  },

  updateFeature: async (key: string, enabled: boolean) => {
    try {
      const features = await api.updateFeature(key, enabled);
      set({ features: features || [] });
    } catch (err) {
      toast.error(`Failed to update feature: ${(err as Error).message}`);
    }
  },
}));
