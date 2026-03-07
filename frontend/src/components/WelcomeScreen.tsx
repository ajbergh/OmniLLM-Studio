import { motion } from 'framer-motion';
import { Zap, Shield, Globe, ArrowRight, Sparkles, Command, Search, Download } from 'lucide-react';
import { AppIcon } from './AppIcon';

interface Props {
  onNewChat: () => void;
  onOpenSettings: () => void;
}

const container = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.1, delayChildren: 0.2 },
  },
};

const item = {
  hidden: { opacity: 0, y: 20 },
  show: { opacity: 1, y: 0, transition: { duration: 0.5, ease: 'easeOut' as const } },
};

const features = [
  {
    icon: Globe,
    title: 'Any Provider',
    desc: 'OpenAI, Anthropic, Gemini, Ollama, and more',
    color: 'from-blue-500/20 to-cyan-500/20',
    iconColor: 'text-blue-400',
  },
  {
    icon: Search,
    title: 'Web Search',
    desc: 'Built-in search with Brave & DuckDuckGo + Jina Reader',
    color: 'from-teal-500/20 to-emerald-500/20',
    iconColor: 'text-teal-400',
  },
  {
    icon: Zap,
    title: 'Real-time Streaming',
    desc: 'Watch responses appear token by token',
    color: 'from-amber-500/20 to-orange-500/20',
    iconColor: 'text-amber-400',
  },
  {
    icon: Shield,
    title: '100% Local',
    desc: 'Your data stays on your machine, always',
    color: 'from-emerald-500/20 to-green-500/20',
    iconColor: 'text-emerald-400',
  },
  {
    icon: Sparkles,
    title: 'Smart Features',
    desc: 'Auto-titles, message editing, regeneration',
    color: 'from-purple-500/20 to-pink-500/20',
    iconColor: 'text-purple-400',
  },
  {
    icon: Download,
    title: 'Export & Share',
    desc: 'Export conversations as Markdown or JSON',
    color: 'from-rose-500/20 to-pink-500/20',
    iconColor: 'text-rose-400',
  },
];

export function WelcomeScreen({ onNewChat, onOpenSettings }: Props) {
  return (
    <div className="flex-1 flex items-center justify-center relative overflow-hidden">
      {/* Ambient background glows */}
      <div className="ambient-glow bg-primary" style={{ top: '10%', left: '20%' }} />
      <div className="ambient-glow bg-accent" style={{ bottom: '10%', right: '20%' }} />

      <motion.div
        variants={container}
        initial="hidden"
        animate="show"
        className="relative z-10 max-w-2xl mx-auto px-6 text-center"
      >
        {/* Logo */}
        <motion.div variants={item} className="mb-8">
          <div className="inline-flex items-center justify-center mb-6 animate-float">
            <AppIcon size={80} />
          </div>
          <h1 className="text-4xl font-bold mb-3">
            <span className="gradient-text">OmniLLM-Studio</span>
          </h1>
          <p className="text-text-muted text-lg font-light">
            Your unified AI conversation hub
          </p>
        </motion.div>

        {/* Feature grid */}
        <motion.div variants={item} className="grid grid-cols-2 sm:grid-cols-3 gap-3 mb-8">
          {features.map((f) => (
            <motion.div
              key={f.title}
              whileHover={{ scale: 1.02, y: -2 }}
              transition={{ duration: 0.2 }}
              className={`p-4 rounded-2xl bg-gradient-to-br ${f.color} border border-border-subtle
                          text-left cursor-default group`}
            >
              <f.icon size={20} className={`${f.iconColor} mb-2`} />
              <h3 className="text-sm font-semibold text-text mb-1">{f.title}</h3>
              <p className="text-xs text-text-muted leading-relaxed">{f.desc}</p>
            </motion.div>
          ))}
        </motion.div>

        {/* Actions */}
        <motion.div variants={item} className="flex items-center justify-center gap-3">
          <motion.button
            whileHover={{ scale: 1.03 }}
            whileTap={{ scale: 0.98 }}
            onClick={onNewChat}
            className="btn-primary px-6 py-3 rounded-2xl text-sm font-medium
                       flex items-center gap-2 shadow-lg shadow-primary/20"
          >
            <Sparkles size={16} />
            Start a conversation
            <ArrowRight size={14} />
          </motion.button>
          <motion.button
            whileHover={{ scale: 1.03 }}
            whileTap={{ scale: 0.98 }}
            onClick={onOpenSettings}
            className="px-6 py-3 rounded-2xl text-sm font-medium border border-border
                       text-text-secondary hover:text-text hover:border-primary/30
                       hover:bg-primary-glow transition-all duration-200"
          >
            Configure Providers
          </motion.button>
        </motion.div>

        {/* Keyboard shortcut hint */}
        <motion.div variants={item} className="mt-8 flex items-center justify-center gap-4 text-text-muted text-xs flex-wrap">
          <span className="flex items-center gap-1.5">
            <kbd className="px-1.5 py-0.5 rounded bg-surface-alt border border-border text-[10px] font-mono">
              <Command size={10} className="inline" />+N
            </kbd>
            New chat
          </span>
          <span className="flex items-center gap-1.5">
            <kbd className="px-1.5 py-0.5 rounded bg-surface-alt border border-border text-[10px] font-mono">
              <Command size={10} className="inline" />+,
            </kbd>
            Settings
          </span>
          <span className="flex items-center gap-1.5">
            <kbd className="px-1.5 py-0.5 rounded bg-surface-alt border border-border text-[10px] font-mono">
              <Command size={10} className="inline" />+K
            </kbd>
            Shortcuts
          </span>
        </motion.div>
      </motion.div>
    </div>
  );
}
