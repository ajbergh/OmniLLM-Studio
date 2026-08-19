import { useState } from 'react';
import { Download, Check, AlertCircle, Sparkles, Terminal, FileCode } from 'lucide-react';
import { parseMCPInput, type ParsedMCPServerConfig } from '../mcpImportParser';

interface MCPImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onImport: (configs: ParsedMCPServerConfig[]) => void;
}

export function MCPImportModal({ isOpen, onClose, onImport }: MCPImportModalProps) {
  const [rawText, setRawText] = useState('');
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const parsed = parseMCPInput(rawText);

  const handleImport = () => {
    if (!rawText.trim()) {
      setError('Please paste a command or JSON configuration.');
      return;
    }

    if (parsed.length === 0) {
      setError('Could not recognize any valid MCP server configuration or CLI command.');
      return;
    }

    setError(null);
    onImport(parsed);
    setRawText('');
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fadeIn">
      <div className="w-full max-w-2xl rounded-2xl border border-border bg-surface shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-border bg-surface-alt">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-cyan-500/20 to-primary/20 flex items-center justify-center shadow-md shadow-cyan-500/10">
              <Download size={18} className="text-cyan-300" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-text">Import MCP Server</h3>
              <p className="text-[11px] text-text-muted">
                Paste an <code className="text-primary font-mono text-[10px]">npx add-mcp</code> command, CLI command, or JSON config
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="text-text-muted hover:text-text p-1.5 rounded-lg hover:bg-surface-hover transition-colors"
          >
            ✕
          </button>
        </div>

        {/* Body */}
        <div className="p-5 overflow-y-auto space-y-4 flex-1">
          <div>
            <label className="block text-xs font-medium text-text mb-1">
              Command or JSON Configuration
            </label>
            <textarea
              value={rawText}
              onChange={(e) => {
                setRawText(e.target.value);
                if (error) setError(null);
              }}
              rows={7}
              placeholder={`Examples supported:\n\n1) npx command:\nnpx add-mcp@latest "https://mcp.neon.tech/mcp?projectId=xxxx" \\\n  --name Neon \\\n  --header "Authorization: Bearer napi_xxxxxxxxxxxx"\n\n2) JSON config:\n{\n  "mcpServers": {\n    "Neon": {\n      "type": "http",\n      "url": "https://mcp.neon.tech/mcp?projectId=xxxxx",\n      "headers": { "Authorization": "Bearer napi_xxx" }\n    }\n  }\n}`}
              className="w-full px-3 py-2 text-xs bg-surface-alt border border-border rounded-xl text-text font-mono focus:outline-none focus:border-primary transition-all input-glow resize-y"
            />
          </div>

          {error && (
            <div className="flex items-center gap-2 p-3 rounded-xl border border-danger/30 bg-danger-soft text-danger text-xs">
              <AlertCircle size={14} className="shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {parsed.length > 0 && (
            <div className="p-4 rounded-xl border border-primary/30 bg-primary/5 space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-text flex items-center gap-1.5">
                  <Sparkles size={14} className="text-primary" />
                  Detected {parsed.length} MCP Server{parsed.length > 1 ? 's' : ''}:
                </span>
              </div>
              <div className="space-y-2">
                {parsed.map((server, idx) => (
                  <div
                    key={idx}
                    className="p-3 rounded-lg bg-surface border border-border text-xs flex flex-col gap-1.5"
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-text">{server.name}</span>
                      <span
                        className={`text-[10px] px-2 py-0.5 rounded-full border ${
                          server.transport === 'http'
                            ? 'border-cyan-500/30 bg-cyan-500/10 text-cyan-300'
                            : 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300'
                        }`}
                      >
                        {server.transport}
                      </span>
                    </div>
                    {server.transport === 'http' && server.url && (
                      <div className="text-[11px] text-text-muted truncate font-mono">
                        URL: {server.url}
                      </div>
                    )}
                    {server.transport === 'stdio' && server.command && (
                      <div className="text-[11px] text-text-muted font-mono truncate">
                        Command: {server.command} {(server.args || []).join(' ')}
                      </div>
                    )}
                    {server.headers && Object.keys(server.headers).length > 0 && (
                      <div className="text-[10px] text-text-muted">
                        Headers: {Object.keys(server.headers).join(', ')}
                      </div>
                    )}
                    {server.env && Object.keys(server.env).length > 0 && (
                      <div className="text-[10px] text-text-muted">
                        Env: {Object.keys(server.env).join(', ')}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2 text-[11px] text-text-muted">
            <div className="p-3 rounded-xl border border-border bg-surface flex items-start gap-2">
              <Terminal size={14} className="text-cyan-400 shrink-0 mt-0.5" />
              <div>
                <strong className="text-text block mb-0.5">CLI / npx support</strong>
                <span>Supports <code className="text-primary">npx add-mcp</code>, <code className="text-primary">claude mcp add</code>, and stdio commands with headers & env flags.</span>
              </div>
            </div>
            <div className="p-3 rounded-xl border border-border bg-surface flex items-start gap-2">
              <FileCode size={14} className="text-purple-400 shrink-0 mt-0.5" />
              <div>
                <strong className="text-text block mb-0.5">JSON Configs</strong>
                <span>Supports standard <code className="text-primary">mcpServers</code> objects from Claude Desktop, Cursor, and VS Code configs.</span>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 p-4 border-t border-border bg-surface-alt">
          <button
            type="button"
            onClick={onClose}
            className="min-h-10 px-4 text-xs rounded-xl border border-border hover:bg-surface-hover text-text transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleImport}
            disabled={!rawText.trim() || parsed.length === 0}
            className="btn-primary min-h-10 inline-flex items-center justify-center gap-1.5 px-4 text-xs rounded-xl font-medium disabled:opacity-50"
          >
            <Check size={14} />
            {parsed.length > 1 ? `Import ${parsed.length} Servers` : 'Import Server'}
          </button>
        </div>
      </div>
    </div>
  );
}
