import { create } from 'zustand';
import { toast } from 'sonner';
import { imageSessionApi } from '../api';
import type {
  ImageSession,
  ImageNode,
  ImageNodeAsset,
  ImageNodeWithMask,
  ImageEditGenerateRequest,
  ImageEditEditRequest,
} from '../types';

// ── Mask stroke types ────────────────────────────────────────────────────

export interface MaskStroke {
  points: { x: number; y: number }[];
  brushSize: number;
  tool: 'brush' | 'eraser';
  feather: number;
}

// ── Store interface ──────────────────────────────────────────────────────

interface ImageEditorState {
  // Session state
  activeSessionId: string | null;
  activeConversationId: string | null;
  sessions: ImageSession[];
  allSessions: ImageSession[];

  // Node graph
  nodes: ImageNode[];
  activeNodeId: string | null;

  // Assets for active node
  activeNodeAssets: ImageNodeAsset[];

  // Tool state
  tool: 'brush' | 'eraser' | 'pan';
  brushSize: number;
  brushFeather: number;
  zoom: number;
  maskVisible: boolean;
  maskOpacity: number;

  // Mask strokes
  maskStrokes: MaskStroke[];
  maskUndoStack: MaskStroke[][];
  maskRedoStack: MaskStroke[][];

  // Node navigation undo/redo
  nodeUndoStack: string[];
  nodeRedoStack: string[];

  // Content/style references for next generate/edit
  contentReferenceIds: string[];
  styleReferenceIds: string[];

  // Loading state
  generating: boolean;
  loadingAssets: boolean;
  error: string | null;

  // Mode
  editMode: 'generate' | 'edit';

  // Actions
  createSession: (title?: string) => Promise<ImageSession | null>;
  loadSession: (conversationId: string, sessionId: string) => Promise<void>;
  loadSessions: (conversationId: string) => Promise<void>;
  loadAllSessions: () => Promise<void>;
  deleteSession: (conversationId: string, sessionId: string) => Promise<void>;
  renameSession: (conversationId: string, sessionId: string, title: string) => Promise<void>;
  generate: (conversationId: string, req: ImageEditGenerateRequest) => Promise<void>;
  edit: (conversationId: string, req: ImageEditEditRequest) => Promise<void>;
  setActiveNode: (nodeId: string) => void;
  loadNodeAssets: (conversationId: string, nodeId: string) => Promise<void>;
  selectVariant: (conversationId: string, nodeId: string, assetId: string) => Promise<void>;

  // Tool actions
  setTool: (tool: 'brush' | 'eraser' | 'pan') => void;
  setBrushSize: (size: number) => void;
  setBrushFeather: (feather: number) => void;
  setZoom: (zoom: number) => void;
  toggleMask: () => void;
  setMaskOpacity: (opacity: number) => void;
  setEditMode: (mode: 'generate' | 'edit') => void;

  // Mask stroke actions
  addMaskStroke: (stroke: MaskStroke) => void;
  undoMaskStroke: () => void;
  redoMaskStroke: () => void;
  clearMask: () => void;

  // Node navigation undo/redo
  undoNodeNavigation: () => void;
  redoNodeNavigation: () => void;
  branchFromNode: (nodeId: string) => void;

  // Reference image actions
  addContentReference: (attachmentId: string) => void;
  removeContentReference: (attachmentId: string) => void;
  addStyleReference: (attachmentId: string) => void;
  removeStyleReference: (attachmentId: string) => void;

  // Reset
  reset: () => void;
}

const initialState = {
  activeSessionId: null as string | null,
  activeConversationId: null as string | null,
  sessions: [] as ImageSession[],
  allSessions: [] as ImageSession[],
  nodes: [] as ImageNode[],
  activeNodeId: null as string | null,
  activeNodeAssets: [] as ImageNodeAsset[],
  tool: 'brush' as const,
  brushSize: 30,
  brushFeather: 0,
  zoom: 1,
  maskVisible: true,
  maskOpacity: 0.5,
  maskStrokes: [] as MaskStroke[],
  maskUndoStack: [] as MaskStroke[][],
  maskRedoStack: [] as MaskStroke[][],
  nodeUndoStack: [] as string[],
  nodeRedoStack: [] as string[],
  contentReferenceIds: [] as string[],
  styleReferenceIds: [] as string[],
  generating: false,
  loadingAssets: false,
  error: null as string | null,
  editMode: 'generate' as const,
};

export const useImageEditorStore = create<ImageEditorState>((set, get) => ({
  ...initialState,

  createSession: async (title) => {
    try {
      const session = await imageSessionApi.create({ title });
      set((s) => ({
        sessions: [session, ...s.sessions],
        allSessions: [session, ...s.allSessions],
        activeSessionId: session.id,
        activeConversationId: session.conversation_id,
        nodes: [],
        activeNodeId: null,
        activeNodeAssets: [],
      }));
      return session;
    } catch (err) {
      toast.error(`Failed to create session: ${(err as Error).message}`);
      return null;
    }
  },

  loadSession: async (conversationId, sessionId) => {
    try {
      const detail = await imageSessionApi.get(conversationId, sessionId);
      // Restore mask strokes from active node if available
      let restoredStrokes: MaskStroke[] = [];
      const activeNode = detail.nodes.find(
        (n: ImageNodeWithMask) => n.id === detail.session.active_node_id
      );
      if (activeNode?.mask?.stroke_json) {
        try {
          restoredStrokes = JSON.parse(activeNode.mask.stroke_json);
        } catch {
          // Ignore malformed stroke_json
        }
      }
      set({
        activeSessionId: detail.session.id,
        activeConversationId: detail.session.conversation_id,
        nodes: detail.nodes,
        activeNodeId: detail.session.active_node_id || null,
        nodeUndoStack: [],
        nodeRedoStack: [],
        maskStrokes: restoredStrokes,
        maskUndoStack: [],
        maskRedoStack: [],
      });
      // Load assets for active node
      if (detail.session.active_node_id) {
        get().loadNodeAssets(conversationId, detail.session.active_node_id);
      }
    } catch (err) {
      toast.error(`Failed to load session: ${(err as Error).message}`);
    }
  },

  loadSessions: async (conversationId) => {
    try {
      const sessions = await imageSessionApi.list(conversationId);
      set({ sessions });
    } catch (err) {
      toast.error(`Failed to load sessions: ${(err as Error).message}`);
    }
  },

  loadAllSessions: async () => {
    try {
      const sessions = await imageSessionApi.listAll();
      set({ allSessions: sessions });
    } catch (err) {
      toast.error(`Failed to load sessions: ${(err as Error).message}`);
    }
  },

  deleteSession: async (conversationId, sessionId) => {
    try {
      await imageSessionApi.delete(conversationId, sessionId);
      set((s) => ({
        sessions: s.sessions.filter((ss) => ss.id !== sessionId),
        allSessions: s.allSessions.filter((ss) => ss.id !== sessionId),
        ...(s.activeSessionId === sessionId
          ? { activeSessionId: null, activeConversationId: null, nodes: [], activeNodeId: null, activeNodeAssets: [] }
          : {}),
      }));
      toast.success('Session deleted');
    } catch (err) {
      toast.error(`Failed to delete session: ${(err as Error).message}`);
    }
  },

  renameSession: async (conversationId, sessionId, title) => {
    try {
      const updated = await imageSessionApi.rename(conversationId, sessionId, title);
      set((s) => ({
        sessions: s.sessions.map((ss) => (ss.id === sessionId ? updated : ss)),
        allSessions: s.allSessions.map((ss) => (ss.id === sessionId ? updated : ss)),
      }));
    } catch (err) {
      toast.error(`Failed to rename session: ${(err as Error).message}`);
    }
  },

  generate: async (conversationId, req) => {
    const { activeSessionId, contentReferenceIds, styleReferenceIds } = get();
    if (!activeSessionId) return;

    set({ generating: true, error: null });
    try {
      const result = await imageSessionApi.generate(conversationId, activeSessionId, {
        ...req,
        reference_image_ids: req.reference_image_ids ?? contentReferenceIds,
        style_reference_ids: req.style_reference_ids ?? styleReferenceIds,
      });
      set((s) => ({
        nodes: [...s.nodes, result.node],
        activeNodeId: result.node.id,
        activeNodeAssets: result.assets,
        generating: false,
        nodeUndoStack: s.activeNodeId ? [...s.nodeUndoStack, s.activeNodeId] : s.nodeUndoStack,
        nodeRedoStack: [],
        maskStrokes: [],
        maskUndoStack: [],
        maskRedoStack: [],
      }));
      get().loadAllSessions();
    } catch (err) {
      set({ generating: false, error: (err as Error).message });
      toast.error(`Generation failed: ${(err as Error).message}`);
    }
  },

  edit: async (conversationId, req) => {
    const { activeSessionId, contentReferenceIds, styleReferenceIds } = get();
    if (!activeSessionId) return;

    set({ generating: true, error: null });
    try {
      const result = await imageSessionApi.edit(conversationId, activeSessionId, {
        ...req,
        reference_image_ids: req.reference_image_ids ?? contentReferenceIds,
        style_reference_ids: req.style_reference_ids ?? styleReferenceIds,
      });
      set((s) => ({
        nodes: [...s.nodes, result.node],
        activeNodeId: result.node.id,
        activeNodeAssets: result.assets,
        generating: false,
        nodeUndoStack: s.activeNodeId ? [...s.nodeUndoStack, s.activeNodeId] : s.nodeUndoStack,
        nodeRedoStack: [],
        maskStrokes: [],
        maskUndoStack: [],
        maskRedoStack: [],
      }));
      get().loadAllSessions();
    } catch (err) {
      set({ generating: false, error: (err as Error).message });
      toast.error(`Edit failed: ${(err as Error).message}`);
    }
  },

  setActiveNode: (nodeId) => {
    const { activeNodeId } = get();
    if (activeNodeId === nodeId) return;
    set((s) => ({
      activeNodeId: nodeId,
      activeNodeAssets: [],
      nodeUndoStack: s.activeNodeId ? [...s.nodeUndoStack, s.activeNodeId] : s.nodeUndoStack,
      nodeRedoStack: [],
      maskStrokes: [],
      maskUndoStack: [],
      maskRedoStack: [],
    }));
  },

  loadNodeAssets: async (conversationId, nodeId) => {
    const { activeSessionId } = get();
    if (!activeSessionId) return;
    set({ loadingAssets: true });
    try {
      const assets = await imageSessionApi.getAssets(conversationId, activeSessionId, nodeId);
      set({ activeNodeAssets: assets, loadingAssets: false });
    } catch (err) {
      set({ loadingAssets: false });
      toast.error(`Failed to load assets: ${(err as Error).message}`);
    }
  },

  selectVariant: async (conversationId, nodeId, assetId) => {
    const { activeSessionId } = get();
    if (!activeSessionId) return;
    try {
      await imageSessionApi.selectVariant(conversationId, activeSessionId, nodeId, assetId);
      set((s) => ({
        activeNodeAssets: s.activeNodeAssets.map((a) => ({
          ...a,
          is_selected: a.id === assetId,
        })),
      }));
    } catch (err) {
      toast.error(`Failed to select variant: ${(err as Error).message}`);
    }
  },

  // Tool actions
  setTool: (tool) => set({ tool }),
  setBrushSize: (size) => set({ brushSize: Math.max(1, Math.min(100, size)) }),
  setBrushFeather: (feather) => set({ brushFeather: Math.max(0, Math.min(100, feather)) }),
  setZoom: (zoom) => set({ zoom: Math.max(0.1, Math.min(10, zoom)) }),
  toggleMask: () => set((s) => ({ maskVisible: !s.maskVisible })),
  setMaskOpacity: (opacity) => set({ maskOpacity: Math.max(0, Math.min(1, opacity)) }),
  setEditMode: (mode) => set({ editMode: mode }),

  // Mask stroke actions
  addMaskStroke: (stroke) =>
    set((s) => ({
      maskStrokes: [...s.maskStrokes, stroke],
      maskUndoStack: [...s.maskUndoStack, s.maskStrokes],
      maskRedoStack: [],
    })),

  undoMaskStroke: () =>
    set((s) => {
      if (s.maskUndoStack.length === 0) return s;
      const prev = s.maskUndoStack[s.maskUndoStack.length - 1];
      return {
        maskStrokes: prev,
        maskUndoStack: s.maskUndoStack.slice(0, -1),
        maskRedoStack: [...s.maskRedoStack, s.maskStrokes],
      };
    }),

  redoMaskStroke: () =>
    set((s) => {
      if (s.maskRedoStack.length === 0) return s;
      const next = s.maskRedoStack[s.maskRedoStack.length - 1];
      return {
        maskStrokes: next,
        maskUndoStack: [...s.maskUndoStack, s.maskStrokes],
        maskRedoStack: s.maskRedoStack.slice(0, -1),
      };
    }),

  clearMask: () =>
    set((s) => ({
      maskStrokes: [],
      maskUndoStack: [...s.maskUndoStack, s.maskStrokes],
      maskRedoStack: [],
    })),

  // Node navigation undo/redo
  undoNodeNavigation: () =>
    set((s) => {
      if (s.nodeUndoStack.length === 0) return s;
      const prev = s.nodeUndoStack[s.nodeUndoStack.length - 1];
      return {
        activeNodeId: prev,
        nodeUndoStack: s.nodeUndoStack.slice(0, -1),
        nodeRedoStack: s.activeNodeId ? [...s.nodeRedoStack, s.activeNodeId] : s.nodeRedoStack,
      };
    }),

  redoNodeNavigation: () =>
    set((s) => {
      if (s.nodeRedoStack.length === 0) return s;
      const next = s.nodeRedoStack[s.nodeRedoStack.length - 1];
      return {
        activeNodeId: next,
        nodeUndoStack: s.activeNodeId ? [...s.nodeUndoStack, s.activeNodeId] : s.nodeUndoStack,
        nodeRedoStack: s.nodeRedoStack.slice(0, -1),
      };
    }),

  branchFromNode: (nodeId) =>
    set((s) => ({
      activeNodeId: nodeId,
      nodeUndoStack: s.activeNodeId ? [...s.nodeUndoStack, s.activeNodeId] : s.nodeUndoStack,
      nodeRedoStack: [],
    })),

  // Reference image actions
  addContentReference: (attachmentId) =>
    set((s) => ({
      contentReferenceIds: s.contentReferenceIds.length < 2
        ? [...s.contentReferenceIds, attachmentId]
        : s.contentReferenceIds,
    })),

  removeContentReference: (attachmentId) =>
    set((s) => ({
      contentReferenceIds: s.contentReferenceIds.filter((id) => id !== attachmentId),
    })),

  addStyleReference: (attachmentId) =>
    set((s) => ({
      styleReferenceIds: s.styleReferenceIds.length < 2
        ? [...s.styleReferenceIds, attachmentId]
        : s.styleReferenceIds,
    })),

  removeStyleReference: (attachmentId) =>
    set((s) => ({
      styleReferenceIds: s.styleReferenceIds.filter((id) => id !== attachmentId),
    })),

  reset: () => set(initialState),
}));
