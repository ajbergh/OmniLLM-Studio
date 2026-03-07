import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Check, Copy } from 'lucide-react';
import { resolveApiUrl } from '../api';

interface Props {
  content: string;
}

function CodeBlock({ className, children }: { className?: string; children: React.ReactNode }) {
  const [copied, setCopied] = useState(false);
  const text = String(children).replace(/\n$/, '');
  const language = className?.replace('language-', '') || '';

  const handleCopy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative group">
      {language && (
        <div className="absolute top-0 left-0 px-3 py-1 text-[10px] font-mono uppercase tracking-wider
                        text-text-muted bg-surface-alt rounded-tl-[0.65rem] rounded-br-md border-r border-b border-border">
          {language}
        </div>
      )}
      <button
        onClick={handleCopy}
        className="absolute top-2 right-2 p-1.5 rounded-lg bg-surface-hover/80 text-text-muted
                   hover:text-text hover:bg-surface-hover transition-all opacity-0 group-hover:opacity-100"
        aria-label="Copy code"
      >
        {copied ? <Check size={13} className="text-success" /> : <Copy size={13} />}
      </button>
      <pre className={language ? 'pt-8' : ''}>
        <code className={className}>{children}</code>
      </pre>
    </div>
  );
}

export function MarkdownContent({ content }: Props) {
  return (
    <div className="message-content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          code({ className, children, ref: _ref, ...props }) {
            const isInline = !className && typeof children === 'string' && !children.includes('\n');
            if (isInline) {
              return <code className={className} {...props}>{children}</code>;
            }
            return <CodeBlock className={className}>{children}</CodeBlock>;
          },
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          img({ src, alt, ref: _ref, ...props }) {
            // In the Wails desktop build, /v1/ URLs must be rewritten to
            // point at the real local HTTP server instead of wails.localhost.
            const resolvedSrc = src ? resolveApiUrl(src) : src;
            return (
              <a href={resolvedSrc} target="_blank" rel="noopener noreferrer" className="block my-2">
                <img
                  src={resolvedSrc}
                  alt={alt || 'Generated image'}
                  className="rounded-xl max-w-full max-h-[512px] object-contain border border-border shadow-sm
                             hover:shadow-md transition-shadow cursor-pointer"
                  loading="lazy"
                  {...props}
                />
              </a>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
