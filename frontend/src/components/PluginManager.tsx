import { useState, useEffect, useCallback } from 'react';
import { pluginApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { Puzzle, Plus, Trash2, Power, PowerOff, X, FolderOpen, RefreshCw } from 'lucide-react';
import type { InstalledPlugin } from '../types';

interface PluginManagerProps {
  open: boolean;
  onClose: () => void;
}

export function PluginManager({ open, onClose }: PluginManagerProps) {
  const [plugins, setPlugins] = useState<InstalledPlugin[]>([]);
  const [loading, setLoading] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [installDir, setInstallDir] = useState('');

  const fetchPlugins = useCallback(async () => {
    setLoading(true);
    try {
      const data = await pluginApi.list();
      setPlugins(data);
    } catch (err) {
      toast.error(`Failed to load plugins: ${(err as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open) fetchPlugins();
  }, [open, fetchPlugins]);

  const handleInstall = async () => {
    if (!installDir.trim()) return;
    try {
      await pluginApi.install({ directory: installDir });
      toast.success('Plugin installed');
      setInstallDir('');
      setInstalling(false);
      fetchPlugins();
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleToggle = async (name: string, enabled: boolean) => {
    try {
      await pluginApi.update(name, { enabled: !enabled });
      toast.success(enabled ? 'Plugin disabled' : 'Plugin enabled');
      fetchPlugins();
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleUninstall = async (name: string) => {
    try {
      await pluginApi.uninstall(name);
      toast.success('Plugin uninstalled');
      fetchPlugins();
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  if (!open) return null;

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-center justify-center"
        onClick={onClose}
      >
        <motion.div
          initial={{ scale: 0.95, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0.95, opacity: 0 }}
          onClick={(e) => e.stopPropagation()}
          className="glass-strong rounded-2xl w-full max-w-2xl max-h-[80vh] overflow-hidden mx-4"
        >
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-border">
            <div className="flex items-center gap-2">
              <Puzzle size={18} className="text-primary" />
              <h2 className="text-lg font-semibold text-text">Plugin Manager</h2>
            </div>
            <div className="flex items-center gap-2">
              <motion.button
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                onClick={fetchPlugins}
                className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-light transition-colors"
              >
                <RefreshCw size={14} />
              </motion.button>
              {!installing && (
                <motion.button
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={() => setInstalling(true)}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg btn-primary text-xs font-medium"
                >
                  <Plus size={14} /> Install Plugin
                </motion.button>
              )}
              <motion.button whileHover={{ scale: 1.1 }} whileTap={{ scale: 0.9 }} onClick={onClose}>
                <X size={18} className="text-text-muted hover:text-text" />
              </motion.button>
            </div>
          </div>

          {/* Content */}
          <div className="px-6 py-4 overflow-y-auto max-h-[65vh]">
            {/* Install form */}
            {installing && (
              <div className="glass rounded-xl p-4 mb-4 space-y-3">
                <div className="flex items-center gap-2 text-sm text-text">
                  <FolderOpen size={14} />
                  <span>Install from directory</span>
                </div>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={installDir}
                    onChange={(e) => setInstallDir(e.target.value)}
                    placeholder="Path to plugin directory..."
                    className="flex-1 px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                               focus:outline-none focus:border-primary/50"
                    autoFocus
                  />
                  <motion.button
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    onClick={handleInstall}
                    disabled={!installDir.trim()}
                    className="px-4 py-2 rounded-lg btn-primary text-sm font-medium disabled:opacity-50"
                  >
                    Install
                  </motion.button>
                  <button
                    onClick={() => { setInstalling(false); setInstallDir(''); }}
                    className="p-2 text-text-muted hover:text-text"
                  >
                    <X size={16} />
                  </button>
                </div>
              </div>
            )}

            {/* Plugin list */}
            {loading ? (
              <div className="py-12 text-center text-text-muted">Loading plugins...</div>
            ) : plugins.length === 0 ? (
              <div className="py-12 text-center text-text-muted text-sm">
                <Puzzle size={32} className="mx-auto mb-3 opacity-30" />
                <p>No plugins installed</p>
                <p className="text-xs mt-1">Install plugins from a local directory to extend OmniLLM-Studio</p>
              </div>
            ) : (
              <div className="space-y-3">
                {plugins.map((plugin) => (
                  <div key={plugin.name} className="glass rounded-xl p-4 group">
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-text">{plugin.name}</span>
                          <span className="text-xs px-1.5 py-0.5 rounded-full bg-surface-light text-text-muted">
                            v{plugin.version}
                          </span>
                          {plugin.running && (
                            <span className="text-xs px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400">
                              Running
                            </span>
                          )}
                          {!plugin.enabled && (
                            <span className="text-xs px-1.5 py-0.5 rounded-full bg-gray-500/10 text-gray-400">
                              Disabled
                            </span>
                          )}
                        </div>
                        {plugin.manifest?.description && (
                          <p className="text-xs text-text-muted mt-1">{plugin.manifest.description}</p>
                        )}
                        {plugin.manifest?.capabilities && (
                          <div className="flex gap-1 mt-2">
                            {plugin.manifest.capabilities.map((cap) => (
                              <span key={cap} className="text-xs px-1.5 py-0.5 rounded bg-primary/10 text-primary">
                                {cap}
                              </span>
                            ))}
                          </div>
                        )}
                        {plugin.manifest?.tools && plugin.manifest.tools.length > 0 && (
                          <div className="mt-2 text-xs text-text-muted">
                            Tools: {plugin.manifest.tools.map((t) => t.name).join(', ')}
                          </div>
                        )}
                      </div>

                      <div className="flex items-center gap-1 ml-3">
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleToggle(plugin.name, plugin.enabled)}
                          className={`p-2 rounded-lg transition-colors ${
                            plugin.enabled
                              ? 'text-emerald-400 hover:bg-emerald-400/10'
                              : 'text-gray-400 hover:bg-gray-400/10'
                          }`}
                          title={plugin.enabled ? 'Disable' : 'Enable'}
                        >
                          {plugin.enabled ? <Power size={14} /> : <PowerOff size={14} />}
                        </motion.button>
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleUninstall(plugin.name)}
                          className="p-2 rounded-lg text-text-muted hover:text-red-400 hover:bg-red-400/10 transition-colors"
                          title="Uninstall"
                        >
                          <Trash2 size={14} />
                        </motion.button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
