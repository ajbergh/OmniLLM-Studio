import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useImageEditorStore } from '../../stores/imageEditor';
import { useProviderStore } from '../../stores';
import { ImageCanvas, type ImageCanvasHandle } from './ImageCanvas';
import { ImageHistoryPanel } from './ImageHistoryPanel';
import { ImageAdvancedControls } from './ImageAdvancedControls';
import { PromptQualityTips } from './PromptQualityTips';
import { VariantComparePanel } from './VariantComparePanel';
import { useImageEditorShortcuts } from './useImageEditorShortcuts';
import {
  Image, Sparkles, Pencil, X, Undo2, Redo2, Trash2,
  Paintbrush, Eraser, Move, Eye, EyeOff, ImagePlus, XCircle, AlertTriangle,
  PanelLeft, PanelRight, Upload,
} from 'lucide-react';
import { clsx } from 'clsx';
import { toast } from 'sonner';
import { imageSessionApi, api, attachmentUrl, uploadAttachment } from '../../api';
import type { Conversation, ImageCapabilities } from '../../types';

interface ImageEditStudioProps {
  conversationId?: string;
  onClose?: () => void;
}

export function ImageEditStudio({ conversationId: propConversationId, onClose }: ImageEditStudioProps = {}) {
  const providers = useProviderStore((s) => s.providers);
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
    addContentReference,
    removeContentReference,
    addStyleReference,
    removeStyleReference,
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
  const refInputRef = useRef<HTMLInputElement>(null);
  const pendingRefType = useRef<'content' | 'style'>('content');
  const canvasRef = useRef<ImageCanvasHandle>(null);
  const [capabilities, setCapabilities] = useState<ImageCapabilities | null>(null);
  const imageCapableProviders = providers.filter((p) => p.image_capable && p.enabled);
  const prevProviderRef = useRef<string | null>(null);
  const baseImageInputRef = useRef<HTMLInputElement>(null);
  const [uploadedBaseImageId, setUploadedBaseImageId] = useState<string | null>(null);

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
  };

  const handleEdit = async () => {
    if (!prompt.trim() || !activeSessionId) return;
    const selectedAsset = activeNodeAssets.find((a) => a.is_selected);
    const baseAttachmentId = selectedAsset?.attachment_id ?? uploadedBaseImageId;
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
  };

  const handleSubmit = () => {
    if (editMode === 'generate') {
      handleGenerate();
    } else {
      handleEdit();
    }
  };

  const selectedAsset = activeNodeAssets.find((a) => a.is_selected) ?? activeNodeAssets[0];

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
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-surface-raised shrink-0">
        <div className="flex items-center gap-3">
          <Image size={18} className="text-primary" />
          <span className="text-sm font-medium text-text">Image Edit Studio</span>
          {activeSessionId && (
            <span className="text-xs text-text-muted">
              — {sessions.find((s) => s.id === activeSessionId)?.title}
            </span>
          )}
          {/* Node-level undo/redo */}
          {activeSessionId && (
            <div className="flex items-center gap-0.5 ml-2">
              <button
                onClick={undoNodeNavigation}
                disabled={nodeUndoStack.length === 0}
                className={clsx(
                  'p-1 rounded-md transition-colors',
                  nodeUndoStack.length === 0
                    ? 'text-text-muted/20 cursor-not-allowed'
                    : 'text-text-muted hover:text-text hover:bg-surface-hover'
                )}
                title="Undo node navigation"
              >
                <Undo2 size={14} />
              </button>
              <button
                onClick={redoNodeNavigation}
                disabled={nodeRedoStack.length === 0}
                className={clsx(
                  'p-1 rounded-md transition-colors',
                  nodeRedoStack.length === 0
                    ? 'text-text-muted/20 cursor-not-allowed'
                    : 'text-text-muted hover:text-text hover:bg-surface-hover'
                )}
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
              'p-1.5 rounded-lg transition-colors',
              leftPanelOpen ? 'text-text-muted hover:text-text hover:bg-surface-hover' : 'text-primary bg-primary/10'
            )}
            title={leftPanelOpen ? 'Hide controls' : 'Show controls'}
          >
            <PanelLeft size={16} />
          </button>
          <button
            onClick={() => setRightPanelOpen((v) => !v)}
            className={clsx(
              'p-1.5 rounded-lg transition-colors',
              rightPanelOpen ? 'text-text-muted hover:text-text hover:bg-surface-hover' : 'text-primary bg-primary/10'
            )}
            title={rightPanelOpen ? 'Hide history' : 'Show history'}
          >
            <PanelRight size={16} />
          </button>
          {onClose && (
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
            >
              <X size={18} />
            </button>
          )}
        </div>
      </div>

      {/* Main content — 3-column layout */}
      <div className="flex flex-1 min-h-0">
        {/* Left panel — generation controls */}
        {leftPanelOpen && (
        <div className="w-80 border-r border-border bg-surface-raised flex flex-col shrink-0 overflow-y-auto">
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
                    {uploadedBaseImageId ? (
                      <div className="flex items-center gap-2">
                        <div className="relative w-12 h-12 rounded-lg overflow-hidden border border-primary/40">
                          <img src={attachmentUrl(uploadedBaseImageId)} alt="Base" className="w-full h-full object-cover" />
                        </div>
                        <button
                          onClick={() => setUploadedBaseImageId(null)}
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
                          setUploadedBaseImageId(data.id as string);
                        } catch {
                          toast.error('Failed to upload base image');
                        }
                        e.target.value = '';
                      }}
                    />
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
                  <label className="text-xs font-medium text-text-muted uppercase tracking-wide">
                    {editMode === 'generate' ? 'Prompt' : 'Instruction'}
                  </label>
                  <textarea
                    value={prompt}
                    onChange={(e) => setPrompt(e.target.value)}
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
                          'p-2 rounded-lg transition-colors',
                          tool === 'brush' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
                        )}
                        title="Brush (B)"
                      >
                        <Paintbrush size={14} />
                      </button>
                      <button
                        onClick={() => setTool('eraser')}
                        className={clsx(
                          'p-2 rounded-lg transition-colors',
                          tool === 'eraser' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
                        )}
                        title="Eraser (E)"
                      >
                        <Eraser size={14} />
                      </button>
                      <button
                        onClick={() => setTool('pan')}
                        className={clsx(
                          'p-2 rounded-lg transition-colors',
                          tool === 'pan' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
                        )}
                        title="Pan (Space)"
                      >
                        <Move size={14} />
                      </button>
                      <div className="w-px h-5 bg-border mx-1" />
                      <button
                        onClick={toggleMask}
                        className="p-2 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
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
                        className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
                        title="Undo mask stroke (Ctrl+Z)"
                      >
                        <Undo2 size={13} />
                      </button>
                      <button
                        onClick={redoMaskStroke}
                        className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
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
                  disabled={generating || !prompt.trim()}
                  className={clsx(
                    'w-full py-2.5 rounded-xl text-sm font-medium transition-all flex items-center justify-center gap-2',
                    generating || !prompt.trim()
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
              </>
            )}
          </div>
        </div>
        )}

        {/* Center — canvas */}
        <div className="flex-1 min-w-0 bg-surface flex items-center justify-center relative overflow-hidden">
          {activeSessionId && selectedAsset ? (
            <ImageCanvas
              ref={canvasRef}
              attachmentId={selectedAsset.attachment_id}
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
                  ? 'Generate an image to get started'
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
        {rightPanelOpen && (
        <div className="w-72 border-l border-border bg-surface-raised shrink-0 overflow-y-auto">
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
