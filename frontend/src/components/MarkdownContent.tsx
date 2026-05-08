import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Check, Copy, FileText, Table2, FileJson, FileCode2, Globe } from 'lucide-react';
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

function artifactStyle(ext: string): { icon: React.ReactNode; colorClass: string } {
  switch (ext) {
    case 'xlsx':
    case 'csv':
      return { icon: <Table2 size={14} />, colorClass: 'bg-green-600 hover:bg-green-700' };
    case 'pdf':
      return { icon: <FileText size={14} />, colorClass: 'bg-red-600 hover:bg-red-700' };
    case 'html':
      return { icon: <Globe size={14} />, colorClass: 'bg-orange-500 hover:bg-orange-600' };
    case 'json':
      return { icon: <FileJson size={14} />, colorClass: 'bg-yellow-600 hover:bg-yellow-700' };
    case 'yaml':
    case 'yml':
      return { icon: <FileCode2 size={14} />, colorClass: 'bg-purple-600 hover:bg-purple-700' };
    case 'md':
      return { icon: <FileText size={14} />, colorClass: 'bg-slate-600 hover:bg-slate-700' };
    default:
      return { icon: <FileText size={14} />, colorClass: 'bg-indigo-600 hover:bg-indigo-700' };
  }
}

function safeImageSrc(src?: string): string | undefined {
  const raw = src?.trim();
  if (!raw) return undefined;

  if (raw.startsWith('/v1/')) {
    return resolveApiUrl(raw);
  }

  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== 'https:') return undefined;
    return parsed.toString();
  } catch {
    return undefined;
  }
}

function isSportsLogo(src?: string, alt?: string): boolean {
  const text = `${src ?? ''} ${alt ?? ''}`.toLowerCase();
  return text.includes('logo') && text.includes('espncdn.com');
}

function isSportsNewsImage(src?: string, alt?: string): boolean {
  const text = `${src ?? ''} ${alt ?? ''}`.toLowerCase();
  return text.includes('sports news image') && text.includes('espncdn.com');
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
            const resolvedSrc = safeImageSrc(src);
            if (!resolvedSrc) {
              return alt ? <span>{alt}</span> : null;
            }
            if (isSportsLogo(resolvedSrc, alt)) {
              return (
                <img
                  src={resolvedSrc}
                  alt={alt || ''}
                  className="sports-inline-logo"
                  loading="lazy"
                  {...props}
                />
              );
            }
            if (isSportsNewsImage(resolvedSrc, alt)) {
              return (
                <a href={resolvedSrc} target="_blank" rel="noopener noreferrer" className="sports-news-image-link">
                  <img
                    src={resolvedSrc}
                    alt={alt || 'ESPN news image'}
                    className="sports-news-image"
                    loading="lazy"
                    {...props}
                  />
                </a>
              );
            }
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
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          table({ children, ref: _ref, ...props }) {
            return (
              <div className="message-table-wrap" role="region" aria-label="Scrollable table" tabIndex={0}>
                <table {...props}>{children}</table>
              </div>
            );
          },
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          a({ href, children, ref: _ref, ...props }) {
            const resolvedHref = href ? resolveApiUrl(href) : href;
            const isAttachmentDownload =
              href?.startsWith('/v1/attachments/') && href.endsWith('/download');
            const childText = String(children ?? '');
            const extMatch = childText.match(/\.(\w+)$/);
            const ext = extMatch ? extMatch[1].toLowerCase() : '';

            const artifactExts = ['docx', 'xlsx', 'pptx', 'pdf', 'csv', 'md', 'html', 'json', 'yaml', 'yml'];
            const isArtifactFile = isAttachmentDownload && artifactExts.includes(ext);

            if (isArtifactFile) {
              const { icon, colorClass } = artifactStyle(ext);
              return (
                <a
                  href={resolvedHref}
                  download
                  className={`inline-flex items-center gap-1.5 px-3 py-1.5 mt-2 rounded-lg
                             text-white text-sm font-medium transition-colors no-underline ${colorClass}`}
                  {...props}
                >
                  {icon}
                  {children}
                </a>
              );
            }
            return (
              <a href={resolvedHref} target="_blank" rel="noopener noreferrer" {...props}>
                {children}
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
