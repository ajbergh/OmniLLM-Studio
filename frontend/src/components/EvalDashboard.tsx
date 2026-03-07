import { useState, useEffect, useCallback } from 'react';
import { evalApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { FlaskConical, Play, Trash2, X, ChevronRight, Upload } from 'lucide-react';
import type { EvalRun, EvalSuite, EvalCaseResult } from '../types';

interface EvalDashboardProps {
  open: boolean;
  onClose: () => void;
}

export function EvalDashboard({ open, onClose }: EvalDashboardProps) {
  const [runs, setRuns] = useState<EvalRun[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedRun, setSelectedRun] = useState<EvalRun | null>(null);
  const [results, setResults] = useState<EvalCaseResult[]>([]);
  const [tab, setTab] = useState<'runs' | 'new'>('runs');

  // New run form
  const [suiteJson, setSuiteJson] = useState('');
  const [provider, setProvider] = useState('');
  const [model, setModel] = useState('');
  const [running, setRunning] = useState(false);

  const fetchRuns = useCallback(async () => {
    setLoading(true);
    try {
      const data = await evalApi.listRuns();
      setRuns(data || []);
    } catch (err) {
      toast.error(`Failed to load eval runs: ${(err as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open) fetchRuns();
  }, [open, fetchRuns]);

  const handleRunEval = async () => {
    if (!suiteJson.trim() || !provider.trim()) return;
    setRunning(true);
    try {
      const suite: EvalSuite = JSON.parse(suiteJson);
      const run = await evalApi.run({ provider, model, suite });
      toast.success(`Eval complete: score ${((run.total_score ?? 0) * 100).toFixed(1)}%`);
      setSuiteJson('');
      setProvider('');
      setModel('');
      setTab('runs');
      fetchRuns();
    } catch (err) {
      toast.error(`Eval failed: ${(err as Error).message}`);
    } finally {
      setRunning(false);
    }
  };

  const viewRunDetails = async (run: EvalRun) => {
    setSelectedRun(run);
    try {
      const full = await evalApi.getRun(run.id);
      if (full.results_json) {
        try {
          const parsed = JSON.parse(full.results_json);
          setResults(Array.isArray(parsed) ? parsed : (parsed.results || []));
        } catch {
          setResults([]);
        }
      } else {
        setResults([]);
      }
    } catch {
      setResults([]);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await evalApi.deleteRun(id);
      setRuns((prev) => prev.filter((r) => r.id !== id));
      if (selectedRun?.id === id) setSelectedRun(null);
      toast.success('Eval run deleted');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (ev) => {
      setSuiteJson(ev.target?.result as string || '');
    };
    reader.readAsText(file);
  };

  const scoreColor = (score: number | null | undefined) => {
    const s = score ?? 0;
    if (s >= 0.8) return 'text-emerald-400';
    if (s >= 0.5) return 'text-amber-400';
    return 'text-red-400';
  };

  const scoreBg = (score: number | null | undefined) => {
    const s = score ?? 0;
    if (s >= 0.8) return 'bg-emerald-400';
    if (s >= 0.5) return 'bg-amber-400';
    return 'bg-red-400';
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
          className="glass-strong rounded-2xl w-full max-w-3xl max-h-[85vh] overflow-hidden mx-4"
        >
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-border">
            <div className="flex items-center gap-2">
              <FlaskConical size={18} className="text-primary" />
              <h2 className="text-lg font-semibold text-text">Evaluation Harness</h2>
            </div>
            <motion.button whileHover={{ scale: 1.1 }} whileTap={{ scale: 0.9 }} onClick={onClose}>
              <X size={18} className="text-text-muted hover:text-text" />
            </motion.button>
          </div>

          {/* Tabs */}
          <div className="flex gap-1 p-1 mx-6 mt-4 glass rounded-xl w-fit">
            <button
              onClick={() => { setTab('runs'); setSelectedRun(null); }}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                tab === 'runs' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
              }`}
            >
              Past Runs
            </button>
            <button
              onClick={() => setTab('new')}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                tab === 'new' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
              }`}
            >
              New Eval
            </button>
          </div>

          {/* Content */}
          <div className="px-6 py-4 overflow-y-auto max-h-[65vh]">
            {tab === 'new' ? (
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs font-medium text-text-muted mb-1">Provider</label>
                    <input
                      type="text"
                      value={provider}
                      onChange={(e) => setProvider(e.target.value)}
                      placeholder="e.g., openai"
                      className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                                 focus:outline-none focus:border-primary/50"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-text-muted mb-1">Model (optional)</label>
                    <input
                      type="text"
                      value={model}
                      onChange={(e) => setModel(e.target.value)}
                      placeholder="e.g., gpt-4.1"
                      className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                                 focus:outline-none focus:border-primary/50"
                    />
                  </div>
                </div>

                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="text-xs font-medium text-text-muted">Eval Suite JSON</label>
                    <label className="flex items-center gap-1 text-xs text-primary cursor-pointer hover:text-primary/80">
                      <Upload size={12} /> Upload
                      <input type="file" accept=".json" onChange={handleFileUpload} className="hidden" />
                    </label>
                  </div>
                  <textarea
                    value={suiteJson}
                    onChange={(e) => setSuiteJson(e.target.value)}
                    rows={12}
                    className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-xs
                               focus:outline-none focus:border-primary/50 resize-y font-mono"
                    placeholder='{"name": "my-eval", "cases": [{"id": "1", "input": "...", "expected_keywords": ["..."]}]}'
                  />
                </div>

                <motion.button
                  whileHover={{ scale: 1.01 }}
                  whileTap={{ scale: 0.99 }}
                  onClick={handleRunEval}
                  disabled={running || !suiteJson.trim() || !provider.trim()}
                  className="w-full py-2.5 rounded-xl btn-primary text-sm font-medium flex items-center justify-center gap-2
                             disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {running ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      Running evaluation...
                    </>
                  ) : (
                    <>
                      <Play size={14} /> Run Evaluation
                    </>
                  )}
                </motion.button>
              </div>
            ) : selectedRun ? (
              /* Run detail view */
              <div className="space-y-4">
                <button
                  onClick={() => setSelectedRun(null)}
                  className="text-xs text-text-muted hover:text-text transition-colors"
                >
                  ← Back to runs
                </button>

                <div className="glass rounded-xl p-4">
                  <div className="flex items-center justify-between mb-3">
                    <div>
                      <h3 className="text-sm font-medium text-text">{selectedRun.suite_name}</h3>
                      <div className="text-xs text-text-muted mt-0.5">
                        {selectedRun.provider} / {selectedRun.model || 'default'}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className={`text-2xl font-bold ${scoreColor(selectedRun.total_score || 0)}`}>
                        {((selectedRun.total_score || 0) * 100).toFixed(1)}%
                      </div>
                      <div className="text-xs text-text-muted">
                        {new Date(selectedRun.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  </div>

                  {/* Score bar */}
                  <div className="w-full h-2 rounded-full bg-surface-light overflow-hidden">
                    <div
                      className={`h-full rounded-full ${scoreBg(selectedRun.total_score || 0)} transition-all`}
                      style={{ width: `${(selectedRun.total_score || 0) * 100}%` }}
                    />
                  </div>
                </div>

                {/* Case results */}
                {results.length > 0 && (
                  <div className="space-y-2">
                    <h4 className="text-sm font-medium text-text">Case Results</h4>
                    {results.map((cr, i) => (
                      <div key={cr.case_id || i} className="glass rounded-lg p-3">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-xs font-medium text-text">{cr.case_id}</span>
                          <span className={`text-sm font-semibold ${scoreColor(cr.score)}`}>
                            {(cr.score * 100).toFixed(0)}%
                          </span>
                        </div>
                        <div className="text-xs text-text-muted mb-1">
                          <span className="font-medium">Input:</span> {cr.input}
                        </div>
                        {cr.response && (
                          <div className="text-xs text-text-muted/70 mt-1 line-clamp-3">
                            <span className="font-medium">Response:</span> {cr.response}
                          </div>
                        )}
                        <div className="flex gap-3 mt-2 text-xs">
                          {cr.keyword_hits && cr.keyword_hits.length > 0 && (
                            <span className="text-emerald-400">
                              Hits: {cr.keyword_hits.join(', ')}
                            </span>
                          )}
                          {cr.keyword_misses && cr.keyword_misses.length > 0 && (
                            <span className="text-red-400">
                              Misses: {cr.keyword_misses.join(', ')}
                            </span>
                          )}
                        </div>
                        {cr.breakdown && Object.keys(cr.breakdown).length > 0 && (
                          <div className="flex gap-2 mt-2">
                            {Object.entries(cr.breakdown).map(([key, val]) => (
                              <span key={key} className="text-xs px-1.5 py-0.5 rounded bg-surface-light text-text-muted">
                                {key}: {(val * 100).toFixed(0)}%
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              /* Runs list */
              <div className="space-y-2">
                {loading ? (
                  <div className="py-12 text-center text-text-muted">Loading...</div>
                ) : runs.length === 0 ? (
                  <div className="py-12 text-center text-text-muted text-sm">
                    <FlaskConical size={32} className="mx-auto mb-3 opacity-30" />
                    <p>No eval runs yet</p>
                    <p className="text-xs mt-1">Run an evaluation to test prompt quality across providers</p>
                  </div>
                ) : (
                  runs.map((run) => (
                    <div key={run.id} className="glass rounded-xl p-3 group flex items-center justify-between">
                      <button
                        onClick={() => viewRunDetails(run)}
                        className="flex-1 text-left flex items-center gap-3"
                      >
                        <div className={`text-lg font-bold ${scoreColor(run.total_score || 0)}`}>
                          {((run.total_score || 0) * 100).toFixed(0)}%
                        </div>
                        <div>
                          <div className="text-sm font-medium text-text">{run.suite_name}</div>
                          <div className="text-xs text-text-muted">
                            {run.provider} / {run.model || 'default'} — {new Date(run.created_at).toLocaleDateString()}
                          </div>
                        </div>
                        <ChevronRight size={14} className="ml-auto text-text-muted" />
                      </button>
                      <motion.button
                        whileHover={{ scale: 1.1 }}
                        whileTap={{ scale: 0.9 }}
                        onClick={() => handleDelete(run.id)}
                        className="p-1.5 rounded-lg text-text-muted hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all ml-2"
                      >
                        <Trash2 size={14} />
                      </motion.button>
                    </div>
                  ))
                )}
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
