import { useState, useEffect } from 'react';
import { authApi, setAuthToken } from '../api';
import { motion } from 'framer-motion';
import { toast } from 'sonner';
import { LogIn, UserPlus, MessageSquare } from 'lucide-react';

interface LoginScreenProps {
  onAuthenticated: () => void;
}

export function LoginScreen({ onAuthenticated }: LoginScreenProps) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [loading, setLoading] = useState(false);

  // Check if auth is needed on mount
  useEffect(() => {
    authApi.status().then((s) => {
      if (!s.auth_enabled) {
        // Solo mode — no users registered, skip login
        onAuthenticated();
      } else if (!s.has_users) {
        // Auth enabled but no users — show register
        setMode('register');
      }
    }).catch(() => {
      // If status check fails, show login anyway
    });
  }, [onAuthenticated]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password.trim()) return;

    setLoading(true);
    try {
      if (mode === 'register') {
        const res = await authApi.register({ username, password, display_name: displayName || undefined });
        setAuthToken(res.token);
        toast.success('Account created');
      } else {
        const res = await authApi.login({ username, password });
        setAuthToken(res.token);
        toast.success('Logged in');
      }
      onAuthenticated();
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex items-center justify-center h-full bg-background">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-sm mx-4"
      >
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary/20 to-accent/20 mb-4">
            <MessageSquare size={32} className="text-primary" />
          </div>
          <h1 className="text-2xl font-bold gradient-text">OmniLLM-Studio</h1>
          <p className="text-text-muted text-sm mt-1">Local-first AI chat</p>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 p-1 glass rounded-xl mb-6">
          <button
            onClick={() => setMode('login')}
            className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
              mode === 'login' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
            }`}
          >
            Sign In
          </button>
          <button
            onClick={() => setMode('register')}
            className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
              mode === 'register' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
            }`}
          >
            Register
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="glass rounded-2xl p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-text-muted mb-1.5">Username</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                         focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20"
              placeholder="Enter username"
              autoFocus
            />
          </div>

          {mode === 'register' && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }}>
              <label className="block text-xs font-medium text-text-muted mb-1.5">Display Name</label>
              <input
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                           focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20"
                placeholder="Optional display name"
              />
            </motion.div>
          )}

          <div>
            <label className="block text-xs font-medium text-text-muted mb-1.5">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2 rounded-lg bg-surface-light border border-border text-text text-sm
                         focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20"
              placeholder="Enter password"
            />
          </div>

          <motion.button
            whileHover={{ scale: 1.01 }}
            whileTap={{ scale: 0.99 }}
            type="submit"
            disabled={loading || !username.trim() || !password.trim()}
            className="w-full py-2.5 rounded-xl btn-primary text-sm font-medium flex items-center justify-center gap-2
                       disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? (
              <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
            ) : mode === 'login' ? (
              <>
                <LogIn size={16} /> Sign In
              </>
            ) : (
              <>
                <UserPlus size={16} /> Create Account
              </>
            )}
          </motion.button>
        </form>
      </motion.div>
    </div>
  );
}
