import { useState, useEffect, useCallback } from 'react';
import { pluginApi } from '../api';
import { motion } from 'framer-motion';
import { toast } from 'sonner';
import { Puzzle, Plus, Trash2, Power, PowerOff, X, FolderOpen, RefreshCw } from 'lucide-react';
import { DialogShell } from './DialogShell';
import type { InstalledPlugin } from '../types';

interface PluginManagerProps {
  open: boolean;
  onClose: () => void;
}

export function PluginManager({ open, onClose }: PluginManagerProps) {
  const [plugins, setPlugins] = useState<InstalledPlugin[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [installing, setInstalling] = useState(false);
  const [installDir, setInstallDir] = useState('');

  const fetchPlugins = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const data = await pluginApi.list();
      setPlugins(data);
    } catch (err) {
      const message = (err as Error).message;
      setLoadError(message);
      toast.error(`Failed to load plugins: ${message}`);
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
    <DialogShell
      open={open}
      onClose={onClose}
      title="Plugin Manager"
      icon={<Puzzle size={18} />}
      maxWidth="max-w-2xl"
      maxHeight="max-h-[80vh]"
      bodyClassName="px-4 py-4 sm:px-6"
      actions={(
        <>
          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            onClick={fetchPlugins}
            className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl text-text-muted hover:text-text hover:bg-surface-light transition-colors"
            aria-label="Refresh plugins"
            title="Refresh plugins"
          >
            <RefreshCw size={14} />
          </motion.button>
          {!installing && (
            <motion.button
              whileHover={{ scale: 1.03 }}
              whileTap={{ scale: 0.97 }}
              onClick={() => setInstalling(true)}
              className="min-h-10 inline-flex items-center gap-1.5 px-3 rounded-xl btn-primary text-xs font-medium"
            >
              <Plus size={14} /> Install
            </motion.button>
          )}
        </>
      )}
    >
            {/* Install form */}
            {installing && (
              <div className="glass rounded-xl p-4 mb-4 space-y-3">
                <div className="flex items-center gap-2 text-sm text-text">
                  <FolderOpen size={14} />
                  <span>Install from directory</span>
                </div>
                <div className="flex flex-col gap-2 sm:flex-row">
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
                    className="min-h-10 px-4 rounded-lg btn-primary text-sm font-medium disabled:opacity-50"
                  >
                    Install
                  </motion.button>
                  <button
                    onClick={() => { setInstalling(false); setInstallDir(''); }}
                    className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
                    aria-label="Cancel plugin install"
                    title="Cancel"
                  >
                    <X size={16} />
                  </button>
                </div>
              </div>
            )}

            {/* Plugin list */}
            {loading ? (
              <div className="py-12 text-center text-text-muted">Loading plugins...</div>
            ) : loadError ? (
              <div className="py-12 text-center">
                <p className="text-sm text-danger">Failed to load plugins</p>
                <p className="text-xs text-text-muted mt-1 break-words">{loadError}</p>
                <button
                  onClick={fetchPlugins}
                  className="mt-4 min-h-10 px-4 rounded-xl glass text-sm text-text hover:bg-surface-hover transition-colors"
                >
                  Retry
                </button>
              </div>
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
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-sm font-medium text-text break-words">{plugin.name}</span>
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
                          <p className="text-xs text-text-muted mt-1 break-words">{plugin.manifest.description}</p>
                        )}
                        {plugin.manifest?.capabilities && (
                          <div className="flex flex-wrap gap-1 mt-2">
                            {plugin.manifest.capabilities.map((cap) => (
                              <span key={cap} className="text-xs px-1.5 py-0.5 rounded bg-primary/10 text-primary">
                                {cap}
                              </span>
                            ))}
                          </div>
                        )}
                        {plugin.manifest?.tools && plugin.manifest.tools.length > 0 && (
                          <div className="mt-2 text-xs text-text-muted break-words">
                            Tools: {plugin.manifest.tools.map((t) => t.name).join(', ')}
                          </div>
                        )}
                      </div>

                      <div className="flex items-center gap-1 sm:ml-3">
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleToggle(plugin.name, plugin.enabled)}
                          className={`min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg transition-colors ${
                            plugin.enabled
                              ? 'text-emerald-400 hover:bg-emerald-400/10'
                              : 'text-gray-400 hover:bg-gray-400/10'
                          }`}
                          aria-label={plugin.enabled ? `Disable ${plugin.name}` : `Enable ${plugin.name}`}
                          title={plugin.enabled ? 'Disable' : 'Enable'}
                        >
                          {plugin.enabled ? <Power size={14} /> : <PowerOff size={14} />}
                        </motion.button>
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleUninstall(plugin.name)}
                          className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-red-400 hover:bg-red-400/10 transition-colors"
                          aria-label={`Uninstall ${plugin.name}`}
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
    </DialogShell>
  );
}
