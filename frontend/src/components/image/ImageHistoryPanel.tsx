import { useMemo } from 'react';
import { Sparkles, Pencil, Shuffle, Clock, GitBranch } from 'lucide-react';
import { clsx } from 'clsx';
import type { ImageNode } from '../../types';

interface ImageHistoryPanelProps {
  conversationId: string;
  nodes: ImageNode[];
  activeNodeId: string | null;
  onNodeSelect: (nodeId: string) => void;
  onBranchFrom?: (nodeId: string) => void;
}

interface TreeNode {
  node: ImageNode;
  children: TreeNode[];
  depth: number;
}

const OperationIcon = ({ type }: { type: string }) => {
  switch (type) {
    case 'generate':
      return <Sparkles size={12} className="text-primary" />;
    case 'edit':
      return <Pencil size={12} className="text-accent" />;
    case 'variation':
      return <Shuffle size={12} className="text-amber-400" />;
    default:
      return <Sparkles size={12} className="text-text-muted" />;
  }
};

const OperationBadge = ({ type }: { type: string }) => {
  const colors: Record<string, string> = {
    generate: 'bg-primary/15 text-primary border-primary/20',
    edit: 'bg-accent/15 text-accent border-accent/20',
    variation: 'bg-amber-500/15 text-amber-400 border-amber-500/20',
  };
  return (
    <span className={clsx('text-[9px] px-1.5 py-0.5 rounded-md border font-medium', colors[type] || 'bg-surface-hover text-text-muted border-border')}>
      {type}
    </span>
  );
};

function formatTimeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = Math.max(0, now - then);
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function buildTree(nodes: ImageNode[]): TreeNode[] {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const childrenMap = new Map<string, ImageNode[]>();
  const roots: ImageNode[] = [];

  for (const node of nodes) {
    if (!node.parent_node_id || !byId.has(node.parent_node_id)) {
      roots.push(node);
    } else {
      const siblings = childrenMap.get(node.parent_node_id) || [];
      siblings.push(node);
      childrenMap.set(node.parent_node_id, siblings);
    }
  }

  const sortByCreated = (a: ImageNode, b: ImageNode) =>
    new Date(a.created_at).getTime() - new Date(b.created_at).getTime();

  roots.sort(sortByCreated);

  function buildSubtree(node: ImageNode, depth: number): TreeNode {
    const children = (childrenMap.get(node.id) || [])
      .sort(sortByCreated)
      .map((c) => buildSubtree(c, depth + 1));
    return { node, children, depth };
  }

  return roots.map((r) => buildSubtree(r, 0));
}

function getActivePath(nodes: ImageNode[], activeNodeId: string | null): Set<string> {
  if (!activeNodeId) return new Set();
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const path = new Set<string>();
  let current = activeNodeId;
  while (current) {
    path.add(current);
    const node = byId.get(current);
    current = node?.parent_node_id || '';
    if (!byId.has(current)) break;
  }
  return path;
}

function flattenTree(tree: TreeNode[]): TreeNode[] {
  const result: TreeNode[] = [];
  function walk(nodes: TreeNode[]) {
    for (const tn of nodes) {
      result.push(tn);
      walk(tn.children);
    }
  }
  walk(tree);
  return result;
}

export function ImageHistoryPanel({ nodes, activeNodeId, onNodeSelect, onBranchFrom }: ImageHistoryPanelProps) {
  const tree = useMemo(() => buildTree(nodes), [nodes]);
  const flatNodes = useMemo(() => flattenTree(tree), [tree]);
  const activePath = useMemo(() => getActivePath(nodes, activeNodeId), [nodes, activeNodeId]);
  const hasBranches = nodes.some((n) => n.parent_node_id);

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 py-3 border-b border-border">
        <h3 className="text-xs font-medium text-text-muted uppercase tracking-wide">History</h3>
        {nodes.length > 0 && (
          <p className="text-[10px] text-text-muted/60 mt-0.5">
            {nodes.length} node{nodes.length !== 1 ? 's' : ''}
            {hasBranches && ' · tree view'}
          </p>
        )}
      </div>

      <div className="flex-1 overflow-y-auto">
        {flatNodes.length === 0 ? (
          <div className="p-4 text-center">
            <Clock size={24} className="mx-auto text-text-muted/30 mb-2" />
            <p className="text-xs text-text-muted/50">No history yet</p>
            <p className="text-[10px] text-text-muted/40 mt-1">Generate an image to see it here</p>
          </div>
        ) : (
          <div className="p-2 space-y-0.5">
            {flatNodes.map(({ node, depth, children }) => {
              const isActive = node.id === activeNodeId;
              const isOnPath = activePath.has(node.id);
              const isBranchPoint = children.length > 1;

              return (
                <div key={node.id} style={{ paddingLeft: depth * 16 }}>
                  {/* Tree connector line */}
                  {depth > 0 && (
                    <div className="flex items-center gap-1 -mb-1 ml-2">
                      <div className={clsx(
                        'w-3 border-l border-b rounded-bl-md h-3',
                        isOnPath ? 'border-primary/40' : 'border-border'
                      )} />
                    </div>
                  )}
                  <button
                    onClick={() => onNodeSelect(node.id)}
                    className={clsx(
                      'w-full text-left px-3 py-2 rounded-xl transition-all group relative',
                      isActive
                        ? 'bg-primary/10 border border-primary/20'
                        : isOnPath
                          ? 'bg-primary/5 border border-primary/10 hover:bg-primary/10'
                          : 'hover:bg-surface-hover border border-transparent'
                    )}
                  >
                    <div className="flex items-start gap-2">
                      <div className={clsx(
                        'mt-0.5 p-1.5 rounded-lg shrink-0',
                        isActive ? 'bg-primary/20' : 'bg-surface-alt'
                      )}>
                        <OperationIcon type={node.operation_type} />
                      </div>

                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-1.5 mb-0.5">
                          <OperationBadge type={node.operation_type} />
                          {isBranchPoint && (
                            <span className="text-[8px] px-1 py-px rounded bg-amber-500/10 text-amber-400 border border-amber-500/15">
                              {children.length} branches
                            </span>
                          )}
                        </div>

                        <p className="text-xs text-text leading-relaxed line-clamp-2">
                          {node.instruction || '(no instruction)'}
                        </p>

                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-[10px] text-text-muted/60">
                            {node.model || node.provider}
                          </span>
                          {node.seed != null && (
                            <span className="text-[10px] text-text-muted/40 font-mono">
                              seed:{node.seed}
                            </span>
                          )}
                          <span className="text-[10px] text-text-muted/40 ml-auto">
                            {formatTimeAgo(node.created_at)}
                          </span>
                        </div>
                      </div>

                      {/* Branch from here button */}
                      {onBranchFrom && !isActive && (
                        <button
                          onClick={(e) => { e.stopPropagation(); onBranchFrom(node.id); }}
                          className="opacity-0 group-hover:opacity-100 p-1 rounded-md
                                     text-text-muted hover:text-primary hover:bg-primary/10
                                     transition-all shrink-0 mt-0.5"
                          title="Branch from here"
                        >
                          <GitBranch size={12} />
                        </button>
                      )}
                    </div>
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
