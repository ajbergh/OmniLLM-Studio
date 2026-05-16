import { useRef, useEffect, useState, useCallback } from 'react';
import { useConversationStore, useMessageStore, useSettingsStore, useProviderStore } from '../stores';
import {
  Send, Square, Bot, User, Copy, Check, Clock, Cpu, Globe,
  ChevronDown, ChevronUp, ExternalLink, RefreshCw, Pencil,
  Download, FileText, ArrowDown, Sparkles, Paperclip, X, Image, ImagePlus,
  GitBranch, Layout, Zap, Brain, Link as LinkIcon, Files, DollarSign,
} from 'lucide-react';
import { clsx } from 'clsx';
import { motion, AnimatePresence } from 'framer-motion';
import { ModelSelector } from './ModelSelector';
import { WelcomeScreen } from './WelcomeScreen';
import { MarkdownContent } from './MarkdownContent';
import { BranchSwitcher } from './BranchSwitcher';
import { RAGSourcePanel } from './RAGSourcePanel';
import { URLContextSourcePanel } from './URLContextSourcePanel';
import { ToolCallCard } from './ToolCallCard';
import { AgentRunView } from './AgentRunView';
import { AttachmentPanel } from './AttachmentPanel';
import { toast } from 'sonner';
import { api, imageSessionApi, templateApi, branchApi, agentApi, workspaceApi } from '../api';
import { useImageEditorStore } from '../stores/imageEditor';
import { matchesShortcut } from '../shortcuts';
import type { Message, WebSearchResult, FileSearchResult, MessageMetadata, OpenRouterMetadata, URLContextSourceRef, PromptTemplate, UsageSummary, ToolCall, ToolResult } from '../types';
import { AgentEventType } from '../types';
import { getModelReasoningLevels, getModelToolCallingSupport, isFreeModel, type ReasoningEffortLevel } from '../models';

type PendingUploadStatus = 'pending' | 'uploading' | 'failed';

interface PendingUploadFile {
  id: string;
  file: File;
  status: PendingUploadStatus;
  error?: string;
}

const MAX_ATTACHMENT_BYTES = 50 * 1024 * 1024;

// MIME types whose text content the backend can extract for LLM context / RAG.
// Kept in sync with isTextMIME + canExtractAttachmentText in attachment_text.go.
const EXTRACTABLE_MIME_PREFIXES = ['text/'];
const EXTRACTABLE_MIME_EXACT = new Set([
  'application/pdf',
  'application/json',
  'application/xml',
  'application/javascript',
  'application/typescript',
  'application/x-yaml',
  'application/yaml',
  'application/toml',
  'application/x-sh',
  'application/sql',
  'application/csv',
  // Office Open XML
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document', // .docx
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',        // .xlsx
  'application/vnd.openxmlformats-officedocument.presentationml.presentation', // .pptx
]);

function isExtractableMIME(mime: string): boolean {
  const base = mime.split(';')[0].trim().toLowerCase();
  return EXTRACTABLE_MIME_PREFIXES.some((p) => base.startsWith(p)) || EXTRACTABLE_MIME_EXACT.has(base);
}

// File-picker accept string covering images + all extractable types.
const FILE_ACCEPT = [
  'image/*',
  'application/pdf',
  'text/*',
  'application/json',
  'application/xml',
  'application/javascript',
  'application/typescript',
  'application/x-yaml',
  'application/yaml',
  'application/toml',
  'application/x-sh',
  'application/sql',
  'application/csv',
  '.docx',
  '.xlsx',
  '.pptx',
].join(',');

function toPendingUpload(file: File): PendingUploadFile {
  return {
    id: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2, 8)}`,
    file,
    status: 'pending',
  };
}

function formatBrowserDetail(value: string): string {
  try {
    return new URL(value).hostname;
  } catch {
    return value;
  }
}

function getToolCallName(call: ToolCall): string {
  return call.name || call.function?.name || 'tool';
}

function getToolCallArgs(call: ToolCall): Record<string, unknown> | undefined {
  if (call.arguments) return call.arguments;
  const rawArgs = call.function?.arguments;
  if (!rawArgs) return undefined;
  if (typeof rawArgs === 'object') return rawArgs;
  try {
    const parsed = JSON.parse(rawArgs);
    return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : undefined;
  } catch {
    return { arguments: rawArgs };
  }
}

export function ChatView() {
  const activeId = useConversationStore((s) => s.activeId);
  const conversations = useConversationStore((s) => s.conversations);
  const { createConversation, selectConversation, updateConversation, fetchConversations } = useConversationStore();
  const { toggleSettings, setAppMode } = useSettingsStore();
  const providers = useProviderStore((s) => s.providers);
  const {
    messages, streaming, streamingContent, streamingThinking, error,
    sendMessage, clearMessages, stopStreaming,
    webSearching, webSearchQuery, urlContextStatus, urlContextKind,
    browserStatus, browserStatusDetail, browserProgress,
    ragIndexingStatus, ragIndexingDetail, imageGenerating,
    regenerateLastMessage, editAndResend, generateImage,
  } = useMessageStore();
  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [inputFocused, setInputFocused] = useState(false);
  const [showScrollDown, setShowScrollDown] = useState(false);
  const [isNearBottom, setIsNearBottom] = useState(true);
  const [titleGenerated, setTitleGenerated] = useState<Set<string>>(new Set());
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
  const [pendingFiles, setPendingFiles] = useState<PendingUploadFile[]>([]);
  const [uploading, setUploading] = useState(false);
  const [imageMode, setImageMode] = useState(false);
  const [editPreviousImage, setEditPreviousImage] = useState(false);
  const [activeBranchId, setActiveBranchId] = useState<string | undefined>(undefined);
  const [agentMode, setAgentMode] = useState(false);
  const [templatePickerOpen, setTemplatePickerOpen] = useState(false);
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [conversationUsage, setConversationUsage] = useState<UsageSummary | null>(null);
  const [attachmentPanelOpen, setAttachmentPanelOpen] = useState(false);
  const [webSearchEnabled, setWebSearchEnabled] = useState(true);
  const [thinkEnabled, setThinkEnabled] = useState(false);
  const [reasoningEffort, setReasoningEffort] = useState<ReasoningEffortLevel | undefined>(undefined);
  const [activeWorkspaceName, setActiveWorkspaceName] = useState<string | null>(null);
  const browserStatusDetailLabel = browserStatusDetail ? formatBrowserDetail(browserStatusDetail) : null;

  // Detect if the active conversation is using an Ollama provider
  const activeConvo = conversations.find((c) => c.id === activeId);
  const activeProvider = (() => {
    const provId = activeConvo?.default_provider;
    if (provId) {
      return providers.find((p) => p.id === provId || p.name === provId);
    }
    // Fallback: first enabled provider
    const enabled = providers.filter((p) => p.enabled);
    return enabled.length === 1 ? enabled[0] : undefined;
  })();
  const isOllamaProvider = (() => {
    const isOllamaType = (p: { type?: string; base_url?: string }) =>
      p.type?.toLowerCase() === 'ollama' ||
      (p.base_url?.includes('11434') ?? false);

    if (activeProvider) return isOllamaType(activeProvider);
    return false;
  })();
  const isOpenRouterProvider = activeProvider?.type?.toLowerCase() === 'openrouter';

  const openRouterOptions = (() => {
    if (!isOpenRouterProvider) return undefined;
    try {
      const meta = JSON.parse(activeProvider?.metadata_json || '{}') as OpenRouterMetadata;
      return {
        provider_prefs: meta.provider_prefs,
        model_fallbacks: meta.model_fallbacks,
        route: meta.route,
        plugins: meta.plugins,
      };
    } catch {
      return undefined;
    }
  })();

  // Check if the active provider supports image generation (from backend capability field)
  const isImageCapable = activeProvider?.image_capable === true;

  // Reasoning effort levels supported by the current model (null = not supported)
  const reasoningLevels = getModelReasoningLevels(
    activeProvider?.type || '',
    activeConvo?.default_model || ''
  );
  // Whether the current provider+model supports structured function calling.
  // false for Gemini (excluded in backend) and non-capable Ollama models.
  const toolsSupported = getModelToolCallingSupport(
    activeProvider?.type || '',
    activeConvo?.default_model || ''
  );
  const activeModelIsFree = isFreeModel(activeProvider?.type || '', activeConvo?.default_model);

  // Find the most recent generated-image attachment ID in the conversation.
  // Image generation messages have metadata_json containing "image_generation".
  // The content contains markdown like ![...](/v1/attachments/{id}/download).
  const lastImageAttachmentId = (() => {
    const imgMsgs = messages.filter(
      (m) => m.role === 'assistant' && m.metadata_json?.includes('image_generation')
    );
    if (imgMsgs.length === 0) return undefined;
    const lastMsg = imgMsgs[imgMsgs.length - 1];
    const match = lastMsg.content.match(/\/v1\/attachments\/([a-f0-9-]+)\/download/);
    return match?.[1];
  })();

  const setComposerMode = (mode: 'chat' | 'image' | 'agent') => {
    setImageMode(mode === 'image');
    setAgentMode(mode === 'agent');
    if (mode !== 'image') setEditPreviousImage(false);
  };

  // Fetch workspace name for the active conversation's project badge
  useEffect(() => {
    const wsId = activeConvo?.workspace_id;
    if (!wsId) { setActiveWorkspaceName(null); return; }
    workspaceApi.list().then((list) => {
      const ws = list?.find((w) => w.id === wsId);
      setActiveWorkspaceName(ws?.name || null);
    }).catch(() => setActiveWorkspaceName(null));
  }, [activeConvo?.workspace_id]);

  // Ctrl+E export shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (matchesShortcut(e, 'exportConversation')) {
        e.preventDefault();
        if (messages.length > 0) {
          setExportMenuOpen((prev) => !prev);
        }
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [messages.length]);

  // Auto-scroll while the user is already near bottom.
  // If they scroll up to read earlier content, avoid forcing them back down.
  useEffect(() => {
    if (!isNearBottom) return;
    messagesEndRef.current?.scrollIntoView({ behavior: streaming ? 'auto' : 'smooth' });
  }, [messages, streamingContent, isNearBottom, streaming]);

  // Focus input when conversation changes
  useEffect(() => {
    inputRef.current?.focus();
    // Reset branch and modes when switching conversations
    setActiveBranchId(undefined);
    setConversationUsage(null);
    setImageMode(false);
    setEditPreviousImage(false);
    setAgentMode(false);
  }, [activeId]);

  // Fetch per-conversation usage
  useEffect(() => {
    if (!activeId || messages.length === 0) return;
    api.getConversationUsage(activeId).then(setConversationUsage).catch(() => {});
  }, [activeId, messages.length]);

  // Auto-exit image mode if provider becomes non-capable
  useEffect(() => {
    if (imageMode && !isImageCapable) {
      setImageMode(false);
    }
  }, [imageMode, isImageCapable]);

  // Track scroll position for "scroll to bottom" button
  useEffect(() => {
    const container = messagesContainerRef.current;
    if (!container) return;
    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = container;
      const distanceFromBottom = scrollHeight - scrollTop - clientHeight;
      const nearBottom = distanceFromBottom <= 100;
      setIsNearBottom(nearBottom);
      setShowScrollDown(!nearBottom);
    };
    container.addEventListener('scroll', handleScroll);
    handleScroll();
    return () => container.removeEventListener('scroll', handleScroll);
  }, []);

  // Auto-generate title after first assistant response
  useEffect(() => {
    if (!activeId || streaming) return;
    if (titleGenerated.has(activeId)) return;

    const userMsgCount = messages.filter((m) => m.role === 'user').length;
    const assistantMsgCount = messages.filter((m) => m.role === 'assistant').length;
    const activeConvo = conversations.find((c) => c.id === activeId);

    // Generate title when we have exactly 1 exchange and title is still default
    if (
      userMsgCount >= 1 &&
      assistantMsgCount >= 1 &&
      activeConvo &&
      (activeConvo.title === 'New Conversation' || activeConvo.title === 'New Chat')
    ) {
      setTitleGenerated((prev) => new Set(prev).add(activeId));
      api.generateTitle(activeId).then(({ title }) => {
        updateConversation(activeId, { title });
        fetchConversations();
      }).catch(() => {
        // Silent failure - title gen is best-effort
      });
    }
  }, [activeId, messages, streaming, conversations, titleGenerated, updateConversation, fetchConversations]);

  const queueFiles = useCallback((files: File[]) => {
    const withinLimit = files.filter((f) => f.size <= MAX_ATTACHMENT_BYTES);
    if (withinLimit.length < files.length) {
      toast.error('Some files exceed the 10 MB limit and were skipped');
    }

    if (withinLimit.length === 0) return;

    // Warn about files that will be uploaded but whose content can't be extracted
    // for LLM context or RAG (not text, not PDF/docx/xlsx/pptx, not images).
    const notExtractable = withinLimit.filter(
      (f) => !isExtractableMIME(f.type) && !f.type.startsWith('image/')
    );
    if (notExtractable.length > 0) {
      const names = notExtractable.map((f) => f.name).join(', ');
      toast.warning(`File content will not be readable by the AI (unsupported type): ${names}`);
    }

    setPendingFiles((prev) => {
      const existing = new Set(
        prev.map((p) => `${p.file.name}-${p.file.size}-${p.file.lastModified}`)
      );
      const next = [...prev];

      for (const file of withinLimit) {
        const signature = `${file.name}-${file.size}-${file.lastModified}`;
        if (existing.has(signature)) continue;
        existing.add(signature);
        next.push(toPendingUpload(file));
      }
      return next;
    });
  }, []);

  const handleSend = useCallback(async () => {
    if ((!input.trim() && pendingFiles.length === 0) || streaming) return;

    let currentId = activeId;

    // Auto-create conversation if none selected
    if (!currentId) {
      try {
        const enabledProviders = providers.filter((p) => p.enabled);
        const defaultProvider = enabledProviders[0];
        const convo = await createConversation(undefined, {
          provider: defaultProvider?.id,
          model: defaultProvider?.default_model || undefined,
        });
        clearMessages();
        selectConversation(convo.id);
        currentId = convo.id;
      } catch {
        toast.error('Failed to create conversation');
        return;
      }
    }

    // Image generation mode
    if (imageMode && input.trim()) {
      const editId = editPreviousImage && lastImageAttachmentId ? lastImageAttachmentId : undefined;
      generateImage(currentId!, input.trim(), undefined, { referenceImageId: editId });
      setInput('');
      if (inputRef.current) {
        inputRef.current.style.height = 'auto';
      }
      return;
    }

    // Agent mode — start an agent run via the composer
    if (agentMode && input.trim()) {
      const goalText = input.trim();
      setInput('');
      if (inputRef.current) inputRef.current.style.height = 'auto';

      try {
        const { promise } = agentApi.startRun(
          currentId!,
          {
            goal: goalText,
            provider: activeConvo?.default_provider || '',
            model: activeConvo?.default_model || '',
          },
          (event) => {
            try {
              const payload = event.data ? JSON.parse(event.data) : {};
              if (event.type === AgentEventType.Complete) {
                toast.success('Agent run completed');
              } else if (event.type === AgentEventType.Error) {
                toast.error(`Agent error: ${payload?.data?.error || payload?.error || 'Unknown error'}`);
              } else if (event.type === AgentEventType.ApprovalRequired) {
                toast.info('Agent is waiting for your approval');
              }
            } catch { /* ignore parse errors */ }
          },
        );
        await promise;
      } catch (err) {
        toast.error(`Agent run failed: ${(err as Error).message}`);
      }
      return;
    }

    // Upload pending files and collect IDs
    const attachmentIds: string[] = [];
    if (pendingFiles.length > 0) {
      setUploading(true);
      let failedCount = 0;
      const filesToUpload = pendingFiles.filter((p) => p.status === 'pending' || p.status === 'failed');

      for (const pending of filesToUpload) {
        setPendingFiles((prev) => prev.map((p) => (
          p.id === pending.id
            ? { ...p, status: 'uploading', error: undefined }
            : p
        )));

        try {
          const uploaded = await api.uploadAttachment(currentId!, pending.file);
          attachmentIds.push(uploaded.id);
          setPendingFiles((prev) => prev.filter((p) => p.id !== pending.id));
        } catch {
          failedCount++;
          setPendingFiles((prev) => prev.map((p) => (
            p.id === pending.id
              ? { ...p, status: 'failed', error: 'Upload failed' }
              : p
          )));
        }
      }

      setUploading(false);
      if (failedCount > 0) {
        toast.error(`${failedCount} file(s) failed to upload. Fix or retry, then send again.`);
      }
    }

    if (input.trim() || attachmentIds.length > 0) {
      const content = input.trim() || 'Please analyze the attached files.';
      sendMessage(
        currentId!,
        content,
        undefined,
        attachmentIds.length > 0 ? attachmentIds : undefined,
        webSearchEnabled,
        isOllamaProvider && thinkEnabled ? true : undefined,
        reasoningLevels ? reasoningEffort : undefined,
        openRouterOptions,
      );
      setInput('');
      if (inputRef.current) {
        inputRef.current.style.height = 'auto';
      }
    }
  }, [input, activeId, streaming, pendingFiles, imageMode, editPreviousImage, lastImageAttachmentId, agentMode, activeConvo, webSearchEnabled, thinkEnabled, isOllamaProvider, openRouterOptions, reasoningEffort, reasoningLevels, sendMessage, generateImage, createConversation, clearMessages, selectConversation]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (matchesShortcut(e as unknown as KeyboardEvent, 'sendMessage')) {
      e.preventDefault();
      handleSend();
      return;
    }
    if (matchesShortcut(e as unknown as KeyboardEvent, 'stopGenerating') && streaming) {
      stopStreaming();
    }
  };

  const handleNewChat = async () => {
    const enabledProviders = providers.filter((p) => p.enabled);
    const defaultProvider = enabledProviders[0];
    const convo = await createConversation(undefined, {
      provider: defaultProvider?.id,
      model: defaultProvider?.default_model || undefined,
    });
    clearMessages();
    selectConversation(convo.id);
    toast.success('New conversation created');
  };

  const handleExport = (format: 'markdown' | 'json') => {
    setExportMenuOpen(false);
    const activeConvo = conversations.find((c) => c.id === activeId);
    const title = activeConvo?.title || 'conversation';
    const safeName = title.replace(/[^a-zA-Z0-9]/g, '_').substring(0, 50);

    if (format === 'markdown') {
      let md = `# ${title}\n\n`;
      md += `*Exported on ${new Date().toLocaleDateString()} at ${new Date().toLocaleTimeString()}*\n\n---\n\n`;
      for (const msg of messages) {
        const icon = msg.role === 'user' ? '**You**' : '**Assistant**';
        const meta = msg.model ? ` *(${msg.model})*` : '';
        md += `### ${icon}${meta}\n\n${msg.content}\n\n---\n\n`;
      }
      downloadFile(`${safeName}.md`, md, 'text/markdown');
    } else {
      const data = {
        title,
        exported_at: new Date().toISOString(),
        messages: messages.map((m) => ({
          role: m.role,
          content: m.content,
          model: m.model,
          provider: m.provider,
          timestamp: m.created_at,
          ...(m.metadata_json ? { metadata: JSON.parse(m.metadata_json) } : {}),
        })),
      };
      downloadFile(`${safeName}.json`, JSON.stringify(data, null, 2), 'application/json');
    }
    toast.success(`Exported as ${format.toUpperCase()}`);
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const openProjectFiles = () => {
    const preferredScope = activeConvo?.workspace_id ? 'workspace' : 'conversation';
    window.dispatchEvent(new CustomEvent('omnillm:open-file-library', {
      detail: {
        preferredScope,
        preferredWorkspaceId: activeConvo?.workspace_id || null,
      },
    }));
  };

  if (!activeId) {
    return <WelcomeScreen onNewChat={handleNewChat} onOpenSettings={toggleSettings} />;
  }

  const messageCount = messages.length;
  const wordCount = messages.reduce((acc, m) => acc + m.content.split(/\s+/).length, 0);

  return (
    <>
    <div className="flex-1 flex flex-col min-h-0">
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -10 }}
        animate={{ opacity: 1, y: 0 }}
        className="px-3 py-3 sm:px-5 border-b border-border flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between glass-strong relative z-20"
      >
        <div className="flex min-w-0 items-center gap-3">
          <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center">
            <Bot size={14} className="text-primary" />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-medium text-text leading-tight truncate">
              {activeConvo?.title || 'Chat'}
            </h2>
            {activeWorkspaceName && (
              <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full bg-primary/15 text-primary font-medium mr-1">
                <Files size={9} /> {activeWorkspaceName}
              </span>
            )}
            <p className="text-[11px] text-text-muted truncate">
              {messageCount} message{messageCount !== 1 ? 's' : ''} · {wordCount.toLocaleString()} words
              {conversationUsage && conversationUsage.estimated_cost > 0 && (
                <span className="ml-1">· ${conversationUsage.estimated_cost.toFixed(4)}</span>
              )}
              {conversationUsage && conversationUsage.total_input_tokens + conversationUsage.total_output_tokens > 0 && (
                <span className="ml-1">· {((conversationUsage.total_input_tokens + conversationUsage.total_output_tokens) / 1000).toFixed(1)}k tokens</span>
              )}
            </p>
          </div>
        </div>
        <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
          {/* Branch Switcher */}
          {activeId && (
            <BranchSwitcher
              conversationId={activeId}
              activeBranchId={activeBranchId}
              lastMessageId={messages.length > 0 ? messages[messages.length - 1].id : undefined}
              onSwitchBranch={(branchId) => {
                setActiveBranchId(branchId ?? undefined);
                if (branchId) {
                  // Fetch branch messages and update the message store
                  branchApi.listMessages(activeId, branchId).then((msgs) => {
                    if (msgs) useMessageStore.getState().fetchMessages(activeId);
                  }).catch(() => {});
                } else {
                  // Switching back to main — reload original messages
                  useMessageStore.getState().fetchMessages(activeId);
                }
              }}
            />
          )}
          {/* Attachments button */}
          {activeId && (
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => setAttachmentPanelOpen(true)}
              className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-light/50 transition-colors"
              aria-label="Manage attachments"
              title="Manage attachments"
            >
              <Paperclip size={14} />
            </motion.button>
          )}
          {/* Export button */}
          {messages.length > 0 && (
            <div className="relative">
              <motion.button
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                onClick={() => setExportMenuOpen(!exportMenuOpen)}
                className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl hover:bg-surface-hover text-text-muted hover:text-text transition-colors"
                aria-label="Export conversation"
              >
                <Download size={14} />
              </motion.button>
              <AnimatePresence>
                {exportMenuOpen && (
                  <>
                    <div className="fixed inset-0 z-40" onClick={() => setExportMenuOpen(false)} />
                    <motion.div
                      initial={{ opacity: 0, y: -4, scale: 0.96 }}
                      animate={{ opacity: 1, y: 0, scale: 1 }}
                      exit={{ opacity: 0, y: -4, scale: 0.96 }}
                      className="absolute right-0 top-full mt-1 z-50 glass-strong rounded-xl shadow-lg py-1 min-w-[140px]"
                    >
                      <button
                        onClick={() => handleExport('markdown')}
                        className="w-full text-left px-3 py-2 text-xs hover:bg-surface-hover text-text-secondary hover:text-text transition-colors flex items-center gap-2"
                      >
                        <FileText size={12} /> Markdown (.md)
                      </button>
                      <button
                        onClick={() => handleExport('json')}
                        className="w-full text-left px-3 py-2 text-xs hover:bg-surface-hover text-text-secondary hover:text-text transition-colors flex items-center gap-2"
                      >
                        <Download size={12} /> JSON (.json)
                      </button>
                    </motion.div>
                  </>
                )}
              </AnimatePresence>
            </div>
          )}
          <ModelSelector conversationId={activeId} />
        </div>
      </motion.div>

      {/* Messages */}
      <div ref={messagesContainerRef} className="flex-1 overflow-y-auto relative">
        <div className="max-w-3xl xl:max-w-4xl 2xl:max-w-5xl mx-auto px-4 py-6 space-y-6">
          <div className="rounded-xl border border-border bg-surface-alt/60 p-3">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="text-xs font-medium text-text flex items-center gap-2">
                  <Files size={13} className="text-primary" />
                  {activeConvo?.workspace_id ? 'Project Files are enabled for this workspace' : 'Use File Library for this chat'}
                </p>
                <p className="mt-1 text-xs text-text-muted">
                  {activeConvo?.workspace_id
                    ? 'Upload docs once and all chats in this workspace can reference them automatically.'
                    : 'Upload and index docs to ground answers with citations.'}
                </p>
              </div>
              <button
                onClick={openProjectFiles}
                className="shrink-0 rounded-lg border border-border px-2.5 py-1.5 text-xs text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
              >
                Open Project Files
              </button>
            </div>
          </div>

          {messages.length === 0 && !streaming && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="text-center py-16"
            >
              <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-primary/10 to-accent/10 border border-border
                              flex items-center justify-center mx-auto mb-4">
                <Bot size={24} className="text-primary/60" />
              </div>
              <p className="text-text-muted text-sm">Send a message to start the conversation.</p>
              <p className="text-text-muted/50 text-xs mt-1">Shift+Enter for new line · Esc to stop</p>
            </motion.div>
          )}

          <AnimatePresence initial={false}>
            {messages.map((msg, index) => (
              <motion.div
                key={msg.id}
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.35, delay: index > messages.length - 3 ? 0.05 : 0 }}
              >
                <MessageBubble
                  message={msg}
                  isLastAssistant={
                    msg.role === 'assistant' &&
                    index === findLastIndex(messages, (m) => m.role === 'assistant')
                  }
                  conversationId={activeId}
                  onSendToImageStudio={() => setAppMode('image')}
                  onRegenerate={() => regenerateLastMessage(activeId)}
                  onEdit={(newContent) => editAndResend(activeId, msg.id, newContent)}
                  onBranchFromHere={(messageId) => {
                    branchApi.create(activeId, { fork_message_id: messageId, name: `Branch at message ${index + 1}` })
                      .then((branch) => {
                        setActiveBranchId(branch.id);
                        toast.success(`Branch "${branch.name}" created`);
                      })
                      .catch(() => toast.error('Failed to create branch'));
                  }}
                  streaming={streaming}
                />
              </motion.div>
            ))}
          </AnimatePresence>

          {/* URL context indicator */}
          {streaming && urlContextStatus && urlContextStatus !== 'complete' && (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex items-start gap-3 max-w-3xl xl:max-w-4xl 2xl:max-w-5xl min-w-0"
            >
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-amber-500/20 to-orange-500/20 flex items-center justify-center shrink-0 mt-0.5">
                <LinkIcon size={15} className="text-amber-400 animate-pulse" />
              </div>
              <div className="flex min-w-0 flex-col gap-1 px-4 py-3 rounded-2xl bg-surface-alt border border-amber-500/20 rounded-bl-md">
                <div className="flex items-center gap-2">
                  <div className="flex gap-1">
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                  </div>
                  <span className="text-xs text-amber-400">
                    {urlContextStatus === 'detected' ? 'URL detected, reading source…' :
                     urlContextStatus === 'browser_fallback' ? 'Reading linked source with browser…' :
                     urlContextStatus === 'fetching' && urlContextKind === 'github_repo' ? 'Inspecting GitHub repository…' :
                     urlContextStatus === 'fetching' ? 'Reading linked source…' :
                     urlContextStatus === 'indexed' ? 'Indexing source context…' :
                     'Reading linked source…'}
                  </span>
                </div>
              </div>
            </motion.div>
          )}

          {/* Browser tool indicator */}
          {streaming && browserStatus && (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex items-start gap-3 max-w-3xl xl:max-w-4xl 2xl:max-w-5xl min-w-0"
            >
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/20 flex items-center justify-center shrink-0 mt-0.5">
                <Globe size={15} className="text-cyan-400 animate-pulse" />
              </div>
              <div className="flex min-w-0 flex-col gap-1 px-4 py-3 rounded-2xl bg-surface-alt border border-cyan-500/20 rounded-bl-md">
                <div className="flex items-center gap-2">
                  <div className="flex gap-1">
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                  </div>
                  <span className="text-xs text-cyan-400">
                    {browserStatus}
                    {browserProgress !== null && browserProgress !== undefined ? ` ${browserProgress}%` : ''}
                    {browserStatusDetailLabel ? `: ${browserStatusDetailLabel}` : ''}
                  </span>
                </div>
              </div>
            </motion.div>
          )}

          {/* RAG indexing indicator */}
          {streaming && ragIndexingStatus && ragIndexingStatus !== 'complete' && (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex items-start gap-3 max-w-3xl xl:max-w-4xl 2xl:max-w-5xl min-w-0"
            >
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-emerald-500/20 to-teal-500/20 flex items-center justify-center shrink-0 mt-0.5">
                <FileText size={15} className="text-emerald-400 animate-pulse" />
              </div>
              <div className="flex min-w-0 flex-col gap-1 px-4 py-3 rounded-2xl bg-surface-alt border border-emerald-500/20 rounded-bl-md">
                <div className="flex items-center gap-2">
                  <div className="flex gap-1">
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                  </div>
                  <span className="text-xs text-emerald-400">
                    {ragIndexingDetail || 'Reading and understanding the document…'}
                  </span>
                </div>
              </div>
            </motion.div>
          )}

          {/* Image generation indicator */}
          {streaming && imageGenerating && (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex items-start gap-3 max-w-3xl xl:max-w-4xl 2xl:max-w-5xl min-w-0"
            >
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-violet-500/20 to-fuchsia-500/20 flex items-center justify-center shrink-0 mt-0.5">
                <Image size={15} className="text-violet-400 animate-pulse" />
              </div>
              <div className="flex min-w-0 flex-col gap-1 px-4 py-3 rounded-2xl bg-surface-alt border border-violet-500/20 rounded-bl-md">
                <div className="flex items-center gap-2">
                  <div className="flex gap-1">
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                  </div>
                  <span className="text-xs text-violet-400">Generating image…</span>
                </div>
              </div>
            </motion.div>
          )}

          {/* Web search indicator */}
          {streaming && webSearching && (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex items-start gap-3 max-w-3xl xl:max-w-4xl 2xl:max-w-5xl min-w-0"
            >
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-blue-500/20 to-cyan-500/20 flex items-center justify-center shrink-0 mt-0.5">
                <Globe size={15} className="text-blue-400 animate-pulse" />
              </div>
              <div className="flex min-w-0 flex-col gap-1 px-4 py-3 rounded-2xl bg-surface-alt border border-blue-500/20 rounded-bl-md">
                <div className="flex items-center gap-2">
                  <div className="flex gap-1">
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                    <span className="typing-dot" />
                  </div>
                  <span className="text-xs text-blue-400">Searching the web…</span>
                </div>
                {webSearchQuery && (
                  <span className="text-[10px] text-text-muted italic ml-5">
                    &ldquo;{webSearchQuery}&rdquo;
                  </span>
                )}
              </div>
            </motion.div>
          )}

          {/* Streaming thinking (Ollama think mode) */}
          {streaming && streamingThinking && (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex items-start gap-3 max-w-3xl xl:max-w-4xl 2xl:max-w-5xl min-w-0"
            >
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-purple-500/20 to-violet-500/20 flex items-center justify-center shrink-0 mt-0.5">
                <Brain size={15} className="text-purple-400 animate-pulse" />
              </div>
              <details open className="flex-1 min-w-0">
                <summary className="cursor-pointer text-xs text-purple-400 font-medium mb-1 select-none">
                  Thinking…
                </summary>
                <div className="px-4 py-3 rounded-2xl bg-surface-alt border border-purple-500/20 rounded-bl-md text-sm text-text-muted whitespace-pre-wrap break-words max-h-60 overflow-y-auto">
                  {streamingThinking}
                </div>
              </details>
            </motion.div>
          )}

          {/* Streaming content */}
          {streaming && streamingContent && (
            <motion.div
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
            >
              <StreamingBubble content={streamingContent} />
            </motion.div>
          )}

          {/* Thinking indicator */}
          {streaming && !streamingContent && !webSearching && !urlContextStatus && !imageGenerating && (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex items-start gap-3 max-w-3xl xl:max-w-4xl 2xl:max-w-5xl min-w-0"
            >
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center shrink-0 mt-0.5">
                <Bot size={15} className="text-primary" />
              </div>
              <div className="flex items-center gap-2 px-4 py-3 rounded-2xl bg-surface-alt border border-border rounded-bl-md">
                <div className="flex gap-1">
                  <span className="typing-dot" />
                  <span className="typing-dot" />
                  <span className="typing-dot" />
                </div>
                <span className="text-xs text-text-muted ml-1">Thinking...</span>
              </div>
            </motion.div>
          )}

          {/* Error */}
          <AnimatePresence>
            {error && (
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.95 }}
                className="p-4 rounded-2xl bg-danger-soft border border-danger/20 text-danger text-sm flex items-start gap-3"
              >
                <div className="w-6 h-6 rounded-full bg-danger/20 flex items-center justify-center shrink-0 mt-0.5">
                  <span className="text-xs font-bold">!</span>
                </div>
                <div>
                  <p className="font-medium mb-0.5">Something went wrong</p>
                  <p className="text-danger/80 text-xs">{error}</p>
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          <div ref={messagesEndRef} />
        </div>

        {/* Scroll to bottom button */}
        <AnimatePresence>
          {showScrollDown && (
            <motion.button
              initial={{ opacity: 0, scale: 0.8 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.8 }}
              onClick={scrollToBottom}
              className="fixed bottom-28 right-4 sm:right-8 z-20 p-3 rounded-full glass-strong shadow-lg
                         text-text-muted hover:text-text transition-colors border border-border"
            >
              <ArrowDown size={16} />
            </motion.button>
          )}
        </AnimatePresence>
      </div>

      {/* Agent Run View — shows active agent runs inline */}
      {agentMode && activeId && (
        <div className="border-t border-border">
          <AgentRunView conversationId={activeId} />
        </div>
      )}

      {/* Input area */}
      <div className="px-3 py-4 sm:px-4">
        <div className="max-w-3xl xl:max-w-4xl 2xl:max-w-5xl mx-auto">
          {/* Streaming controls */}
          {streaming && (
            <motion.div
              initial={{ opacity: 0, y: 5 }}
              animate={{ opacity: 1, y: 0 }}
              className="flex justify-center mb-2"
            >
              <button
                onClick={stopStreaming}
                className="flex items-center gap-2 px-4 py-1.5 rounded-full text-xs
                           bg-surface-alt border border-border hover:border-danger/30
                           text-text-muted hover:text-danger transition-all"
              >
                <Square size={10} className="fill-current" />
                Stop generating
              </button>
            </motion.div>
          )}

          {/* Composer mode chips */}
          <div className="mb-2 flex flex-col gap-2 px-1 sm:flex-row sm:items-center sm:justify-between">
            <div className="inline-flex w-fit max-w-full flex-wrap items-center gap-1 rounded-xl bg-surface-alt border border-border p-1">
              <button
                onClick={() => setComposerMode('chat')}
                className={clsx(
                  'min-h-8 px-2.5 rounded-lg text-[11px] transition-colors',
                  !imageMode && !agentMode
                    ? 'bg-primary/20 text-primary'
                    : 'text-text-muted hover:text-text'
                )}
              >
                Chat
              </button>
              {isImageCapable && (
              <button
                onClick={() => setComposerMode(imageMode ? 'chat' : 'image')}
                className={clsx(
                  'min-h-8 px-2.5 rounded-lg text-[11px] transition-colors inline-flex items-center gap-1',
                  imageMode
                    ? 'bg-primary/20 text-primary'
                    : 'text-text-muted hover:text-text'
                )}
              >
                <Image size={11} /> Image
              </button>
              )}
              <button
                onClick={() => setComposerMode(agentMode ? 'chat' : 'agent')}
                className={clsx(
                  'min-h-8 px-2.5 rounded-lg text-[11px] transition-colors inline-flex items-center gap-1',
                  agentMode
                    ? 'bg-amber-500/20 text-amber-400'
                    : 'text-text-muted hover:text-text'
                )}
              >
                <Zap size={11} /> Agent
              </button>
            </div>

            {(uploading || pendingFiles.length > 0) && (
              <span className="text-[10px] text-text-muted/70">
                {uploading ? 'Uploading attachments...' : `${pendingFiles.length} file(s) queued`}
              </span>
            )}
          </div>

          <motion.div
            className={clsx(
              'flex flex-col rounded-2xl p-2 transition-all duration-300',
              'bg-surface-alt border',
              inputFocused
                ? 'border-primary/30 shadow-glow'
                : 'border-border hover:border-border-subtle'
            )}
            onDragOver={(e) => { e.preventDefault(); e.stopPropagation(); }}
            onDrop={(e) => {
              e.preventDefault();
              e.stopPropagation();
              queueFiles(Array.from(e.dataTransfer.files));
            }}
          >
            {/* Pending file chips */}
            {pendingFiles.length > 0 && (
              <div className="flex flex-wrap gap-1.5 px-2 pt-1 pb-1">
                {pendingFiles.map((pending) => (
                  <span
                    key={pending.id}
                    className={clsx(
                      'inline-flex items-center gap-1 px-2 py-0.5 rounded-lg text-xs border',
                      pending.status === 'uploading'
                        ? 'bg-blue-500/10 text-blue-300 border-blue-500/20'
                        : pending.status === 'failed'
                          ? 'bg-danger-soft text-danger border-danger/30'
                          : 'bg-surface-hover text-text-muted border-border'
                    )}
                  >
                    {pending.status === 'uploading'
                      ? <RefreshCw size={10} className="animate-spin" />
                      : <Paperclip size={10} />}
                    <span className="max-w-[140px] truncate">{pending.file.name}</span>
                    {pending.status === 'failed' && (
                      <button
                        onClick={() => setPendingFiles((prev) => prev.map((p) => (
                          p.id === pending.id
                            ? { ...p, status: 'pending', error: undefined }
                            : p
                        )))}
                        className="ml-0.5 hover:text-text transition-colors"
                        aria-label={`Retry upload for ${pending.file.name}`}
                        title="Retry upload"
                      >
                        <RefreshCw size={10} />
                      </button>
                    )}
                    <button
                      onClick={() => setPendingFiles((prev) => prev.filter((p) => p.id !== pending.id))}
                      className="ml-0.5 hover:text-danger transition-colors"
                      aria-label={`Remove ${pending.file.name}`}
                    >
                      <X size={10} />
                    </button>
                  </span>
                ))}
              </div>
            )}

            <div className="flex flex-wrap items-end gap-2 sm:flex-nowrap sm:gap-3">
            {/* Hidden file input */}
            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept={FILE_ACCEPT}
              className="hidden"
              onChange={(e) => {
                if (e.target.files) {
                  queueFiles(Array.from(e.target.files));
                }
                e.target.value = '';
              }}
            />

            {/* Paperclip button */}
            <button
              onClick={() => fileInputRef.current?.click()}
              className="order-1 min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl text-text-muted hover:text-text transition-colors shrink-0"
              aria-label="Attach file"
            >
              <Paperclip size={16} />
            </button>

            {/* Template picker */}
            <div className="relative order-1">
              <button
                onClick={() => {
                  if (!templatePickerOpen) {
                    templateApi.list().then((t) => setTemplates(t || [])).catch(() => {});
                  }
                  setTemplatePickerOpen(!templatePickerOpen);
                }}
                className={clsx(
                  'min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl transition-colors shrink-0',
                  templatePickerOpen
                    ? 'bg-primary/20 text-primary'
                    : 'text-text-muted hover:text-text'
                )}
                aria-label="Insert template"
                title="Insert from prompt template"
              >
                <Layout size={16} />
              </button>
              <AnimatePresence>
                {templatePickerOpen && (
                  <>
                    <div className="fixed inset-0 z-40" onClick={() => setTemplatePickerOpen(false)} />
                    <motion.div
                      initial={{ opacity: 0, y: 4, scale: 0.96 }}
                      animate={{ opacity: 1, y: 0, scale: 1 }}
                      exit={{ opacity: 0, y: 4, scale: 0.96 }}
                      className="absolute left-0 bottom-full mb-2 z-50 glass-strong rounded-xl shadow-lg
                                 py-1.5 min-w-[220px] max-h-[200px] overflow-y-auto"
                    >
                      {templates.length === 0 ? (
                        <div className="px-3 py-4 text-center text-xs text-text-muted">
                          No templates found
                        </div>
                      ) : (
                        templates.map((tpl) => (
                          <button
                            key={tpl.id}
                            onClick={() => {
                              setInput(tpl.template_body || '');
                              setTemplatePickerOpen(false);
                              inputRef.current?.focus();
                            }}
                            className="w-full text-left px-3 py-2 text-xs hover:bg-surface-hover
                                       text-text-secondary hover:text-text transition-colors
                                       flex flex-col gap-0.5"
                          >
                            <span className="font-medium truncate">{tpl.name}</span>
                            {tpl.description && (
                              <span className="text-text-muted text-[10px] truncate">{tpl.description}</span>
                            )}
                          </button>
                        ))
                      )}
                    </motion.div>
                  </>
                )}
              </AnimatePresence>
            </div>

            {/* Web search toggle */}
            <button
              onClick={() => toolsSupported && setWebSearchEnabled((v) => !v)}
              disabled={!toolsSupported}
              className={clsx(
                'order-1 min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl transition-colors shrink-0',
                !toolsSupported
                  ? 'opacity-30 cursor-not-allowed text-text-muted'
                  : webSearchEnabled
                  ? 'bg-blue-500/20 text-blue-400'
                  : 'text-text-muted hover:text-text'
              )}
              aria-label={
                !toolsSupported
                  ? 'Web search unavailable — current model does not support tool calling'
                  : webSearchEnabled ? 'Disable web search' : 'Enable web search'
              }
              title={
                !toolsSupported
                  ? `Web search requires function calling. ${activeProvider?.type === 'gemini' ? 'Gemini models do not currently support tool use in this app.' : 'Switch to a model that supports tool calling (e.g. Claude, GPT-4o, or llama3.1 via Ollama).'}`
                  : webSearchEnabled ? 'Web search ON (click to disable)' : 'Web search OFF (click to enable)'
              }
            >
              <Globe size={16} />
            </button>

            {/* Think toggle (Ollama only) */}
            {isOllamaProvider && (
              <button
                onClick={() => setThinkEnabled((v) => !v)}
                className={clsx(
                  'order-1 min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl transition-colors shrink-0',
                  thinkEnabled
                    ? 'bg-purple-500/20 text-purple-400'
                    : 'text-text-muted hover:text-text'
                )}
                aria-label={thinkEnabled ? 'Disable thinking' : 'Enable thinking'}
                title={thinkEnabled ? 'Thinking ON — model will show reasoning (click to disable)' : 'Thinking OFF — click to enable extended reasoning'}
              >
                <Brain size={16} />
              </button>
            )}

            {/* Reasoning effort picker (models that support reasoning_effort: o-series, gpt-5.x, claude-3.7+) */}
            {reasoningLevels && !isOllamaProvider && !imageMode && (
              <div className="order-1 flex items-center gap-0.5 rounded-xl border border-border/50 bg-surface-hover/50 p-0.5" title="Reasoning effort — controls how much the model thinks before responding">
                <button
                  onClick={() => setReasoningEffort(undefined)}
                  className={clsx(
                    'h-8 px-2 rounded-lg text-[11px] font-medium transition-colors',
                    reasoningEffort === undefined
                      ? 'bg-surface text-text shadow-sm'
                      : 'text-text-muted hover:text-text'
                  )}
                  title="Default reasoning (no effort override)"
                >
                  auto
                </button>
                {reasoningLevels.map((level) => (
                  <button
                    key={level}
                    onClick={() => setReasoningEffort(level === reasoningEffort ? undefined : level)}
                    className={clsx(
                      'h-8 px-2 rounded-lg text-[11px] font-medium transition-colors capitalize',
                      reasoningEffort === level
                        ? level === 'low'
                          ? 'bg-sky-500/20 text-sky-400 shadow-sm'
                          : level === 'medium'
                          ? 'bg-amber-500/20 text-amber-400 shadow-sm'
                          : 'bg-purple-500/20 text-purple-400 shadow-sm'
                        : 'text-text-muted hover:text-text'
                    )}
                    title={
                      level === 'low'
                        ? 'Low reasoning — faster, less compute'
                        : level === 'medium'
                        ? 'Medium reasoning — balanced'
                        : 'High reasoning — slower, deeper thinking'
                    }
                  >
                    {level}
                  </button>
                ))}
              </div>
            )}

            <div className="order-2 min-w-0 flex-1 basis-[calc(100%-3rem)] sm:basis-auto relative">
              {imageMode && (
                <div className="mb-1.5 px-3 py-1.5 rounded-lg bg-primary/10 border border-primary/20 text-[11px] text-primary flex items-center gap-1.5">
                  <Image size={12} />
                  <span>
                    {editPreviousImage && lastImageAttachmentId
                      ? 'Editing previous image — describe changes to apply'
                      : 'Image generation mode — provider will use its default image model'}
                  </span>
                  {lastImageAttachmentId && (
                    <button
                      onClick={() => setEditPreviousImage(!editPreviousImage)}
                      className={clsx(
                        'ml-auto px-2 py-0.5 rounded text-[10px] font-medium transition-colors',
                        editPreviousImage
                          ? 'bg-primary/30 text-primary'
                          : 'bg-surface-hover text-text-muted hover:text-text'
                      )}
                    >
                      {editPreviousImage ? 'Edit mode ON' : 'Edit previous'}
                    </button>
                  )}
                </div>
              )}
              <textarea
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                onFocus={() => setInputFocused(true)}
                onBlur={() => setInputFocused(false)}
                placeholder={imageMode ? (editPreviousImage && lastImageAttachmentId ? 'Describe edits to apply to the previous image...' : 'Describe the image you want to generate...') : agentMode ? 'Describe a goal for the agent...' : activeId ? 'Message OmniLLM-Studio...' : 'Start a new conversation...'}
                rows={1}
                className="w-full resize-none px-3 py-2.5 bg-transparent
                           text-sm text-text placeholder-text-muted focus:outline-none
                           max-h-40 overflow-y-auto leading-relaxed"
                style={{ minHeight: '42px' }}
                onInput={(e) => {
                  const target = e.target as HTMLTextAreaElement;
                  target.style.height = 'auto';
                  target.style.height = `${Math.min(target.scrollHeight, 160)}px`;
                }}
              />
            </div>

            {/* Character count for long messages */}
            {input.length > 200 && (
              <span className="order-3 text-[10px] text-text-muted self-center pr-1 sm:order-none">
                {input.length.toLocaleString()}
              </span>
            )}

            <AnimatePresence mode="wait">
              {streaming ? (
                <motion.button
                  key="stop"
                  initial={{ scale: 0.8, opacity: 0 }}
                  animate={{ scale: 1, opacity: 1 }}
                  exit={{ scale: 0.8, opacity: 0 }}
                  onClick={stopStreaming}
                  className="order-2 min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl bg-danger/90 hover:bg-danger text-white
                             transition-colors shrink-0"
                  aria-label="Stop generation (Esc)"
                >
                  <Square size={16} />
                </motion.button>
              ) : (
                <motion.button
                  key="send"
                  initial={{ scale: 0.8, opacity: 0 }}
                  animate={{ scale: 1, opacity: 1 }}
                  exit={{ scale: 0.8, opacity: 0 }}
                  whileHover={input.trim() || pendingFiles.length > 0 ? { scale: 1.05 } : {}}
                  whileTap={input.trim() || pendingFiles.length > 0 ? { scale: 0.95 } : {}}
                  onClick={handleSend}
                  disabled={(!input.trim() && pendingFiles.length === 0) || uploading}
                  className={clsx(
                    'order-2 min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl transition-all duration-200 shrink-0',
                    input.trim() || pendingFiles.length > 0
                      ? 'btn-primary shadow-md shadow-primary/20'
                      : 'bg-surface-hover text-text-muted cursor-not-allowed'
                  )}
                  aria-label="Send message (Enter)"
                >
                  <Send size={16} />
                </motion.button>
              )}
            </AnimatePresence>
          </div>
          </motion.div>

          {/* Input hints */}
          <div className="flex flex-col gap-1 mt-1.5 px-1 sm:flex-row sm:items-center sm:justify-between">
            <span className="text-[10px] text-text-muted/40">
              {imageMode ? (
                <span className="flex items-center gap-1"><Image size={8} /> {editPreviousImage && lastImageAttachmentId ? 'Edit mode · Enter to edit image' : 'Image mode · Enter to generate'}</span>
              ) : agentMode ? (
                <span className="flex items-center gap-1"><Zap size={8} className="text-amber-400" /> Agent mode · Multi-step reasoning</span>
              ) : (
                'Enter to send · Shift+Enter for new line · Esc to stop'
              )}
            </span>
            {activeConvo?.default_model && (
              <span className="text-[10px] text-text-muted/40 flex min-w-0 items-center gap-1">
                <Sparkles size={8} />
                <span className="truncate">{activeConvo.default_model}</span>
                {activeModelIsFree && (
                  <span className="shrink-0 rounded border border-success/25 bg-success/10 px-1 py-0.5 text-[8px] font-bold uppercase tracking-wide text-success/80">
                    FREE
                  </span>
                )}
              </span>
            )}
          </div>
        </div>
      </div>
    </div>

      {/* Attachment Panel */}
      {activeId && (
        <AttachmentPanel
          conversationId={activeId}
          open={attachmentPanelOpen}
          onClose={() => setAttachmentPanelOpen(false)}
        />
      )}
    </>
  );
}

// ============================================
// StreamingBubble - shows streaming content with cursor
// ============================================

function StreamingBubble({ content }: { content: string }) {
  return (
    <div className="flex min-w-0 gap-3 justify-start">
      <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center shrink-0 mt-0.5">
        <Bot size={15} className="text-primary" />
      </div>
      <div className="group relative min-w-0 max-w-[calc(100%-2.75rem)] sm:max-w-[85%] text-sm leading-relaxed rounded-2xl rounded-bl-md px-4 py-3 bg-surface-alt border border-border">
        <MarkdownContent content={content} />
        <span
          className="inline-block w-[2px] h-4 ml-0.5 rounded-full align-middle"
          style={{
            background: 'linear-gradient(to bottom, var(--color-primary), var(--color-accent))',
            animation: 'blink-cursor 1s step-end infinite',
          }}
        />
      </div>
    </div>
  );
}

// ============================================
// MessageBubble - individual message with actions
// ============================================

function MessageBubble({
  message,
  isLastAssistant,
  conversationId,
  onSendToImageStudio,
  onRegenerate,
  onEdit,
  onBranchFromHere,
  streaming,
}: {
  message: Message;
  isLastAssistant: boolean;
  conversationId: string;
  onSendToImageStudio: () => void;
  onRegenerate: () => void;
  onEdit: (newContent: string) => void;
  onBranchFromHere: (messageId: string) => void;
  streaming: boolean;
}) {
  const isUser = message.role === 'user';
  const [copied, setCopied] = useState(false);
  const [sourcesOpen, setSourcesOpen] = useState(false);
  const [fileSourcesOpen, setFileSourcesOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [sendingToStudio, setSendingToStudio] = useState(false);
  const [editContent, setEditContent] = useState(message.content);
  const editRef = useRef<HTMLTextAreaElement>(null);
  const createImageSession = useImageEditorStore((s) => s.createSession);
  const loadAllImageSessions = useImageEditorStore((s) => s.loadAllSessions);
  const loadImageSession = useImageEditorStore((s) => s.loadSession);
  const setImageEditMode = useImageEditorStore((s) => s.setEditMode);
  const setSessionBaseImage = useImageEditorStore((s) => s.setSessionBaseImage);

  // Parse metadata for web search sources
  let metadata: MessageMetadata | null = null;
  if (message.metadata_json) {
    try {
      metadata = JSON.parse(message.metadata_json) as MessageMetadata;
    } catch {
      // ignore parse errors
    }
  }
  const sources: WebSearchResult[] = metadata?.sources ?? [];
  const fileSources: FileSearchResult[] = metadata?.file_sources ?? [];
  const toolCalls: ToolCall[] = metadata?.tool_calls && metadata.tool_calls.length > 0
    ? metadata.tool_calls
    : metadata?.tool_call
      ? [metadata.tool_call]
      : [];
  const browserToolResults: ToolResult[] = metadata?.browser_tool_results ?? [];
  const isImageGenerationMessage = Boolean(metadata?.image_generation) || message.metadata_json?.includes('image_generation') || false;
  const generatedAttachmentIDs = Array.from(new Set(Array.from(
    message.content.matchAll(/\/v1\/attachments\/([a-f0-9-]+)\/download/g)
  ).map((m) => m[1])));
  const primaryGeneratedAttachmentID = generatedAttachmentIDs[0] || null;

  const handleSendToImageStudio = async () => {
    if (!primaryGeneratedAttachmentID || sendingToStudio) return;
    setSendingToStudio(true);
    try {
      const session = await createImageSession(`From chat - ${new Date().toLocaleString()}`);
      if (!session) {
        return;
      }

      const sourceRes = await fetch(`/v1/attachments/${encodeURIComponent(primaryGeneratedAttachmentID)}/download`, {
        credentials: 'include',
      });
      if (!sourceRes.ok) {
        throw new Error(`Failed to fetch image (${sourceRes.status})`);
      }
      const sourceBlob = await sourceRes.blob();
      const sourceType = sourceBlob.type || 'image/png';
      const uploaded = await api.uploadAttachment(
        session.conversation_id,
        new File([sourceBlob], `chat-image-${primaryGeneratedAttachmentID.slice(0, 8)}.png`, { type: sourceType }),
      );

      await imageSessionApi.importAttachment(session.conversation_id, session.id, {
        attachment_id: uploaded.id,
        instruction: 'Imported from chat',
      });

      setSessionBaseImage(session.id, uploaded.id);
      setImageEditMode('edit');

      await loadAllImageSessions();
      await loadImageSession(session.conversation_id, session.id);

      onSendToImageStudio();
      toast.success('Opened in Image Studio');
    } catch (err) {
      toast.error(`Failed to send to Image Studio: ${(err as Error).message}`);
    } finally {
      setSendingToStudio(false);
    }
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(message.content);
    setCopied(true);
    toast.success('Copied to clipboard');
    setTimeout(() => setCopied(false), 2000);
  };

  const startEdit = () => {
    setEditing(true);
    setEditContent(message.content);
    setTimeout(() => {
      editRef.current?.focus();
      editRef.current?.select();
    }, 50);
  };

  const cancelEdit = () => {
    setEditing(false);
    setEditContent(message.content);
  };

  const submitEdit = () => {
    if (editContent.trim() && editContent !== message.content) {
      onEdit(editContent.trim());
    }
    setEditing(false);
  };

  return (
    <div className={clsx('flex min-w-0 gap-3 group/msg', isUser ? 'justify-end' : 'justify-start')}>
      {!isUser && (
        <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center shrink-0 mt-0.5">
          <Bot size={15} className="text-primary" />
        </div>
      )}

      <div
        className={clsx(
          'relative min-w-0 max-w-[calc(100%-2.75rem)] sm:max-w-[85%] text-sm leading-relaxed',
          isUser
            ? 'rounded-2xl rounded-br-md px-4 py-3 bg-gradient-to-br from-primary to-primary-hover text-white shadow-md shadow-primary/10'
            : 'rounded-2xl rounded-bl-md px-4 py-3 bg-surface-alt border border-border'
        )}
      >
        {/* User message editing */}
        {isUser && editing ? (
          <div className="space-y-2">
            <textarea
              ref={editRef}
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submitEdit(); }
                if (e.key === 'Escape') cancelEdit();
              }}
              className="w-full bg-white/10 border border-white/20 rounded-xl px-3 py-2 text-sm text-white
                         placeholder-white/50 focus:outline-none focus:border-white/40 resize-none min-h-[60px]"
              rows={3}
            />
            <div className="flex items-center gap-2 justify-end">
              <button
                onClick={cancelEdit}
                className="px-3 py-1 text-xs rounded-lg bg-white/10 hover:bg-white/20 text-white/80 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={submitEdit}
                className="px-3 py-1 text-xs rounded-lg bg-white/20 hover:bg-white/30 text-white font-medium transition-colors"
              >
                Save & Resend
              </button>
            </div>
          </div>
        ) : isUser ? (
          <div className="whitespace-pre-wrap break-words">{message.content}</div>
        ) : (
          <>
            {/* Saved thinking block (Ollama think mode) */}
            {metadata?.thinking && (
              <details className="mb-3">
                <summary className="cursor-pointer text-xs text-purple-400 font-medium select-none flex items-center gap-1">
                  <Brain size={12} />
                  Thinking
                </summary>
                <div className="mt-1 px-3 py-2 rounded-xl bg-purple-500/5 border border-purple-500/20 text-sm text-text-muted whitespace-pre-wrap break-words max-h-60 overflow-y-auto">
                  {metadata.thinking}
                </div>
              </details>
            )}
            <MarkdownContent content={message.content} />
          </>
        )}

        {/* User message action buttons (hover) */}
        {isUser && !editing && !streaming && (
          <>
            {/* Desktop hover/focus actions */}
            <div className="absolute -left-28 top-1/2 -translate-y-1/2 hidden sm:flex items-center gap-1
                            opacity-0 group-hover/msg:opacity-100 group-focus-within/msg:opacity-100 transition-opacity">
              <button
                onClick={() => onBranchFromHere(message.id)}
                className="p-1.5 rounded-lg bg-surface-alt border border-border text-text-muted
                           hover:text-text hover:border-primary/30 transition-all"
                aria-label="Branch from here"
                title="Branch from here"
              >
                <GitBranch size={12} />
              </button>
              <button
                onClick={startEdit}
                className="p-1.5 rounded-lg bg-surface-alt border border-border text-text-muted
                           hover:text-text hover:border-primary/30 transition-all"
                aria-label="Edit message"
              >
                <Pencil size={12} />
              </button>
              <button
                onClick={handleCopy}
                className="p-1.5 rounded-lg bg-surface-alt border border-border text-text-muted
                           hover:text-text hover:border-primary/30 transition-all"
                aria-label="Copy message"
              >
                {copied ? <Check size={12} className="text-success" /> : <Copy size={12} />}
              </button>
            </div>

            {/* Mobile always-visible actions */}
            <div className="mt-2 flex sm:hidden items-center justify-end gap-1">
              <button
                onClick={() => onBranchFromHere(message.id)}
                className="p-1.5 rounded-lg bg-white/10 border border-white/20 text-white/80
                           hover:text-white hover:bg-white/20 transition-all"
                aria-label="Branch from here"
              >
                <GitBranch size={12} />
              </button>
              <button
                onClick={startEdit}
                className="p-1.5 rounded-lg bg-white/10 border border-white/20 text-white/80
                           hover:text-white hover:bg-white/20 transition-all"
                aria-label="Edit message"
              >
                <Pencil size={12} />
              </button>
              <button
                onClick={handleCopy}
                className="p-1.5 rounded-lg bg-white/10 border border-white/20 text-white/80
                           hover:text-white hover:bg-white/20 transition-all"
                aria-label="Copy message"
              >
                {copied ? <Check size={12} className="text-success" /> : <Copy size={12} />}
              </button>
            </div>
          </>
        )}

        {/* Meta info & actions for assistant messages */}
        {!isUser && (
          <div className="flex items-center justify-between mt-3 pt-2 border-t border-border/50">
            <div className="flex items-center gap-3 text-[11px] text-text-muted">
              {metadata?.web_search && (
                <span className="flex items-center gap-1 text-blue-400">
                  <Globe size={10} />
                  Web
                </span>
              )}
              {metadata?.file_search && (
                <span className="flex items-center gap-1 text-emerald-400">
                  <FileText size={10} />
                  Files
                </span>
              )}
              {metadata?.browser_tool && (
                <span className="flex items-center gap-1 text-cyan-400">
                  <Globe size={10} />
                  Browser
                </span>
              )}
              {metadata?.cost !== undefined && metadata.cost > 0 && (
                <span className="flex items-center gap-1 text-yellow-400">
                  <DollarSign size={10} />
                  {metadata.cost.toFixed(6)}
                </span>
              )}
              {metadata?.thinking && (
                <span className="flex items-center gap-1 text-purple-400">
                  <Brain size={10} />
                  Think
                </span>
              )}
              {message.model && (
                <span className="flex items-center gap-1">
                  <Cpu size={10} />
                  {message.model}
                </span>
              )}
              {message.latency_ms && (
                <span className="flex items-center gap-1">
                  <Clock size={10} />
                  {(message.latency_ms / 1000).toFixed(1)}s
                </span>
              )}
            </div>
            <div className="flex items-center gap-0.5 opacity-100 sm:opacity-0 sm:group-hover/msg:opacity-100 sm:group-focus-within/msg:opacity-100 transition-opacity">
              <button
                onClick={() => onBranchFromHere(message.id)}
                className="p-1 rounded-md hover:bg-surface-hover text-text-muted hover:text-text
                           transition-all"
                aria-label="Branch from here"
                title="Branch from here"
              >
                <GitBranch size={12} />
              </button>
              <button
                onClick={handleCopy}
                className="p-1 rounded-md hover:bg-surface-hover text-text-muted hover:text-text
                           transition-all"
                aria-label="Copy message"
              >
                {copied ? <Check size={12} className="text-success" /> : <Copy size={12} />}
              </button>
              {isImageGenerationMessage && primaryGeneratedAttachmentID && (
                <button
                  onClick={handleSendToImageStudio}
                  disabled={sendingToStudio}
                  className="p-1 rounded-md hover:bg-surface-hover text-text-muted hover:text-text
                             transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                  aria-label="Send to Image Studio"
                  title="Send to Image Studio"
                >
                  <ImagePlus size={12} />
                </button>
              )}
              {isLastAssistant && !streaming && (
                <button
                  onClick={onRegenerate}
                  className="p-1 rounded-md hover:bg-surface-hover text-text-muted hover:text-text
                             transition-all"
                  aria-label="Regenerate response"
                >
                  <RefreshCw size={12} />
                </button>
              )}
            </div>
          </div>
        )}

        {/* Collapsible Sources panel */}
        {!isUser && sources.length > 0 && (
          <div className="mt-2">
            <button
              onClick={() => setSourcesOpen(!sourcesOpen)}
              className="flex items-center gap-1.5 text-[11px] text-text-muted hover:text-text transition-colors"
            >
              <Globe size={10} className="text-blue-400" />
              <span>{sources.length} source{sources.length !== 1 ? 's' : ''} cited</span>
              {sourcesOpen ? <ChevronUp size={10} /> : <ChevronDown size={10} />}
            </button>

            <AnimatePresence>
              {sourcesOpen && (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: 'auto', opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.2 }}
                  className="overflow-hidden"
                >
                  <div className="mt-2 space-y-1.5">
                    {sources.map((src) => (
                      <a
                        key={src.index}
                        href={src.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-start gap-2 p-2.5 rounded-lg bg-surface-hover/50
                                   hover:bg-surface-hover border border-border/30 transition-colors group/source"
                      >
                        <span className="shrink-0 w-5 h-5 rounded bg-blue-500/10 text-blue-400
                                         flex items-center justify-center text-[10px] font-bold mt-0.5">
                          {src.index}
                        </span>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1">
                            <span className="text-[11px] font-medium text-text truncate">
                              {src.title}
                            </span>
                            <ExternalLink size={8} className="shrink-0 text-text-muted opacity-0 group-hover/source:opacity-100 transition-opacity" />
                          </div>
                          <div className="flex items-center gap-1.5 mt-0.5">
                            <span className="text-[10px] text-primary/70 font-medium">
                              {src.source}
                            </span>
                            {src.publishedAt && (
                              <>
                                <span className="text-[10px] text-text-muted">·</span>
                                <span className="text-[10px] text-text-muted">{src.publishedAt}</span>
                              </>
                            )}
                          </div>
                          {src.snippet && (
                            <p className="text-[10px] text-text-muted mt-1 line-clamp-2 leading-relaxed">
                              {src.snippet.substring(0, 150)}{src.snippet.length > 150 ? '…' : ''}
                            </p>
                          )}
                        </div>
                      </a>
                    ))}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}

        {!isUser && fileSources.length > 0 && (
          <div className="mt-2">
            <button
              onClick={() => setFileSourcesOpen(!fileSourcesOpen)}
              className="flex items-center gap-1.5 text-[11px] text-text-muted hover:text-text transition-colors"
            >
              <FileText size={10} className="text-emerald-400" />
              <span>{fileSources.length} file source{fileSources.length !== 1 ? 's' : ''} cited</span>
              {fileSourcesOpen ? <ChevronUp size={10} /> : <ChevronDown size={10} />}
            </button>

            <AnimatePresence>
              {fileSourcesOpen && (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: 'auto', opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.2 }}
                  className="overflow-hidden"
                >
                  <div className="mt-2 space-y-1.5">
                    {fileSources.map((src, idx) => (
                      <div
                        key={`${src.chunk_id}-${idx}`}
                        className="p-2.5 rounded-lg bg-surface-hover/50 border border-border/30"
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-[11px] font-medium text-text truncate">
                            {src.display_name || src.file_name}
                          </span>
                          <span className="text-[10px] text-emerald-400 shrink-0">{src.scope}</span>
                        </div>
                        <div className="text-[10px] text-text-muted mt-1 line-clamp-3 leading-relaxed">
                          {src.snippet}
                        </div>
                      </div>
                    ))}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}

        {/* URL Context Source Panel — shows fetched URL sources under assistant messages */}
        {!isUser && metadata?.url_context && metadata.url_context_sources && metadata.url_context_sources.length > 0 && (
          <div className="mt-2 border-t border-border/30 pt-2">
            <URLContextSourcePanel
              sources={metadata.url_context_sources as URLContextSourceRef[]}
              usedRag={metadata.url_context_used_rag}
              warnings={metadata.url_context_warnings}
            />
          </div>
        )}

        {/* RAG Source Panel — shows indexed document chunks used as context */}
        {!isUser && metadata?.rag_sources && (
          <div className="mt-2 border-t border-border/30 pt-2">
            <RAGSourcePanel
              conversationId={conversationId}
            />
          </div>
        )}

        {/* Tool Call Cards — shows inline tool execution results when available */}
        {!isUser && toolCalls.length > 0 && (
          <div className="mt-2 space-y-1.5 border-t border-border/30 pt-2">
            {toolCalls.map((call, index) => {
              const result = browserToolResults.find((item) => item.tool_call_id && call.id && item.tool_call_id === call.id)
                || browserToolResults[index];
              return (
                <ToolCallCard
                  key={call.id || `${getToolCallName(call)}-${index}`}
                  toolName={getToolCallName(call)}
                  args={getToolCallArgs(call)}
                  result={result}
                  status={result?.is_error ? 'error' : 'success'}
                />
              );
            })}
          </div>
        )}
      </div>

      {isUser && (
        <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-surface-hover to-surface-alt border border-border
                        flex items-center justify-center shrink-0 mt-0.5">
          <User size={15} className="text-text-muted" />
        </div>
      )}
    </div>
  );
}

// ============================================
// Utilities
// ============================================

function findLastIndex<T>(arr: T[], predicate: (item: T) => boolean): number {
  for (let i = arr.length - 1; i >= 0; i--) {
    if (predicate(arr[i])) return i;
  }
  return -1;
}

function downloadFile(filename: string, content: string, mimeType: string) {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
