import { useState, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { Download, Upload, X, FileArchive, AlertTriangle, Check, Loader2 } from 'lucide-react';
import { api } from '../api';
import type { ValidationReport } from '../types';

interface ImportExportPanelProps {
  open: boolean;
  onClose: () => void;
}

export function ImportExportPanel({ open, onClose }: ImportExportPanelProps) {
  const [tab, setTab] = useState<'export' | 'import'>('export');
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validation, setValidation] = useState<ValidationReport | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleExport = async () => {
    setExporting(true);
    try {
      const blob = await api.exportBundle();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      const date = new Date().toISOString().slice(0, 10);
      a.download = `omnillm-studio-backup-${date}.json`;
      a.click();
      URL.revokeObjectURL(url);
      toast.success('Backup exported successfully');
    } catch (err) {
      toast.error(`Export failed: ${(err as Error).message}`);
    } finally {
      setExporting(false);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setSelectedFile(file);
    setValidation(null);
  };

  const handleValidate = async () => {
    if (!selectedFile) return;
    setValidating(true);
    try {
      const result = await api.validateBundle(selectedFile);
      setValidation(result);
    } catch (err) {
      toast.error(`Validation failed: ${(err as Error).message}`);
    } finally {
      setValidating(false);
    }
  };

  const handleImport = async () => {
    if (!selectedFile) return;
    setImporting(true);
    try {
      const result = await api.importBundle(selectedFile);
      toast.success(`Imported ${result.conversations_imported} conversations, ${result.messages_imported} messages`);
      setSelectedFile(null);
      setValidation(null);
    } catch (err) {
      toast.error(`Import failed: ${(err as Error).message}`);
    } finally {
      setImporting(false);
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
          className="glass-strong rounded-2xl w-full max-w-lg max-h-[80vh] overflow-hidden mx-4"
        >
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-border">
            <div className="flex items-center gap-2">
              <FileArchive size={18} className="text-primary" />
              <h2 className="text-lg font-semibold text-text">Import / Export</h2>
            </div>
            <motion.button whileHover={{ scale: 1.1 }} whileTap={{ scale: 0.9 }} onClick={onClose}>
              <X size={18} className="text-text-muted hover:text-text" />
            </motion.button>
          </div>

          {/* Tabs */}
          <div className="flex gap-1 p-1 mx-6 mt-4 glass rounded-xl w-fit">
            <button
              onClick={() => setTab('export')}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                tab === 'export' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
              }`}
            >
              Export
            </button>
            <button
              onClick={() => setTab('import')}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                tab === 'import' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
              }`}
            >
              Import
            </button>
          </div>

          <div className="px-6 py-5 overflow-y-auto">
            {tab === 'export' ? (
              <div className="space-y-4">
                <div className="glass rounded-xl p-4 text-center">
                  <Download size={32} className="mx-auto mb-3 text-primary opacity-70" />
                  <h3 className="text-sm font-medium text-text mb-1">Export Full Backup</h3>
                  <p className="text-xs text-text-muted mb-4">
                    Downloads all conversations, messages, attachments, providers, settings, templates, and workspaces as a single JSON file.
                  </p>
                  <motion.button
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    onClick={handleExport}
                    disabled={exporting}
                    className="px-6 py-2.5 rounded-xl btn-primary text-sm font-medium inline-flex items-center gap-2
                               disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {exporting ? (
                      <>
                        <Loader2 size={14} className="animate-spin" /> Exporting...
                      </>
                    ) : (
                      <>
                        <Download size={14} /> Download Backup
                      </>
                    )}
                  </motion.button>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                {/* File select */}
                <div
                  onClick={() => fileInputRef.current?.click()}
                  className="glass rounded-xl p-6 text-center cursor-pointer hover:bg-white/5 transition-colors
                             border-2 border-dashed border-border hover:border-primary/30"
                >
                  <Upload size={28} className="mx-auto mb-2 text-text-muted" />
                  {selectedFile ? (
                    <div>
                      <p className="text-sm text-text font-medium">{selectedFile.name}</p>
                      <p className="text-xs text-text-muted">
                        {(selectedFile.size / 1024).toFixed(1)} KB — Click to change
                      </p>
                    </div>
                  ) : (
                    <p className="text-sm text-text-muted">Click to select backup JSON file</p>
                  )}
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept=".json"
                    onChange={handleFileSelect}
                    className="hidden"
                  />
                </div>

                {/* Validate button */}
                {selectedFile && !validation && (
                  <motion.button
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    onClick={handleValidate}
                    disabled={validating}
                    className="w-full py-2.5 rounded-xl glass text-sm font-medium text-text flex items-center justify-center gap-2
                               hover:bg-white/5 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {validating ? (
                      <>
                        <Loader2 size={14} className="animate-spin" /> Validating...
                      </>
                    ) : (
                      <>
                        <Check size={14} /> Validate Before Import
                      </>
                    )}
                  </motion.button>
                )}

                {/* Validation results */}
                {validation && (
                  <div className={`rounded-xl p-4 ${validation.valid ? 'glass' : 'bg-red-500/10 border border-red-500/20'}`}>
                    <div className="flex items-center gap-2 mb-2">
                      {validation.valid ? (
                        <Check size={16} className="text-emerald-400" />
                      ) : (
                        <AlertTriangle size={16} className="text-red-400" />
                      )}
                      <span className={`text-sm font-medium ${validation.valid ? 'text-emerald-400' : 'text-red-400'}`}>
                        {validation.valid ? 'Validation passed' : 'Validation failed'}
                      </span>
                    </div>
                    {validation.manifest?.stats && (
                      <div className="grid grid-cols-2 gap-2 mt-2">
                        {Object.entries(validation.manifest.stats).map(([key, val]) => (
                          <div key={key} className="text-xs text-text-muted">
                            <span className="capitalize">{key}:</span>{' '}
                            <span className="text-text font-medium">{val}</span>
                          </div>
                        ))}
                      </div>
                    )}
                    {validation.warnings && validation.warnings.length > 0 && (
                      <div className="mt-2">
                        {validation.warnings.map((w, i) => (
                          <p key={i} className="text-xs text-amber-400">⚠ {w}</p>
                        ))}
                      </div>
                    )}
                    {validation.errors && validation.errors.length > 0 && (
                      <div className="mt-2">
                        {validation.errors.map((e, i) => (
                          <p key={i} className="text-xs text-red-400">✕ {e}</p>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Import button */}
                {selectedFile && validation?.valid && (
                  <motion.button
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    onClick={handleImport}
                    disabled={importing}
                    className="w-full py-2.5 rounded-xl btn-primary text-sm font-medium flex items-center justify-center gap-2
                               disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {importing ? (
                      <>
                        <Loader2 size={14} className="animate-spin" /> Importing...
                      </>
                    ) : (
                      <>
                        <Upload size={14} /> Import Backup
                      </>
                    )}
                  </motion.button>
                )}
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
