import { useState, useEffect, useCallback, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, Command, RotateCcw } from 'lucide-react';
import {
  type ShortcutId,
  getShortcutBindings,
  getShortcutDisplayKeys,
  formatBindingKeys,
  updateBinding,
  resetBinding,
  resetAllBindings,
  isCustomized,
} from '../shortcuts';

interface Props {
  open: boolean;
  onClose: () => void;
}

export function KeyboardShortcuts({ open, onClose }: Props) {
  const [recordingId, setRecordingId] = useState<ShortcutId | null>(null);
  const [pendingKeys, setPendingKeys] = useState<{ key: string; requiresMod?: boolean; shift?: boolean } | null>(null);
  const [version, setVersion] = useState(0); // force re-render after binding changes
  const recordRef = useRef<HTMLDivElement>(null);

  const bindings = getShortcutBindings();

  const commitBinding = useCallback((id: ShortcutId, key: string, requiresMod?: boolean, shift?: boolean) => {
    updateBinding(id, key, requiresMod, shift);
    setRecordingId(null);
    setPendingKeys(null);
    setVersion((v) => v + 1);
  }, []);

  const cancelRecording = useCallback(() => {
    setRecordingId(null);
    setPendingKeys(null);
  }, []);

  // Keyboard listener for recording
  useEffect(() => {
    if (!recordingId) return;

    const handler = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();

      // Ignore bare modifier keys
      if (['Control', 'Meta', 'Alt', 'Shift'].includes(e.key)) return;

      // Escape cancels recording
      if (e.key === 'Escape' && !e.ctrlKey && !e.metaKey && !e.shiftKey) {
        cancelRecording();
        return;
      }

      const hasMod = e.ctrlKey || e.metaKey;
      const hasShift = e.shiftKey;
      const key = e.key;

      setPendingKeys({ key, requiresMod: hasMod || undefined, shift: hasShift || undefined });
      commitBinding(recordingId, key, hasMod || undefined, hasShift || undefined);
    };

    window.addEventListener('keydown', handler, true);
    return () => window.removeEventListener('keydown', handler, true);
  }, [recordingId, commitBinding, cancelRecording]);

  // Click outside cancels recording
  useEffect(() => {
    if (!recordingId) return;
    const handler = (e: MouseEvent) => {
      if (recordRef.current && !recordRef.current.contains(e.target as Node)) {
        cancelRecording();
      }
    };
    window.addEventListener('mousedown', handler);
    return () => window.removeEventListener('mousedown', handler);
  }, [recordingId, cancelRecording]);

  const handleResetAll = () => {
    resetAllBindings();
    cancelRecording();
    setVersion((v) => v + 1);
  };

  const handleResetOne = (id: ShortcutId) => {
    resetBinding(id);
    cancelRecording();
    setVersion((v) => v + 1);
  };

  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50"
            onClick={() => { cancelRecording(); onClose(); }}
          />
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
            className="fixed inset-0 z-50 flex items-center justify-center pointer-events-none"
          >
            <div
              ref={recordRef}
              role="dialog"
              aria-modal="true"
              aria-label="Keyboard Shortcuts"
              className="glass-strong rounded-2xl shadow-2xl border border-border w-full max-w-md mx-4 pointer-events-auto"
            >
              {/* Header */}
              <div className="flex items-center justify-between px-5 py-4 border-b border-border">
                <div className="flex items-center gap-2.5">
                  <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center">
                    <Command size={14} className="text-primary" />
                  </div>
                  <h2 className="text-sm font-semibold text-text">Keyboard Shortcuts</h2>
                </div>
                <div className="flex items-center gap-1.5">
                  <button
                    onClick={handleResetAll}
                    className="p-1.5 rounded-lg hover:bg-surface-hover text-text-muted hover:text-text transition-colors"
                    title="Reset all to defaults"
                  >
                    <RotateCcw size={14} />
                  </button>
                  <button
                    onClick={() => { cancelRecording(); onClose(); }}
                    className="p-1.5 rounded-lg hover:bg-surface-hover text-text-muted hover:text-text transition-colors"
                  >
                    <X size={16} />
                  </button>
                </div>
              </div>

              {/* Shortcuts list */}
              <div className="p-4 space-y-1 max-h-[60vh] overflow-y-auto">
                {bindings.map((shortcut) => {
                  const isRecording = recordingId === shortcut.id;
                  const custom = isCustomized(shortcut.id);
                  const keys = isRecording && pendingKeys
                    ? formatBindingKeys(pendingKeys)
                    : getShortcutDisplayKeys(shortcut.id);

                  return (
                    <div
                      key={shortcut.id + '-' + version}
                      className={`flex items-center justify-between py-2.5 px-3 rounded-xl transition-colors ${
                        isRecording
                          ? 'bg-primary/10 ring-1 ring-primary/30'
                          : 'hover:bg-surface-hover/50'
                      }`}
                    >
                      <span className={`text-sm ${custom ? 'text-primary' : 'text-text-secondary'}`}>
                        {shortcut.description}
                        {custom && <span className="ml-1.5 text-[10px] text-primary/60">(custom)</span>}
                      </span>
                      <div className="flex items-center gap-2">
                        {custom && !isRecording && (
                          <button
                            onClick={(e) => { e.stopPropagation(); handleResetOne(shortcut.id); }}
                            className="p-1 rounded hover:bg-surface-hover text-text-muted hover:text-text transition-colors"
                            title="Reset to default"
                          >
                            <RotateCcw size={11} />
                          </button>
                        )}
                        <button
                          onClick={() => {
                            if (isRecording) {
                              cancelRecording();
                            } else {
                              setPendingKeys(null);
                              setRecordingId(shortcut.id);
                            }
                          }}
                          className="flex items-center gap-1 group cursor-pointer"
                          title={isRecording ? 'Press a key combo or Esc to cancel' : 'Click to change shortcut'}
                        >
                          {isRecording ? (
                            <span className="px-3 py-1 rounded-md border border-primary/40 bg-primary/5 text-[11px] font-mono text-primary animate-pulse">
                              Press keys…
                            </span>
                          ) : (
                            keys.map((key, i) => (
                              <span key={i}>
                                <kbd className="px-2 py-1 rounded-md bg-surface-alt border border-border text-[11px]
                                               font-mono text-text-muted min-w-[28px] inline-flex items-center justify-center
                                               group-hover:border-primary/40 group-hover:text-text transition-colors">
                                  {key}
                                </kbd>
                                {i < keys.length - 1 && (
                                  <span className="text-text-muted/30 mx-0.5 text-[10px]">+</span>
                                )}
                              </span>
                            ))
                          )}
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>

              {/* Footer */}
              <div className="px-5 py-3 border-t border-border text-center">
                <span className="text-[11px] text-text-muted">
                  Click a shortcut to change it · Press <kbd className="px-1.5 py-0.5 rounded bg-surface-alt border border-border text-[10px] font-mono mx-1">Esc</kbd> to close
                </span>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
