import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useImageEditorStore } from '../../stores/imageEditor';
import { useProviderStore, useConversationStore, useMessageStore, useSettingsStore, useCrossoverStore } from '../../stores';
import { useMusicStudioStore } from '../../stores/musicStudio';
import { ImageCanvas, type ImageCanvasHandle } from './ImageCanvas';
import { ImageHistoryPanel } from './ImageHistoryPanel';
import { ImageAdvancedControls } from './ImageAdvancedControls';
import { PromptQualityTips } from './PromptQualityTips';
import { VariantComparePanel } from './VariantComparePanel';
import { useImageEditorShortcuts } from './useImageEditorShortcuts';
import {
  Image, Sparkles, Pencil, X, Undo2, Redo2, Trash2,
  Paintbrush, Eraser, Move, Eye, EyeOff, ImagePlus, XCircle, AlertTriangle,
  PanelLeft, PanelRight, Upload, MessageSquare, Music2,
} from 'lucide-react';
import { clsx } from 'clsx';
import { toast } from 'sonner';
import { imageSessionApi, api, attachmentUrl, uploadAttachment, crossoverApi } from '../../api';
import type { Conversation, ImageCapabilities } from '../../types';

interface ImageEditStudioProps {
  conversationId?: string;
  onClose?: () => void;
}

type MobileImagePanel = 'prompt' | 'canvas' | 'history';

const CHAT_CAPABLE_PROVIDER_TYPES = new Set([
  'openai',
  'anthropic',
  'ollama',
  'openrouter',
  'groq',
  'together',
  'mistral',
  'gemini',
]);

export function ImageEditStudio({ conversationId: propConversationId, onClose }: ImageEditStudioProps = {}) {
  const providers = useProviderStore((s) => s.providers);
  const { createConversation, selectConversation } = useConversationStore();
  const clearMessages = useMessageStore((s) => s.clearMessages);
  const { setAppMode } = useSettingsStore();
  const { crossoverContext, clearCrossoverContext, setCrossoverContext } = useCrossoverStore();
  const createMusicSession = useMusicStudioStore((s) => s.createSession);
  const activeConversationId = useImageEditorStore((s) => s.activeConversationId);
  const conversationId = propConversationId ?? activeConversationId;
  const [sessionConversation, setSessionConversation] = useState<Conversation | null>(null);
  const activeConvo = sessionConversation ?? undefined;

  const {
    activeSessionId,
    sessions,
    nodes,
    activeNodeId,
    activeNodeAssets,
    tool,
    brushSize,
    zoom,
    maskVisible,
    maskOpacity,
    maskStrokes,
    generating,
    loadingAssets,
    error,
    editMode,
    loadSessions,
    generate,
    edit,
    setActiveNode,
    loadNodeAssets,
    selectVariant,
    setTool,
    setBrushSize,
    setZoom,
    toggleMask,
    setMaskOpacity,
    setEditMode,
    undoMaskStroke,
    redoMaskStroke,
    clearMask,
    contentReferenceIds,
    styleReferenceIds,
    sessionBaseImage,
    addContentReference,
    removeContentReference,
    addStyleReference,
    removeStyleReference,
    setSessionBaseImage,
    clearSessionBaseImage,
    branchFromNode,
    undoNodeNavigation,
    redoNodeNavigation,
    nodeUndoStack,
    nodeRedoStack,
  } = useImageEditorStore();

  const [prompt, setPrompt] = useState('');
  const [size, setSize] = useState('1024x1024');
  const [selectedProvider, setSelectedProvider] = useState(activeConvo?.default_provider || '');
  const [selectedImageModel, setSelectedImageModel] = useState('');
  const [seed, setSeed] = useState<number | null>(null);
  const [creativity, setCreativity] = useState(0.5);
  const [variants, setVariants] = useState(1);
  const [compareOpen, setCompareOpen] = useState(false);
  const [leftPanelOpen, setLeftPanelOpen] = useState(true);
  const [rightPanelOpen, setRightPanelOpen] = useState(true);
  const [mobilePanel, setMobilePanel] = useState<MobileImagePanel>('prompt');
  const [enhancingPrompt, setEnhancingPrompt] = useState(false);
  const [lastPromptBeforeEnhance, setLastPromptBeforeEnhance] = useState<string | null>(null);
  const [sendingToChat, setSendingToChat] = useState(false);
  const [generatingSoundtrack, setGeneratingSoundtrack] = useState(false);
  const refInputRef = useRef<HTMLInputElement>(null);
  const pendingRefType = useRef<'content' | 'style'>('content');
  const canvasRef = useRef<ImageCanvasHandle>(null);
  const promptInputRef = useRef<HTMLTextAreaElement>(null);
  const [capabilities, setCapabilities] = useState<ImageCapabilities | null>(null);
  const imageCapableProviders = useMemo(() => providers.filter((p) => p.image_capable && p.enabled), [providers]);
  const prevProviderRef = useRef<string | null>(null);
  const baseImageInputRef = useRef<HTMLInputElement>(null);

  const normalizeProviderId = useCallback((providerValue?: string) => {
    if (!providerValue) {
      return '';
    }
    const match = imageCapableProviders.find((provider) => provider.id === providerValue || provider.name === providerValue);
    return match?.id || providerValue;
  }, [imageCapableProviders]);

  useEffect(() => {
    if (!conversationId) {
      setSessionConversation(null);
      return;
    }
    let cancelled = false;
    api.getConversation(conversationId)
      .then((conversation) => {
        if (!cancelled) {
          setSessionConversation(conversation);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSessionConversation(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [conversationId]);

  // Resync provider when the active conversation changes
  useEffect(() => {
    const defaultProvider = normalizeProviderId(activeConvo?.default_provider);
    if (defaultProvider) {
      setSelectedProvider(defaultProvider);
      return;
    }
    setSelectedProvider((current) => {
      if (current && imageCapableProviders.some((provider) => provider.id === current)) {
        return current;
      }
      return imageCapableProviders[0]?.id || '';
    });
  }, [activeConvo?.default_provider, imageCapableProviders, normalizeProviderId]);

  // Fetch capabilities when provider changes
  useEffect(() => {
    const effectiveProviderId = normalizeProviderId(selectedProvider || activeConvo?.default_provider);
    if (!effectiveProviderId) {
      setCapabilities(null);
      return;
    }
    const p = imageCapableProviders.find((pr) => pr.id === effectiveProviderId);
    if (!p) {
      setCapabilities(null);
      return;
    }
    api.getProviderImageCapabilities(p.id).then((caps) => {
      setCapabilities(caps);
      // Only reset model on actual provider change or when current model is invalid
      const providerChanged = prevProviderRef.current !== p.id;
      const currentModelValid = caps.image_models?.includes(selectedImageModel);
      if (providerChanged || !currentModelValid) {
        setSelectedImageModel(caps.default_image_model || '');
      }
      prevProviderRef.current = p.id;
    }).catch(() => setCapabilities(null));
  }, [selectedProvider, activeConvo?.default_provider, imageCapableProviders, normalizeProviderId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Merge model-level overrides onto provider-level capabilities
  const effectiveCaps = useMemo(() => {
    if (!capabilities) return null;
    const overrides = capabilities.model_overrides?.[selectedImageModel];
    if (!overrides) return capabilities;
    return {
      ...capabilities,
      supported_sizes: overrides.supported_sizes ?? capabilities.supported_sizes,
      max_variants: overrides.max_variants ?? capabilities.max_variants,
      supports_editing: overrides.supports_editing ?? capabilities.supports_editing,
      supports_masking: overrides.supports_masking ?? capabilities.supports_masking,
      supports_content_reference: overrides.supports_content_reference ?? capabilities.supports_content_reference,
    };
  }, [capabilities, selectedImageModel]);

  const selectedProviderProfile = useMemo(
    () => providers.find((provider) => provider.id === selectedProvider || provider.name === selectedProvider),
    [providers, selectedProvider]
  );

  const promptEnhanceOverride = useMemo(() => {
    if (!selectedProviderProfile || !selectedProviderProfile.enabled) {
      return undefined;
    }
    if (!CHAT_CAPABLE_PROVIDER_TYPES.has(selectedProviderProfile.type.toLowerCase())) {
      return undefined;
    }
    return { provider: selectedProviderProfile.id };
  }, [selectedProviderProfile]);

  // Keyboard shortcuts for canvas tools/zoom
  const handleShortcutDownload = useCallback(() => {
    // Trigger download of the current selected asset image
    const asset = activeNodeAssets.find((a) => a.is_selected) || activeNodeAssets[0];
    if (!asset) return;
    const a = document.createElement('a');
    a.href = attachmentUrl(asset.attachment_id);
    a.download = `image-${asset.attachment_id.slice(0, 8)}.png`;
    a.click();
  }, [activeNodeAssets]);

  // Receive crossover context (from Music Studio → Image Studio)
  useEffect(() => {
    if (!crossoverContext || crossoverContext.type !== 'to-image') return;
    setPrompt(crossoverContext.data.prompt);
    clearCrossoverContext();
    toast.success('Prompt pre-filled from Music Studio');
  }, [crossoverContext, clearCrossoverContext]);

  useImageEditorShortcuts({
    enabled: !!activeSessionId,
    onDownload: handleShortcutDownload,
    onZoomChange: setZoom,
    onFitToViewport: () => canvasRef.current?.fitToViewport(),
  });

  // Load sessions on mount / conversation change
  useEffect(() => {
    if (!conversationId) return;
    loadSessions(conversationId);
  }, [conversationId, loadSessions]);

  // Load assets when active node changes
  useEffect(() => {
    if (activeNodeId && activeSessionId && conversationId) {
      loadNodeAssets(conversationId, activeNodeId);
    }
  }, [activeNodeId, activeSessionId, conversationId, loadNodeAssets]);
  const handleGenerate = async () => {
    if (!prompt.trim() || !activeSessionId) return;
    if (imageCapableProviders.length === 0) {
      toast.error('Add an image-capable provider before generating images');
      return;
    }
    await generate(conversationId!, {
      prompt: prompt.trim(),
      size,
      seed: seed ?? undefined,
      creativity,
      n: variants,
      reference_image_ids: contentReferenceIds.length > 0 ? contentReferenceIds : undefined,
      style_reference_ids: styleReferenceIds.length > 0 ? styleReferenceIds : undefined,
      override: selectedProvider ? { provider: selectedProvider, model: selectedImageModel || undefined } : undefined,
    });
    setPrompt('');
    setLastPromptBeforeEnhance(null);
  };

  const handleEdit = async () => {
    if (!prompt.trim() || !activeSessionId) return;
    if (imageCapableProviders.length === 0) {
      toast.error('Add an image-capable provider before editing images');
      return;
    }
    const selectedAsset = activeNodeAssets.find((a) => a.is_selected);
    const sessionBaseImageId = sessionBaseImage?.sessionId === activeSessionId ? sessionBaseImage.attachmentId : null;
    const baseAttachmentId = selectedAsset?.attachment_id ?? sessionBaseImageId;
    if (!baseAttachmentId) {
      toast.error('No base image selected for editing');
      return;
    }

    let maskAttachmentId: string | undefined;

    // If there are mask strokes, export and upload the mask as base64
    if (maskStrokes.length > 0) {
      try {
        const blob = canvasRef.current?.exportMaskBlob?.();
        if (blob) {
          // Convert blob to base64
          const buffer = await blob.arrayBuffer();
          const bytes = new Uint8Array(buffer);
          let binary = '';
          for (let i = 0; i < bytes.length; i++) {
            binary += String.fromCharCode(bytes[i]);
          }
          const base64 = btoa(binary);
          const result = await imageSessionApi.uploadMask(conversationId!, activeSessionId, {
            node_id: activeNodeId || '',
            mask_data: base64,
            stroke_json: JSON.stringify(maskStrokes),
          });
          maskAttachmentId = result.attachment_id;
        }
      } catch (err) {
        toast.error(`Failed to upload mask: ${(err as Error).message}`);
        return;
      }
    }

    await edit(conversationId!, {
      instruction: prompt.trim(),
      base_image_attachment_id: baseAttachmentId,
      mask_attachment_id: maskAttachmentId,
      size,
      n: variants,
      reference_image_ids: contentReferenceIds.length > 0 ? contentReferenceIds : undefined,
      style_reference_ids: styleReferenceIds.length > 0 ? styleReferenceIds : undefined,
      override: selectedProvider ? { provider: selectedProvider, model: selectedImageModel || undefined } : undefined,
    });
    setPrompt('');
    setLastPromptBeforeEnhance(null);
  };

  const handlePromptChange = (value: string) => {
    setPrompt(value);
    setLastPromptBeforeEnhance(null);
  };

  const handleEnhancePrompt = async () => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt || !activeSessionId || enhancingPrompt) return;

    const selectedAssetForEdit = activeNodeAssets.find((a) => a.is_selected);
    const sessionBaseImageIdForEdit = sessionBaseImage?.sessionId === activeSessionId ? sessionBaseImage.attachmentId : null;
    const hasBaseImage = editMode === 'edit' && Boolean(selectedAssetForEdit?.attachment_id ?? sessionBaseImageIdForEdit);

    setEnhancingPrompt(true);
    try {
      const result = await imageSessionApi.enhancePrompt(conversationId!, activeSessionId, {
        prompt: trimmedPrompt,
        mode: editMode,
        size,
        image_model: selectedImageModel || undefined,
        reference_image_count: contentReferenceIds.length,
        style_reference_image_count: styleReferenceIds.length,
        has_base_image: hasBaseImage,
        override: promptEnhanceOverride,
      });
      const enhancedPrompt = result.prompt.trim();
      if (!enhancedPrompt) {
        throw new Error('Prompt enhancement returned an empty prompt');
      }
      setLastPromptBeforeEnhance(prompt);
      setPrompt(enhancedPrompt);
      toast.success('Prompt enhanced');
      requestAnimationFrame(() => promptInputRef.current?.focus());
    } catch (err) {
      toast.error(`Prompt enhancement failed: ${(err as Error).message}`);
    } finally {
      setEnhancingPrompt(false);
    }
  };

  const handleUndoEnhance = () => {
    if (lastPromptBeforeEnhance == null) return;
    setPrompt(lastPromptBeforeEnhance);
    setLastPromptBeforeEnhance(null);
    requestAnimationFrame(() => promptInputRef.current?.focus());
  };

  const handleSubmit = () => {
    if (enhancingPrompt) return;
    if (editMode === 'generate') {
      handleGenerate();
    } else {
      handleEdit();
    }
  };

  const selectedAsset = activeNodeAssets.find((a) => a.is_selected) ?? activeNodeAssets[0];
  const sessionBaseImageId = sessionBaseImage?.sessionId === activeSessionId ? sessionBaseImage.attachmentId : null;
  const canvasAttachmentId = selectedAsset?.attachment_id ?? sessionBaseImageId;
  const loadedFromChatActive = Boolean(sessionBaseImageId) && canvasAttachmentId === sessionBaseImageId;
  const activeNode = nodes.find((n) => n.id === activeNodeId);

  const handleSendToChat = async () => {
    if (!selectedAsset || !conversationId) return;
    setSendingToChat(true);
    try {
      const resp = await fetch(attachmentUrl(selectedAsset.attachment_id));
      const blob = await resp.blob();
      const file = new File([blob], `image-${selectedAsset.attachment_id.slice(0, 8)}.png`, { type: blob.type || 'image/png' });
      const convo = await createConversation(`Image: ${(activeNode?.instruction || 'Studio export').slice(0, 50)}`);
      const uploaded = await uploadAttachment(convo.id, file);
      const content = activeNode?.instruction
        ? `${activeNode.instruction}\n\n![Generated image](/v1/attachments/${uploaded.id}/download)`
        : `![Generated image](/v1/attachments/${uploaded.id}/download)`;
      await api.sendMessage(convo.id, { content });
      selectConversation(convo.id);
      clearMessages();
      setAppMode('chat');
      toast.success('Image sent to chat');
    } catch (err) {
      toast.error(`Failed to send to chat: ${(err as Error).message}`);
    } finally {
      setSendingToChat(false);
    }
  };

  const handleGenerateSoundtrack = async () => {
    if (!activeNode?.instruction) return;
    setGeneratingSoundtrack(true);
    try {
      const [result, session] = await Promise.all([
        crossoverApi.translate.imageToMusic({ prompt: activeNode.instruction }),
        createMusicSession(),
      ]);
      if (!session) {
        toast.error('Could not create music session');
        return;
      }
      setCrossoverContext({ type: 'to-music', data: { ...result, sessionId: session.id } });
      setAppMode('music');
      toast.success('Opening Music Studio with generated prompt');
    } catch (err) {
      toast.error(`Translation failed: ${(err as Error).message}`);
    } finally {
      setGeneratingSoundtrack(false);
    }
  };

  if (!conversationId) {
    return (
      <div className="flex-1 flex items-center justify-center bg-surface">
        <div className="text-center text-text-muted/50 space-y-3">
          <Image size={48} className="mx-auto opacity-30" />
          <p className="text-sm">Select or create an image session to start</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col bg-surface min-h-0">
      {/* Top bar */}
      <div className="flex flex-col gap-2 px-3 py-2 border-b border-border bg-surface-raised shrink-0 sm:flex-row sm:items-center sm:justify-between sm:px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Image size={18} className="text-primary" />
          <span className="text-sm font-medium text-text shrink-0">Image Edit Studio</span>
          {activeSessionId && (
            <span className="min-w-0 truncate text-xs text-text-muted">
              - {sessions.find((s) => s.id === activeSessionId)?.title}
            </span>
          )}
          {loadedFromChatActive && (
            <span className="inline-flex items-center rounded-full border border-primary/30 bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
              Loaded from chat
            </span>
          )}
          {/* Node-level undo/redo */}
          {activeSessionId && (
            <div className="flex items-center gap-0.5 ml-2">
              <button
                onClick={undoNodeNavigation}
                disabled={nodeUndoStack.length === 0}
                className={clsx(
                  'min-h-8 min-w-8 inline-flex items-center justify-center rounded-md transition-colors',
                  nodeUndoStack.length === 0
                    ? 'text-text-muted/20 cursor-not-allowed'
                    : 'text-text-muted hover:text-text hover:bg-surface-hover'
                )}
                aria-label="Undo node navigation"
                title="Undo node navigation"
              >
                <Undo2 size={14} />
              </button>
              <button
                onClick={redoNodeNavigation}
                disabled={nodeRedoStack.length === 0}
                className={clsx(
                  'min-h-8 min-w-8 inline-flex items-center justify-center rounded-md transition-colors',
                  nodeRedoStack.length === 0
                    ? 'text-text-muted/20 cursor-not-allowed'
                    : 'text-text-muted hover:text-text hover:bg-surface-hover'
                )}
                aria-label="Redo node navigation"
                title="Redo node navigation"
              >
                <Redo2 size={14} />
              </button>
            </div>
          )}
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setLeftPanelOpen((v) => !v)}
            className={clsx(
              'hidden lg:inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg transition-colors',
              leftPanelOpen ? 'text-text-muted hover:text-text hover:bg-surface-hover' : 'text-primary bg-primary/10'
            )}
            aria-label={leftPanelOpen ? 'Hide controls' : 'Show controls'}
            title={leftPanelOpen ? 'Hide controls' : 'Show controls'}
          >
            <PanelLeft size={16} />
          </button>
          <button
            onClick={() => setRightPanelOpen((v) => !v)}
            className={clsx(
              'hidden lg:inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg transition-colors',
              rightPanelOpen ? 'text-text-muted hover:text-text hover:bg-surface-hover' : 'text-primary bg-primary/10'
            )}
            aria-label={rightPanelOpen ? 'Hide history' : 'Show history'}
            title={rightPanelOpen ? 'Hide history' : 'Show history'}
          >
            <PanelRight size={16} />
          </button>
          {onClose && (
            <button
              onClick={onClose}
              className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
              aria-label="Close Image Studio"
              title="Close Image Studio"
            >
              <X size={18} />
            </button>
          )}
        </div>
      </div>

      <div className="grid grid-cols-3 gap-1 border-b border-border bg-surface-raised p-2 lg:hidden">
        {([
          { key: 'prompt', label: 'Prompt', Icon: Sparkles },
          { key: 'canvas', label: 'Canvas', Icon: Image },
          { key: 'history', label: 'History', Icon: PanelRight },
        ] as const).map(({ key, label, Icon }) => (
          <button
            key={key}
            onClick={() => setMobilePanel(key)}
            className={clsx(
              'min-h-10 rounded-xl text-xs font-medium transition-colors inline-flex items-center justify-center gap-1.5',
              mobilePanel === key
                ? 'bg-primary/20 text-primary'
                : 'text-text-muted hover:text-text hover:bg-surface-hover'
            )}
          >
            <Icon size={13} />
            {label}
          </button>
        ))}
      </div>

      {/* Main content */}
      <div className="flex flex-1 min-h-0 flex-col lg:flex-row">
        {/* Left panel — generation controls */}
        {(leftPanelOpen || mobilePanel === 'prompt') && (
        <div className={clsx(
          'min-h-0 flex-1 flex-col overflow-y-auto border-border bg-surface-raised',
          'border-b lg:border-b-0 lg:border-r',
          mobilePanel === 'prompt' ? 'flex' : 'hidden',
          leftPanelOpen ? 'lg:flex lg:w-80 lg:flex-none lg:shrink-0' : 'lg:hidden'
        )}>
          <div className="p-4 space-y-4">
            {activeSessionId && (
              <>
                {/* Mode switcher */}
                <div className="flex rounded-xl bg-surface border border-border p-1">
                  <button
                    onClick={() => setEditMode('generate')}
                    className={clsx(
                      'flex-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors flex items-center justify-center gap-1.5',
                      editMode === 'generate'
                        ? 'bg-primary/20 text-primary'
                        : 'text-text-muted hover:text-text'
                    )}
                  >
                    <Sparkles size={12} /> Generate
                  </button>
                  <button
                    onClick={() => setEditMode('edit')}
                    disabled={effectiveCaps != null && !effectiveCaps.supports_editing}
                    className={clsx(
                      'flex-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors flex items-center justify-center gap-1.5',
                      effectiveCaps != null && !effectiveCaps.supports_editing
                        ? 'text-text-muted/30 cursor-not-allowed'
                        : editMode === 'edit'
                          ? 'bg-primary/20 text-primary'
                          : 'text-text-muted hover:text-text'
                    )}
                    title={effectiveCaps != null && !effectiveCaps.supports_editing ? 'Selected provider/model does not support editing' : ''}
                  >
                    <Pencil size={12} /> Edit
                  </button>
                </div>

                {/* Capability warning banner */}
                {editMode === 'edit' && effectiveCaps != null && !effectiveCaps.supports_editing && (
                  <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-400 text-[10px]">
                    <AlertTriangle size={12} className="shrink-0" />
                    Selected provider does not support editing. Switch to a compatible provider.
                  </div>
                )}

                {/* Upload base image for editing (when no generated image selected) */}
                {editMode === 'edit' && !selectedAsset && (
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-text-muted uppercase tracking-wide">Base Image</label>
                    {sessionBaseImageId ? (
                      <div className="flex items-center gap-2">
                        <div className="relative w-12 h-12 rounded-lg overflow-hidden border border-primary/40">
                          <img src={attachmentUrl(sessionBaseImageId)} alt="Base" className="w-full h-full object-cover" />
                        </div>
                        <button
                          onClick={() => activeSessionId && clearSessionBaseImage(activeSessionId)}
                          className="text-[10px] text-danger hover:text-danger/80 transition-colors"
                        >
                          Remove
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => baseImageInputRef.current?.click()}
                        className="w-full flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg
                                   border border-dashed border-border text-text-muted hover:text-text
                                   hover:border-primary/40 text-xs transition-colors"
                      >
                        <Upload size={12} /> Upload image to edit
                      </button>
                    )}
                    <input
                      ref={baseImageInputRef}
                      type="file"
                      accept="image/*"
                      className="hidden"
                      onChange={async (e) => {
                        const file = e.target.files?.[0];
                        if (!file) return;
                        try {
                          const data = await uploadAttachment(conversationId!, file);
                          if (activeSessionId) {
                            setSessionBaseImage(activeSessionId, data.id as string);
                          }
                        } catch {
                          toast.error('Failed to upload base image');
                        }
                        e.target.value = '';
                      }}
                    />
                  </div>
                )}

                {imageCapableProviders.length === 0 && (
                  <div className="flex items-start gap-2 px-3 py-2 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-400 text-[11px]">
                    <AlertTriangle size={13} className="shrink-0 mt-0.5" />
                    <span>Add an image-capable provider in Settings before generating or editing images.</span>
                  </div>
                )}

                {/* Provider / model */}
                {imageCapableProviders.length > 0 && (
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-text-muted uppercase tracking-wide">Provider</label>
                    <select
                      value={selectedProvider}
                      onChange={(e) => {
                        setSelectedProvider(e.target.value);
                      }}
                      className="w-full px-2 py-1.5 text-xs rounded-lg bg-surface border border-border
                                 text-text focus:outline-none focus:border-primary/40"
                    >
                      <option value="">Default</option>
                      {imageCapableProviders.map((p) => (
                        <option key={p.id} value={p.id}>{p.name}</option>
                      ))}
                    </select>
                  </div>
                )}

                {/* Image model picker */}
                {capabilities?.image_models && capabilities.image_models.length >= 1 && (
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-text-muted uppercase tracking-wide">Image Model</label>
                    <select
                      value={selectedImageModel}
                      onChange={(e) => setSelectedImageModel(e.target.value)}
                      className="w-full px-2 py-1.5 text-xs rounded-lg bg-surface border border-border
                                 text-text focus:outline-none focus:border-primary/40"
                    >
                      {capabilities.image_models.map((m) => (
                        <option key={m} value={m}>
                          {m}{m === capabilities.default_image_model ? ' (default)' : ''}
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                {/* Advanced controls (size, seed, creativity, variants) */}
                <ImageAdvancedControls
                  size={size}
                  onSizeChange={setSize}
                  seed={seed}
                  onSeedChange={setSeed}
                  creativity={creativity}
                  onCreativityChange={setCreativity}
                  variants={variants}
                  onVariantsChange={setVariants}
                  supportsSeed={effectiveCaps?.supports_seed}
                  supportsGuidance={effectiveCaps?.supports_guidance}
                  maxVariants={effectiveCaps?.max_variants}
                  supportedSizes={effectiveCaps?.supported_sizes}
                />

                {/* Prompt / instruction */}
                <div className="space-y-1.5">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <label className="text-xs font-medium text-text-muted uppercase tracking-wide">
                      {editMode === 'generate' ? 'Prompt' : 'Instruction'}
                    </label>
                    <div className="flex items-center gap-1">
                      {lastPromptBeforeEnhance != null && (
                        <button
                          type="button"
                          onClick={handleUndoEnhance}
                          className="min-h-8 rounded-lg px-2 text-[10px] font-medium text-text-muted transition-colors hover:bg-surface-hover hover:text-text"
                          aria-label="Undo AI enhance"
                          title="Undo AI enhance"
                        >
                          Undo
                        </button>
                      )}
                      <button
                        type="button"
                        onClick={handleEnhancePrompt}
                        disabled={enhancingPrompt || generating || !prompt.trim()}
                        className={clsx(
                          'min-h-8 rounded-lg px-2.5 text-[10px] font-medium transition-colors inline-flex items-center gap-1.5',
                          enhancingPrompt || generating || !prompt.trim()
                            ? 'bg-surface-hover text-text-muted/40 cursor-not-allowed'
                            : 'bg-primary/10 text-primary hover:bg-primary/20'
                        )}
                        aria-label="AI enhance prompt"
                        title="AI enhance prompt"
                      >
                        {enhancingPrompt ? (
                          <>
                            <span className="h-3 w-3 rounded-full border-2 border-primary/30 border-t-primary animate-spin" />
                            Enhancing...
                          </>
                        ) : (
                          <>
                            <Sparkles size={12} /> AI Enhance
                          </>
                        )}
                      </button>
                    </div>
                  </div>
                  <textarea
                    ref={promptInputRef}
                    value={prompt}
                    onChange={(e) => handlePromptChange(e.target.value)}
                    placeholder={
                      editMode === 'generate'
                        ? 'Describe the image you want to generate...'
                        : 'Describe what to change in the image...'
                    }
                    rows={4}
                    className="w-full px-3 py-2 text-sm rounded-xl bg-surface border border-border
                               text-text placeholder:text-text-muted/50 resize-none
                               focus:outline-none focus:border-primary/40"
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                        e.preventDefault();
                        handleSubmit();
                      }
                    }}
                  />
                  {editMode === 'generate' && <PromptQualityTips prompt={prompt} />}
                </div>

                {/* Reference images */}
                {(effectiveCaps == null || effectiveCaps.supports_content_reference) && (
                <div className="space-y-2">
                  <label className="text-xs font-medium text-text-muted uppercase tracking-wide">
                    Content References {contentReferenceIds.length > 0 && `(${contentReferenceIds.length}/2)`}
                  </label>
                  <div className="flex items-center gap-2 flex-wrap">
                    {contentReferenceIds.map((id) => (
                      <div key={id} className="relative w-12 h-12 rounded-lg overflow-hidden border border-border group">
                        <img
                          src={attachmentUrl(id)}
                          alt="Content ref"
                          className="w-full h-full object-cover"
                        />
                        <button
                          onClick={() => removeContentReference(id)}
                          className="absolute -top-1 -right-1 opacity-0 group-hover:opacity-100 transition-opacity"
                          aria-label="Remove content reference image"
                          title="Remove content reference"
                        >
                          <XCircle size={14} className="text-danger bg-surface rounded-full" />
                        </button>
                      </div>
                    ))}
                    {contentReferenceIds.length < 2 && (
                      <button
                        onClick={() => { pendingRefType.current = 'content'; refInputRef.current?.click(); }}
                        className="w-12 h-12 rounded-lg border border-dashed border-border
                                   text-text-muted hover:text-text hover:border-primary/40
                                   flex items-center justify-center transition-colors"
                        aria-label="Add content reference image"
                        title="Add content reference image"
                      >
                        <ImagePlus size={14} />
                      </button>
                    )}
                  </div>

                  {editMode === 'edit' && (
                    <>
                      <label className="text-xs font-medium text-text-muted uppercase tracking-wide">
                        Style References {styleReferenceIds.length > 0 && `(${styleReferenceIds.length}/2)`}
                      </label>
                      <div className="flex items-center gap-2 flex-wrap">
                        {styleReferenceIds.map((id) => (
                          <div key={id} className="relative w-12 h-12 rounded-lg overflow-hidden border border-accent/30 group">
                            <img
                              src={attachmentUrl(id)}
                              alt="Style ref"
                              className="w-full h-full object-cover"
                            />
                            <button
                              onClick={() => removeStyleReference(id)}
                              className="absolute -top-1 -right-1 opacity-0 group-hover:opacity-100 transition-opacity"
                              aria-label="Remove style reference image"
                              title="Remove style reference"
                            >
                              <XCircle size={14} className="text-danger bg-surface rounded-full" />
                            </button>
                          </div>
                        ))}
                        {styleReferenceIds.length < 2 && (
                          <button
                            onClick={() => { pendingRefType.current = 'style'; refInputRef.current?.click(); }}
                            className="w-12 h-12 rounded-lg border border-dashed border-accent/30
                                       text-text-muted hover:text-accent hover:border-accent/50
                                       flex items-center justify-center transition-colors"
                            aria-label="Add style reference image"
                            title="Add style reference image"
                          >
                            <ImagePlus size={14} />
                          </button>
                        )}
                      </div>
                    </>
                  )}

                  {/* Hidden file input for reference images */}
                  <input
                    ref={refInputRef}
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={async (e) => {
                      const file = e.target.files?.[0];
                      if (!file) return;
                      try {
                        const data = await uploadAttachment(conversationId!, file);
                        const attachmentId = data.id as string;
                        if (pendingRefType.current === 'style') {
                          addStyleReference(attachmentId);
                        } else {
                          addContentReference(attachmentId);
                        }
                      } catch {
                        toast.error('Failed to upload reference image');
                      }
                      e.target.value = '';
                    }}
                  />
                </div>
                )}

                {/* Mask tools (shown in edit mode) */}
                {editMode === 'edit' && (
                  <div className={clsx('space-y-2', effectiveCaps != null && !effectiveCaps.supports_masking && 'opacity-40 pointer-events-none')}>
                    <label className="text-xs font-medium text-text-muted uppercase tracking-wide">
                      Mask Tools
                      {effectiveCaps != null && !effectiveCaps.supports_masking && (
                        <span className="ml-1 text-[9px] text-amber-400 normal-case">(not supported by provider)</span>
                      )}
                    </label>
                    <div className="flex items-center gap-1.5">
                      <button
                        onClick={() => setTool('brush')}
                        className={clsx(
                          'min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg transition-colors',
                          tool === 'brush' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
                        )}
                        aria-label="Brush tool"
                        title="Brush (B)"
                      >
                        <Paintbrush size={14} />
                      </button>
                      <button
                        onClick={() => setTool('eraser')}
                        className={clsx(
                          'min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg transition-colors',
                          tool === 'eraser' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
                        )}
                        aria-label="Eraser tool"
                        title="Eraser (E)"
                      >
                        <Eraser size={14} />
                      </button>
                      <button
                        onClick={() => setTool('pan')}
                        className={clsx(
                          'min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg transition-colors',
                          tool === 'pan' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
                        )}
                        aria-label="Pan tool"
                        title="Pan (Space)"
                      >
                        <Move size={14} />
                      </button>
                      <div className="w-px h-5 bg-border mx-1" />
                      <button
                        onClick={toggleMask}
                        className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
                        aria-label={maskVisible ? 'Hide mask' : 'Show mask'}
                        title="Toggle mask visibility (M)"
                      >
                        {maskVisible ? <Eye size={14} /> : <EyeOff size={14} />}
                      </button>
                    </div>

                    {/* Brush size */}
                    <div className="flex items-center gap-2">
                      <span className="text-[10px] text-text-muted w-12">Size: {brushSize}</span>
                      <input
                        type="range"
                        min={1}
                        max={100}
                        value={brushSize}
                        onChange={(e) => setBrushSize(Number(e.target.value))}
                        className="flex-1 accent-primary"
                      />
                    </div>

                    {/* Mask opacity */}
                    <div className="flex items-center gap-2">
                      <span className="text-[10px] text-text-muted w-12">Opacity</span>
                      <input
                        type="range"
                        min={0}
                        max={100}
                        value={Math.round(maskOpacity * 100)}
                        onChange={(e) => setMaskOpacity(Number(e.target.value) / 100)}
                        className="flex-1 accent-primary"
                      />
                    </div>

                    {/* Mask undo/redo/clear */}
                    <div className="flex items-center gap-1.5">
                      <button
                        onClick={undoMaskStroke}
                        className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
                        aria-label="Undo mask stroke"
                        title="Undo mask stroke (Ctrl+Z)"
                      >
                        <Undo2 size={13} />
                      </button>
                      <button
                        onClick={redoMaskStroke}
                        className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
                        aria-label="Redo mask stroke"
                        title="Redo mask stroke (Ctrl+Shift+Z)"
                      >
                        <Redo2 size={13} />
                      </button>
                      <button
                        onClick={clearMask}
                        disabled={maskStrokes.length === 0}
                        className={clsx(
                          'px-2 py-1 rounded-lg text-[10px] transition-colors inline-flex items-center gap-1',
                          maskStrokes.length === 0
                            ? 'text-text-muted/30 cursor-not-allowed'
                            : 'text-danger hover:bg-danger-soft'
                        )}
                      >
                        <Trash2 size={11} /> Clear mask
                      </button>
                    </div>
                  </div>
                )}

                {/* Generate / Edit button */}
                <button
                  onClick={handleSubmit}
                  disabled={generating || enhancingPrompt || !prompt.trim() || imageCapableProviders.length === 0}
                  className={clsx(
                    'w-full py-2.5 rounded-xl text-sm font-medium transition-all flex items-center justify-center gap-2',
                    generating || enhancingPrompt || !prompt.trim() || imageCapableProviders.length === 0
                      ? 'bg-surface-hover text-text-muted cursor-not-allowed'
                      : 'bg-primary text-white hover:bg-primary-hover shadow-glow'
                  )}
                >
                  {generating ? (
                    <>
                      <div className="w-4 h-4 border-2 border-text-muted/30 border-t-text rounded-full animate-spin" />
                      Generating...
                    </>
                  ) : editMode === 'generate' ? (
                    <>
                      <Sparkles size={14} /> Generate
                    </>
                  ) : (
                    <>
                      <Pencil size={14} /> Apply Edit
                    </>
                  )}
                </button>

                {error && (
                  <p className="text-xs text-danger bg-danger-soft rounded-lg px-3 py-2">{error}</p>
                )}

                {/* Cross-studio actions */}
                {selectedAsset && (
                  <div className="flex items-center gap-2">
                    <button
                      onClick={handleSendToChat}
                      disabled={sendingToChat}
                      className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl
                                 text-xs font-medium text-text-muted bg-surface-hover border border-border
                                 hover:text-text hover:border-primary/40 transition-colors
                                 disabled:opacity-40 disabled:cursor-not-allowed"
                      title="Send current image to a new chat"
                    >
                      <MessageSquare size={12} />
                      {sendingToChat ? 'Sending…' : 'Send to Chat'}
                    </button>
                    <button
                      onClick={handleGenerateSoundtrack}
                      disabled={generatingSoundtrack || !activeNode?.instruction}
                      className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl
                                 text-xs font-medium text-text-muted bg-surface-hover border border-border
                                 hover:text-text hover:border-primary/40 transition-colors
                                 disabled:opacity-40 disabled:cursor-not-allowed"
                      title={activeNode?.instruction ? 'Generate music from this image prompt' : 'Generate an image first'}
                    >
                      <Music2 size={12} />
                      {generatingSoundtrack ? 'Translating…' : 'Soundtrack'}
                    </button>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
        )}

        {/* Center — canvas */}
        <div className={clsx(
          'min-w-0 flex-1 bg-surface items-center justify-center relative overflow-hidden',
          'min-h-[360px] lg:min-h-0',
          mobilePanel === 'canvas' ? 'flex' : 'hidden',
          'lg:flex'
        )}>
          {activeSessionId && canvasAttachmentId ? (
            <ImageCanvas
              ref={canvasRef}
              attachmentId={canvasAttachmentId}
              zoom={zoom}
              onZoomChange={setZoom}
            />
          ) : activeSessionId && loadingAssets ? (
            <div className="text-center text-text-muted/50 space-y-3">
              <div className="mx-auto w-8 h-8 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
              <p className="text-sm">Loading…</p>
            </div>
          ) : (
            <div className="text-center text-text-muted/50 space-y-3">
              <Image size={48} className="mx-auto opacity-30" />
              <p className="text-sm">
                {activeSessionId
                  ? imageCapableProviders.length === 0
                    ? 'Add an image-capable provider to generate images'
                    : 'Generate an image to get started'
                  : 'Create or select a session'}
              </p>
            </div>
          )}

          {/* Variant strip (bottom of canvas area) */}
          {activeNodeAssets.length > 1 && (
            <div className="absolute bottom-0 left-0 right-0 bg-surface-glass backdrop-blur-sm border-t border-border p-2">
              <div className="flex items-center gap-2 overflow-x-auto px-2">
                {activeNodeAssets.map((asset, idx) => (
                  <button
                    key={asset.id}
                    onClick={() => selectVariant(conversationId!, activeNodeId!, asset.id)}
                    aria-label={`Select image variant ${idx + 1}`}
                    className={clsx(
                      'shrink-0 w-16 h-16 rounded-lg border-2 overflow-hidden transition-all',
                      asset.is_selected
                        ? 'border-primary shadow-glow'
                        : 'border-border hover:border-primary/40'
                    )}
                  >
                    <img
                      src={attachmentUrl(asset.attachment_id)}
                      alt={`Variant ${idx + 1}`}
                      className="w-full h-full object-cover"
                    />
                  </button>
                ))}
                <button
                  onClick={() => setCompareOpen(true)}
                  className="shrink-0 px-3 py-1.5 rounded-lg text-[10px] font-medium
                             bg-surface-glass border border-border text-text-muted
                             hover:text-text hover:border-primary/40 transition-colors"
                >
                  Compare
                </button>
              </div>
            </div>
          )}

          {/* Zoom controls removed — now provided by CanvasToolbar in ImageCanvas */}
        </div>

        {/* Right panel — history */}
        {(rightPanelOpen || mobilePanel === 'history') && (
        <div className={clsx(
          'min-h-0 flex-1 overflow-y-auto border-border bg-surface-raised',
          'border-t lg:border-t-0 lg:border-l',
          mobilePanel === 'history' ? 'block' : 'hidden',
          rightPanelOpen ? 'lg:block lg:w-72 lg:flex-none lg:shrink-0' : 'lg:hidden'
        )}>
          <ImageHistoryPanel
            conversationId={conversationId!}
            nodes={nodes}
            activeNodeId={activeNodeId}
            onNodeSelect={setActiveNode}
            onBranchFrom={branchFromNode}
          />
        </div>
        )}
      </div>

      {/* Variant compare overlay */}
      {compareOpen && activeNodeAssets.length > 1 && (
        <VariantComparePanel
          assets={activeNodeAssets}
          onSelect={(assetId) => selectVariant(conversationId!, activeNodeId!, assetId)}
          onClose={() => setCompareOpen(false)}
        />
      )}
    </div>
  );
}
