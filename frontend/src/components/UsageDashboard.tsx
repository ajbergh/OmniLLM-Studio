import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { BarChart3, DollarSign, Zap, TrendingUp, X } from 'lucide-react';
import type { UsageSummary, PricingRule } from '../types';

interface UsageDashboardProps {
  open: boolean;
  onClose: () => void;
}

export function UsageDashboard({ open, onClose }: UsageDashboardProps) {
  const [period, setPeriod] = useState('month');
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [pricing, setPricing] = useState<PricingRule[]>([]);
  const [loading, setLoading] = useState(false);
  const [tab, setTab] = useState<'usage' | 'pricing'>('usage');

  // Map display labels to backend period values
  const periodOptions = [
    { label: '7d', value: 'week' },
    { label: '30d', value: 'month' },
    { label: 'All Time', value: 'all' },
  ] as const;

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [u, p] = await Promise.all([
        api.getUsage(period),
        api.listPricing(),
      ]);
      setUsage(u);
      setPricing(p || []);
    } catch (err) {
      toast.error(`Failed to load usage data: ${(err as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [period]);

  useEffect(() => {
    if (open) fetchData();
  }, [open, fetchData]);

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
          className="glass-strong rounded-2xl w-full max-w-3xl max-h-[80vh] overflow-hidden mx-4"
        >
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-border">
            <div className="flex items-center gap-2">
              <BarChart3 size={18} className="text-primary" />
              <h2 className="text-lg font-semibold text-text">Usage & Cost Dashboard</h2>
            </div>
            <motion.button whileHover={{ scale: 1.1 }} whileTap={{ scale: 0.9 }} onClick={onClose}>
              <X size={18} className="text-text-muted hover:text-text" />
            </motion.button>
          </div>

          {/* Tabs */}
          <div className="flex gap-1 p-1 mx-6 mt-4 glass rounded-xl w-fit">
            <button
              onClick={() => setTab('usage')}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                tab === 'usage' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
              }`}
            >
              Usage
            </button>
            <button
              onClick={() => setTab('pricing')}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                tab === 'pricing' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
              }`}
            >
              Pricing Rules
            </button>
          </div>

          {/* Content */}
          <div className="px-6 py-4 overflow-y-auto max-h-[60vh]">
            {loading ? (
              <div className="py-12 text-center text-text-muted">Loading...</div>
            ) : tab === 'usage' ? (
              <div className="space-y-4">
                {/* Period selector */}
                <div className="flex gap-2">
                  {periodOptions.map((opt) => (
                    <button
                      key={opt.value}
                      onClick={() => setPeriod(opt.value)}
                      className={`px-3 py-1 rounded-lg text-xs font-medium transition-colors ${
                        period === opt.value ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text glass'
                      }`}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>

                {usage && (
                  <>
                    {/* Summary cards */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                      <StatCard icon={Zap} label="Total Messages" value={usage.total_messages.toLocaleString()} />
                      <StatCard icon={TrendingUp} label="Input Tokens" value={formatTokens(usage.total_input_tokens)} />
                      <StatCard icon={TrendingUp} label="Output Tokens" value={formatTokens(usage.total_output_tokens)} />
                      <StatCard icon={DollarSign} label="Est. Cost" value={`$${(usage.estimated_cost || 0).toFixed(4)}`} />
                    </div>

                    {/* Per-provider breakdown */}
                    {usage.by_provider && usage.by_provider.length > 0 && (
                      <div>
                        <h3 className="text-sm font-medium text-text mb-2">By Provider</h3>
                        <div className="space-y-2">
                          {usage.by_provider.map((bp) => (
                            <div key={bp.provider} className="glass rounded-lg p-3">
                              <div className="flex items-center justify-between mb-1">
                                <span className="text-sm font-medium text-text">{bp.provider}</span>
                                <span className="text-xs text-text-muted">{bp.message_count} requests</span>
                              </div>
                              <div className="flex gap-4 text-xs text-text-muted">
                                <span>In: {formatTokens(bp.input_tokens)}</span>
                                <span>Out: {formatTokens(bp.output_tokens)}</span>
                                {bp.estimated_cost > 0 && <span className="text-emerald-400">${bp.estimated_cost.toFixed(4)}</span>}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Per-model breakdown */}
                    {usage.by_model && usage.by_model.length > 0 && (
                      <div>
                        <h3 className="text-sm font-medium text-text mb-2">By Model</h3>
                        <div className="space-y-1.5">
                          {usage.by_model.map((bm) => (
                            <div key={`${bm.provider}-${bm.model}`} className="flex items-center justify-between py-1.5 px-3 glass rounded-lg text-xs">
                              <div>
                                <span className="text-text font-medium">{bm.model}</span>
                                <span className="text-text-muted ml-2">({bm.provider})</span>
                              </div>
                              <div className="flex gap-3 text-text-muted">
                                <span>{bm.message_count} req</span>
                                <span>{formatTokens(bm.input_tokens + bm.output_tokens)}</span>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </>
                )}
              </div>
            ) : (
              <div className="space-y-3">
                {pricing.length === 0 ? (
                  <div className="py-8 text-center text-text-muted text-sm">No pricing rules configured</div>
                ) : (
                  pricing.map((rule) => (
                    <div key={rule.id} className="glass rounded-lg p-3 flex items-center justify-between">
                      <div>
                        <div className="text-sm font-medium text-text">{rule.model_pattern}</div>
                        <div className="text-xs text-text-muted mt-0.5">{rule.provider_type || 'Any provider'}</div>
                      </div>
                      <div className="text-right">
                        <div className="text-xs text-text-muted">
                          <span className="text-text">${rule.input_cost_per_mtok}</span>/M in
                        </div>
                        <div className="text-xs text-text-muted">
                          <span className="text-text">${rule.output_cost_per_mtok}</span>/M out
                        </div>
                      </div>
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

function StatCard({ icon: Icon, label, value }: { icon: React.ElementType; label: string; value: string }) {
  return (
    <div className="glass rounded-xl p-3">
      <div className="flex items-center gap-1.5 text-text-muted mb-1">
        <Icon size={12} />
        <span className="text-xs">{label}</span>
      </div>
      <div className="text-lg font-semibold text-text">{value}</div>
    </div>
  );
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toString();
}
