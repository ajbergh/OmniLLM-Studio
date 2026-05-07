import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import { toast } from 'sonner';
import { BarChart3, DollarSign, Zap, TrendingUp } from 'lucide-react';
import { DialogShell } from './DialogShell';
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
  const [loadError, setLoadError] = useState('');
  const [tab, setTab] = useState<'usage' | 'pricing'>('usage');

  // Map display labels to backend period values
  const periodOptions = [
    { label: '7d', value: 'week' },
    { label: '30d', value: 'month' },
    { label: 'All Time', value: 'all' },
  ] as const;

  const fetchData = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const [u, p] = await Promise.all([
        api.getUsage(period),
        api.listPricing(),
      ]);
      setUsage(u);
      setPricing(p || []);
    } catch (err) {
      const message = (err as Error).message;
      setLoadError(message);
      toast.error(`Failed to load usage data: ${message}`);
    } finally {
      setLoading(false);
    }
  }, [period]);

  useEffect(() => {
    if (open) fetchData();
  }, [open, fetchData]);

  if (!open) return null;

  return (
    <DialogShell
      open={open}
      onClose={onClose}
      title="Usage & Cost Dashboard"
      icon={<BarChart3 size={18} />}
      maxWidth="max-w-3xl"
      maxHeight="max-h-[80vh]"
      bodyClassName="px-4 py-4 sm:px-6"
    >
      {/* Tabs */}
      <div className="flex gap-1 p-1 glass rounded-xl w-fit max-w-full overflow-x-auto">
        <button
          onClick={() => setTab('usage')}
          className={`min-h-10 px-4 rounded-lg text-sm font-medium transition-colors ${
            tab === 'usage' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
          }`}
        >
          Usage
        </button>
        <button
          onClick={() => setTab('pricing')}
          className={`min-h-10 px-4 rounded-lg text-sm font-medium transition-colors ${
            tab === 'pricing' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
          }`}
        >
          Pricing Rules
        </button>
      </div>

      <div className="pt-4">
        {loading ? (
          <div className="py-12 text-center text-text-muted">Loading...</div>
        ) : loadError ? (
          <div className="py-10 text-center">
            <p className="text-sm text-danger">Failed to load usage data</p>
            <p className="text-xs text-text-muted mt-1 break-words">{loadError}</p>
            <button
              onClick={fetchData}
              className="mt-4 min-h-10 px-4 rounded-xl glass text-sm text-text hover:bg-surface-hover transition-colors"
            >
              Retry
            </button>
          </div>
        ) : tab === 'usage' ? (
          <div className="space-y-4">
            {/* Period selector */}
            <div className="flex flex-wrap gap-2">
              {periodOptions.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setPeriod(opt.value)}
                  className={`min-h-9 px-3 rounded-lg text-xs font-medium transition-colors ${
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
                          <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between mb-1">
                            <span className="text-sm font-medium text-text break-words">{bp.provider}</span>
                            <span className="text-xs text-text-muted">{bp.message_count} requests</span>
                          </div>
                          <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-text-muted">
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
                        <div key={`${bm.provider}-${bm.model}`} className="flex flex-col gap-1 py-2 px-3 glass rounded-lg text-xs sm:flex-row sm:items-center sm:justify-between">
                          <div className="min-w-0">
                            <span className="text-text font-medium break-words">{bm.model}</span>
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
                <div key={rule.id} className="glass rounded-lg p-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-text break-words">{rule.model_pattern}</div>
                    <div className="text-xs text-text-muted mt-0.5">{rule.provider_type || 'Any provider'}</div>
                  </div>
                  <div className="text-left sm:text-right">
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
    </DialogShell>
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
