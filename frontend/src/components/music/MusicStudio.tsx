import { useEffect, useMemo } from 'react';
import { Music, PanelRight, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { useConversationStore, useMessageStore, useSettingsStore, useCrossoverStore } from '../../stores';
import { useMusicStudioStore } from '../../stores/musicStudio';
import { useImageEditorStore } from '../../stores/imageEditor';
import { musicApi, api, crossoverApi } from '../../api';
import { MusicPromptBuilder } from './MusicPromptBuilder';
import { MusicResultCard } from './MusicResultCard';
import { MusicHistoryPanel } from './MusicHistoryPanel';
import { MusicAssetDetails } from './MusicAssetDetails';
import { MusicSidebar } from './MusicSidebar';
import type { MusicGenerationDetail, MusicProviderKey } from '../../types/music';

export function MusicStudio() {
  const sessions = useMusicStudioStore((state) => state.sessions);
  const activeSessionId = useMusicStudioStore((state) => state.activeSessionId);
  const activeGenerationId = useMusicStudioStore((state) => state.activeGenerationId);
  const generations = useMusicStudioStore((state) => state.generations);
  const providers = useMusicStudioStore((state) => state.providers);
  const selectedProvider = useMusicStudioStore((state) => state.selectedProvider);
  const selectedModel = useMusicStudioStore((state) => state.selectedModel);
  const modelsByProvider = useMusicStudioStore((state) => state.modelsByProvider);
  const promptForm = useMusicStudioStore((state) => state.promptForm);
  const isGenerating = useMusicStudioStore((state) => state.isGenerating);
  const generationProgress = useMusicStudioStore((state) => state.generationProgress);
  const error = useMusicStudioStore((state) => state.error);
  const loadProviders = useMusicStudioStore((state) => state.loadProviders);
  const loadSessions = useMusicStudioStore((state) => state.loadSessions);
  const createSession = useMusicStudioStore((state) => state.createSession);
  const selectSession = useMusicStudioStore((state) => state.selectSession);
  const deleteSession = useMusicStudioStore((state) => state.deleteSession);
  const setProvider = useMusicStudioStore((state) => state.setProvider);
  const setModel = useMusicStudioStore((state) => state.setModel);
  const setActiveGeneration = useMusicStudioStore((state) => state.setActiveGeneration);
  const setPromptField = useMusicStudioStore((state) => state.setPromptField);
  const setOption = useMusicStudioStore((state) => state.setOption);
  const clearPrompt = useMusicStudioStore((state) => state.clearPrompt);
  const generate = useMusicStudioStore((state) => state.generate);
  const branchFromGeneration = useMusicStudioStore((state) => state.branchFromGeneration);
  const regenerateFromGeneration = useMusicStudioStore((state) => state.regenerateFromGeneration);
  const stopGeneration = useMusicStudioStore((state) => state.stopGeneration);
  const createConversation = useConversationStore((state) => state.createConversation);
  const selectConversation = useConversationStore((state) => state.selectConversation);
  const clearMessages = useMessageStore((state) => state.clearMessages);
  const fetchMessages = useMessageStore((state) => state.fetchMessages);
  const setAppMode = useSettingsStore((state) => state.setAppMode);
  const { crossoverContext, clearCrossoverContext, setCrossoverContext } = useCrossoverStore();
  const createImageSession = useImageEditorStore((s) => s.createSession);

  useEffect(() => {
    loadProviders();
    loadSessions();
  }, [loadProviders, loadSessions]);

  // Receive crossover context (from Chat → Music Studio or Image → Music Studio).
  // The session is always pre-created by the caller before the context is set,
  // so this effect only needs to apply the prompt fields — no async work, no duplicate sessions.
  useEffect(() => {
    if (!crossoverContext || crossoverContext.type !== 'to-music') return;
    const { data } = crossoverContext;
    clearCrossoverContext();
    setPromptField('prompt', data.prompt);
    if (data.genre) setOption('genre', data.genre);
    if (data.mood) setOption('mood', data.mood);
    if (data.instruments) setOption('instruments', data.instruments);
    toast.success('Prompt pre-filled in new session');
  }, [crossoverContext, clearCrossoverContext, setPromptField, setOption]);

  const activeSession = useMemo(
    () => sessions.find((session) => session.id === activeSessionId),
    [sessions, activeSessionId],
  );

  const activeGeneration = useMemo(
    () => generations.find((generation) => generation.id === activeGenerationId) || generations[generations.length - 1],
    [generations, activeGenerationId],
  );

  const models = selectedProvider ? modelsByProvider[selectedProvider] : [];
  const progressMessage = generationProgress?.message;

  const handleDeleteSession = (sessionId: string) => {
    toast('Delete this music session?', {
      action: {
        label: 'Delete',
        onClick: () => deleteSession(sessionId),
      },
      cancel: { label: 'Cancel', onClick: () => {} },
      duration: 5000,
    });
  };

  const handleSendToChat = async (generation: MusicGenerationDetail) => {
    if (!generation.asset_id) {
      toast.error('No audio asset available to send');
      return;
    }
    try {
      const convo = await createConversation(`Music: ${generation.title}`);
      const attachment = await musicApi.attachToConversation(generation.asset_id, convo.id);
      // User messages render as plain text (no markdown), so avoid bracket-link syntax.
      // The audio player in ChatView detects the /v1/attachments/.../download URL from
      // the raw message.content string, so it must be present here.
      const content = [
        generation.prompt && `Music prompt: ${generation.prompt}`,
        generation.assembled_prompt && `Final prompt:\n${generation.assembled_prompt}`,
        `🎵 ${generation.title}\n/v1/attachments/${attachment.id}/download`,
      ].filter(Boolean).join('\n\n');
      await api.sendMessage(convo.id, { content, no_reply: true });
      selectConversation(convo.id);
      clearMessages();
      await fetchMessages(convo.id);
      setAppMode('chat');
      toast.success('Audio sent to chat');
    } catch (err) {
      toast.error(`Failed to send to chat: ${(err as Error).message}`);
    }
  };

  const handleGenerateVideo = async (generation: MusicGenerationDetail) => {
    if (!generation.prompt) return;
    try {
      const result = await crossoverApi.translate.musicToVideo({ prompt: generation.prompt });
      setCrossoverContext({ type: 'to-video', data: { prompt: result.video_prompt } });
      setAppMode('video');
      toast.success('Opening Video Studio with generated prompt');
    } catch (err) {
      toast.error(`Video translation failed: ${(err as Error).message}`);
    }
  };

  const handleGenerateAlbumArt = async (generation: MusicGenerationDetail) => {
    if (!generation.prompt) return;
    try {
      const sessionTitle = `Album Art – ${(generation.title || generation.prompt).slice(0, 48)}`;
      const [result, session] = await Promise.all([
        crossoverApi.translate.musicToImage({ prompt: generation.prompt }),
        createImageSession(sessionTitle),
      ]);
      if (!session) {
        toast.error('Could not create image session');
        return;
      }
      setCrossoverContext({
        type: 'to-image',
        data: { prompt: result.image_prompt, autoGenerate: true, nonce: crypto.randomUUID() },
      });
      setAppMode('image');
      toast.success('Opening Image Studio — generating album art…');
    } catch (err) {
      toast.error(`Album art translation failed: ${(err as Error).message}`);
    }
  };

  return (
    <div className="flex flex-1 min-h-0 flex-col bg-surface">
      <div className="flex flex-col gap-2 border-b border-border bg-surface-raised px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Music size={18} className="text-primary" />
          <span className="shrink-0 text-sm font-medium text-text">Music Studio</span>
          {activeSession && (
            <span className="min-w-0 truncate text-xs text-text-muted">- {activeSession.title}</span>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2 text-[11px] text-text-muted">
          <span className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-alt px-2 py-1">
            <Sparkles size={12} />
            Lyria only
          </span>
          <span className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-alt px-2 py-1">
            <PanelRight size={12} />
            {generations.length} generation{generations.length === 1 ? '' : 's'}
          </span>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-0 xl:grid-cols-[320px_minmax(0,1fr)_320px]">
        <aside className="min-h-0 overflow-y-auto border-b border-border bg-surface xl:border-b-0 xl:border-r">
          <div className="space-y-3 p-3">
            <MusicSidebar
              sessions={sessions}
              activeSessionId={activeSessionId}
              onNew={() => { void createSession(); }}
              onSelect={(sessionId) => { void selectSession(sessionId); }}
              onDelete={handleDeleteSession}
            />
            <MusicPromptBuilder
              providers={providers}
              selectedProvider={selectedProvider}
              selectedModel={selectedModel}
              models={models}
              promptForm={promptForm}
              isGenerating={isGenerating}
              progressMessage={progressMessage}
              error={error}
              onProviderChange={(provider: MusicProviderKey) => { void setProvider(provider); }}
              onModelChange={setModel}
              onRefreshModels={() => selectedProvider && void useMusicStudioStore.getState().loadModels(selectedProvider, true)}
              onPromptField={setPromptField}
              onOption={setOption}
              onGenerate={() => generate()}
              onClear={clearPrompt}
              onStop={stopGeneration}
            />
          </div>
        </aside>

        <main className="min-h-[520px] min-w-0 overflow-hidden bg-surface">
          <MusicResultCard
            generation={activeGeneration}
            isGenerating={isGenerating}
            progressMessage={progressMessage}
            onBranch={(generationId) => { void branchFromGeneration(generationId); }}
            onRegenerate={regenerateFromGeneration}
            onSendToChat={handleSendToChat}
            onGenerateAlbumArt={handleGenerateAlbumArt}
            onSendToVideo={handleGenerateVideo}
          />
        </main>

        <aside className="min-h-0 border-t border-border bg-surface-raised xl:border-l xl:border-t-0">
          <div className="flex h-full min-h-[420px] flex-col">
            <MusicHistoryPanel
              generations={generations}
              activeGenerationId={activeGeneration?.id || null}
              onSelect={setActiveGeneration}
              onBranch={(generationId) => { void branchFromGeneration(generationId); }}
              onRegenerate={regenerateFromGeneration}
            />
            <MusicAssetDetails generation={activeGeneration} />
          </div>
        </aside>
      </div>
    </div>
  );
}
