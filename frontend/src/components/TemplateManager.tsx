import { useState, useEffect, useCallback } from 'react';
import { templateApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { FileText, Plus, Trash2, Edit3, Copy, Save, ChevronRight, Search } from 'lucide-react';
import { DialogShell } from './DialogShell';
import type { PromptTemplate } from '../types';

interface TemplateManagerProps {
  open: boolean;
  onClose: () => void;
}

export function TemplateManager({ open, onClose }: TemplateManagerProps) {
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [editing, setEditing] = useState<PromptTemplate | null>(null);
  const [creating, setCreating] = useState(false);
  const [formName, setFormName] = useState('');
  const [formContent, setFormContent] = useState('');
  const [formCategory, setFormCategory] = useState('');
  const [formDescription, setFormDescription] = useState('');
  const [query, setQuery] = useState('');

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const data = await templateApi.list();
      setTemplates(data);
    } catch (err) {
      const message = (err as Error).message;
      setLoadError(message);
      toast.error(`Failed to load templates: ${message}`);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open) fetchTemplates();
  }, [open, fetchTemplates]);

  const startCreate = () => {
    setCreating(true);
    setEditing(null);
    setFormName('');
    setFormContent('');
    setFormCategory('');
    setFormDescription('');
  };

  const startEdit = (t: PromptTemplate) => {
    setEditing(t);
    setCreating(false);
    setFormName(t.name);
    setFormContent(t.template_body);
    setFormCategory(t.category || '');
    setFormDescription(t.description || '');
  };

  const handleSave = async () => {
    if (!formName.trim() || !formContent.trim()) return;
    try {
      if (editing) {
        await templateApi.update(editing.id, {
          name: formName,
          template_body: formContent,
          category: formCategory || undefined,
          description: formDescription || undefined,
        });
        toast.success('Template updated');
      } else {
        await templateApi.create({
          name: formName,
          template_body: formContent,
          category: formCategory || undefined,
          description: formDescription || undefined,
        });
        toast.success('Template created');
      }
      setEditing(null);
      setCreating(false);
      fetchTemplates();
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await templateApi.delete(id);
      toast.success('Template deleted');
      fetchTemplates();
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  if (!open) return null;

  const showForm = creating || editing;
  const filteredTemplates = templates.filter((template) => {
    const value = query.trim().toLocaleLowerCase();
    if (!value) return true;
    return [template.name, template.category, template.description, template.template_body]
      .filter(Boolean)
      .some((field) => field!.toLocaleLowerCase().includes(value));
  });

  return (
    <DialogShell
      open={open}
      onClose={onClose}
      title="Prompt Templates"
      icon={<FileText size={18} />}
      maxWidth="max-w-2xl"
      maxHeight="max-h-[80vh]"
      bodyClassName="px-4 py-4 sm:px-6"
      actions={!showForm && (
        <motion.button
          whileHover={{ scale: 1.03 }}
          whileTap={{ scale: 0.97 }}
          onClick={startCreate}
          className="min-h-10 inline-flex items-center gap-1.5 px-3 rounded-xl btn-primary text-xs font-medium"
        >
          <Plus size={14} /> New
        </motion.button>
      )}
    >
            {!showForm && (
              <label className="mb-4 flex min-h-11 items-center gap-2 rounded-xl border border-border bg-surface-alt px-3">
                <Search size={14} className="text-text-muted" />
                <span className="sr-only">Search templates</span>
                <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search name, category, description, or content" className="min-w-0 flex-1 bg-transparent text-sm text-text outline-none placeholder:text-text-muted" />
              </label>
            )}
            {showForm ? (
              <div className="space-y-3">
                <div>
                  <label className="block text-xs font-medium text-text-muted mb-1">Name</label>
                  <input
                    type="text"
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                               focus:outline-none focus:border-primary/50"
                    placeholder="Template name"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-text-muted mb-1">Description</label>
                  <input
                    type="text"
                    value={formDescription}
                    onChange={(e) => setFormDescription(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                               focus:outline-none focus:border-primary/50"
                    placeholder="Optional description"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-text-muted mb-1">Category</label>
                  <input
                    type="text"
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                               focus:outline-none focus:border-primary/50"
                    placeholder="e.g., coding, writing, analysis"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-text-muted mb-1">
                    Content <span className="text-text-muted/60">(use {'{{variable}}'} for placeholders)</span>
                  </label>
                  <textarea
                    value={formContent}
                    onChange={(e) => setFormContent(e.target.value)}
                    rows={8}
                    className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                               focus:outline-none focus:border-primary/50 resize-y font-mono"
                    placeholder="Write your prompt template here..."
                  />
                </div>
                <div className="flex justify-end gap-2">
                  <button
                    onClick={() => { setEditing(null); setCreating(false); }}
                    className="min-h-10 px-4 rounded-lg text-sm text-text-muted hover:text-text transition-colors"
                  >
                    Cancel
                  </button>
                  <motion.button
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    onClick={handleSave}
                    disabled={!formName.trim() || !formContent.trim()}
                    className="min-h-10 flex items-center gap-1.5 px-4 rounded-lg btn-primary text-sm font-medium disabled:opacity-50"
                  >
                    <Save size={14} /> {editing ? 'Update' : 'Create'}
                  </motion.button>
                </div>
              </div>
            ) : loading ? (
              <div className="py-12 text-center text-text-muted">Loading...</div>
            ) : loadError ? (
              <div className="py-12 text-center">
                <p className="text-sm text-danger">Failed to load templates</p>
                <p className="text-xs text-text-muted mt-1 break-words">{loadError}</p>
                <button
                  onClick={fetchTemplates}
                  className="mt-4 min-h-10 px-4 rounded-xl glass text-sm text-text hover:bg-surface-hover transition-colors"
                >
                  Retry
                </button>
              </div>
            ) : filteredTemplates.length === 0 ? (
              <div className="py-12 text-center text-text-muted text-sm">
                No templates yet. Create one to get started.
              </div>
            ) : (
              <div className="space-y-2">
                {filteredTemplates.map((t) => (
                  <div key={t.id} className="glass rounded-xl p-3 group">
                    <div className="flex items-start justify-between gap-3">
                      <div className="flex-1 min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-sm font-medium text-text break-words">{t.name}</span>
                          {t.category && (
                            <span className="text-xs px-1.5 py-0.5 rounded-full bg-primary/10 text-primary">
                              {t.category}
                            </span>
                          )}
                          {t.is_system && (
                            <span className="text-xs px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400">
                              Built-in
                            </span>
                          )}
                        </div>
                        {t.description && (
                          <p className="text-xs text-text-muted mt-0.5">{t.description}</p>
                        )}
                        <p className="text-xs text-text-muted/60 mt-1 line-clamp-2 font-mono">{t.template_body}</p>
                      </div>
                      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity ml-2">
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => { navigator.clipboard.writeText(t.template_body); toast.success('Copied'); }}
                          className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg hover:bg-surface-light text-text-muted hover:text-text transition-colors"
                          aria-label={`Copy ${t.name}`}
                          title="Copy"
                        >
                          <Copy size={14} />
                        </motion.button>
                        <motion.button
                          whileHover={{ scale: 1.1 }}
                          whileTap={{ scale: 0.9 }}
                          onClick={() => startEdit(t)}
                          className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg hover:bg-surface-light text-text-muted hover:text-text transition-colors"
                          aria-label={`Edit ${t.name}`}
                          title="Edit"
                        >
                          <Edit3 size={14} />
                        </motion.button>
                        {!t.is_system && (
                          <motion.button
                            whileHover={{ scale: 1.1 }}
                            whileTap={{ scale: 0.9 }}
                            onClick={() => handleDelete(t.id)}
                            className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg hover:bg-surface-light text-text-muted hover:text-red-400 transition-colors"
                            aria-label={`Delete ${t.name}`}
                            title="Delete"
                          >
                            <Trash2 size={14} />
                          </motion.button>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
    </DialogShell>
  );
}

// Quick template picker inline (for use in ChatView input area)
interface TemplatePickerProps {
  onSelect: (content: string) => void;
}

export function TemplatePicker({ onSelect }: TemplatePickerProps) {
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    templateApi.list().then(setTemplates).catch(() => {});
  }, []);

  if (templates.length === 0) return null;

  return (
    <div className="relative">
      <motion.button
        whileHover={{ scale: 1.05 }}
        whileTap={{ scale: 0.95 }}
        onClick={() => setOpen(!open)}
        className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-light transition-colors"
        title="Insert template"
      >
        <FileText size={16} />
      </motion.button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 8 }}
            className="absolute bottom-full mb-2 left-0 w-64 glass-strong rounded-xl shadow-xl overflow-hidden z-50"
          >
            <div className="px-3 py-2 border-b border-border text-xs font-medium text-text-muted">Templates</div>
            <div className="max-h-48 overflow-y-auto">
              {templates.map((t) => (
                <button
                  key={t.id}
                  onClick={() => { onSelect(t.template_body); setOpen(false); }}
                  className="w-full text-left px-3 py-2 hover:bg-surface-light/50 transition-colors flex items-center justify-between"
                >
                  <div className="min-w-0">
                    <div className="text-sm text-text truncate">{t.name}</div>
                    {t.description && <div className="text-xs text-text-muted truncate">{t.description}</div>}
                  </div>
                  <ChevronRight size={12} className="text-text-muted shrink-0" />
                </button>
              ))}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
