// Types matching the backend data model

export type ConversationKind = 'chat' | 'image';

export interface Conversation {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
  archived: boolean;
  pinned: boolean;
  default_provider?: string;
  default_model?: string;
  system_prompt?: string;
  kind: ConversationKind;
  metadata_json?: string;
  workspace_id?: string;
  user_id?: string;
}

export interface Message {
  id: string;
  conversation_id: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  created_at: string;
  provider?: string;
  model?: string;
  token_input?: number;
  token_output?: number;
  latency_ms?: number;
  metadata_json?: string;
  branch_id?: string;
  parent_message_id?: string;
  user_id?: string;
}

export interface ProviderProfile {
  id: string;
  name: string;
  type: string;
  base_url?: string;
  default_model?: string;
  default_image_model?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  metadata_json?: string;
  image_capable?: boolean;
}

export interface Attachment {
  id: string;
  conversation_id: string;
  message_id?: string;
  type: 'image' | 'file';
  mime_type: string;
  storage_path: string;
  bytes: number;
  width?: number;
  height?: number;
  created_at: string;
  metadata_json?: string;
}

export interface CreateConversationRequest {
  title?: string;
  default_provider?: string;
  default_model?: string;
  system_prompt?: string;
  kind?: ConversationKind;
}

export interface UpdateConversationRequest {
  title?: string;
  pinned?: boolean;
  archived?: boolean;
  default_provider?: string;
  default_model?: string;
  system_prompt?: string;
}

export interface SendMessageRequest {
  content: string;
  attachment_ids?: string[];
  web_search?: boolean;
  think?: boolean;
  override?: {
    provider?: string;
    model?: string;
    system_prompt?: string;
  };
}

export interface CreateProviderRequest {
  name: string;
  type: string;
  base_url?: string;
  default_model?: string;
  default_image_model?: string;
  api_key?: string;
}

export interface UpdateProviderRequest {
  name?: string;
  type?: string;
  base_url?: string;
  default_model?: string;
  default_image_model?: string;
  enabled?: boolean;
  api_key?: string;
}

export interface SSEEvent {
  event: string;
  data: Record<string, unknown>;
}

// ---- Web Search Types ----

export interface WebSearchResult {
  index: number;
  title: string;
  url: string;
  source: string;
  publishedAt: string;
  snippet: string;
}

export interface WebSearchResponse {
  query: string;
  timeRange: string;
  results: WebSearchResult[];
  fetchedAt: string;
}

export interface WebSearchRequest {
  query: string;
  timeRange?: string;
  region?: string;
  locale?: string;
  maxResults?: number;
}

export interface ToolCall {
  name: 'web_search';
  arguments: {
    query: string;
    timeRange: string;
    region: string;
    locale: string;
    maxResults: number;
  };
}

export interface MessageMetadata {
  web_search?: boolean;
  tool?: string;
  sources?: WebSearchResult[];
  tool_call?: ToolCall;
  rag_sources?: RAGSourceRef[];
  thinking?: string;
}

// Typed application settings (mirrors backend AppSettings).
export interface AppSettings {
  web_search_provider: string;
  brave_api_key: string;
  jina_reader_enabled: boolean;
  jina_reader_max_len?: number;
  rag_enabled: boolean;
  rag_embedding_model: string;
  rag_chunk_size: number;
  rag_chunk_overlap: number;
  rag_top_k: number;
}

// ---- Image Generation Types ----

export interface ImageGenerateRequest {
  prompt: string;
  size?: string;
  quality?: string;
  n?: number;
  reference_image_id?: string;
  override?: {
    provider?: string;
    model?: string;
  };
}

export interface ImageGenerateResponse {
  user_message: Message;
  assistant_message: Message;
  attachments: Attachment[];
}

// ---- Feature Flags ----

export interface FeatureFlag {
  key: string;
  enabled: boolean;
  metadata?: string;
}

// ---- RAG Types ----

export interface DocumentChunk {
  id: string;
  attachment_id: string;
  conversation_id: string;
  chunk_index: number;
  content: string;
  char_offset: number;
  char_length: number;
  token_count: number;
  metadata_json?: string;
  created_at: string;
}

export interface RAGSourceRef {
  chunk_id: string;
  attachment_id: string;
  chunk_index: number;
  score: number;
  preview: string;
}

export interface ReindexResponse {
  conversation_id: string;
  chunks_created: number;
  embeddings_stored: number;
}

export interface IndexAttachmentResponse {
  attachment_id: string;
  chunks_created: number;
  embeddings_stored: number;
}

// ---- Tool Framework Types ----

export interface ToolDefinition {
  name: string;
  description: string;
  parameters: Record<string, unknown>; // JSON Schema
  category: string;
  enabled: boolean;
  policy: string; // "allow" | "deny" | "ask"
}

export interface ToolResult {
  tool_call_id: string;
  content: string;
  is_error: boolean;
  metadata?: Record<string, unknown>;
}

export interface ToolPermission {
  tool_name: string;
  policy: string;
  updated_at?: string;
}

// ---- Analytics / Usage Types ----

export interface PricingRule {
  id: string;
  provider_type: string;
  model_pattern: string;
  input_cost_per_mtok: number;
  output_cost_per_mtok: number;
  currency: string;
  effective_from?: string;
  created_at: string;
}

export interface ProviderUsage {
  provider: string;
  input_tokens: number;
  output_tokens: number;
  message_count: number;
  avg_latency_ms: number;
  estimated_cost: number;
}

export interface ModelUsage {
  provider: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  message_count: number;
  avg_latency_ms: number;
  estimated_cost: number;
}

export interface UsageSummary {
  period: string;
  total_input_tokens: number;
  total_output_tokens: number;
  total_messages: number;
  avg_latency_ms: number;
  estimated_cost: number;
  by_provider: ProviderUsage[];
  by_model: ModelUsage[];
}

// --- Import/Export Types ---

export interface ExportRequest {
  include_attachments?: boolean;
  conversation_ids?: string[];
}

export interface ImportResult {
  conversations_imported: number;
  conversations_skipped: number;
  messages_imported: number;
  attachments_imported: number;
  providers_imported: number;
  providers_skipped: number;
  settings_imported: number;
  warnings: string[];
}

export interface BundleManifest {
  format_version: number;
  app_version: string;
  schema_version: number;
  created_at: string;
  stats: {
    conversations: number;
    messages: number;
    attachments: number;
    providers: number;
  };
}

export interface ValidationReport {
  manifest: BundleManifest;
  valid: boolean;
  warnings?: string[];
  errors?: string[];
}

// Phase 5: Prompt Templates

export interface TemplateVariable {
  name: string;
  label: string;
  type: 'text' | 'select';
  default?: string;
  required?: boolean;
  options?: string[];
}

export interface PromptTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  template_body: string;
  variables: TemplateVariable[];
  is_system: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface InterpolateResult {
  text: string;
  missing_required?: string[];
}

// Phase 6: Agent Mode

// --- Canonical status/type constants (mirrors backend agent package) ---

export const AgentRunStatus = {
  Planning: 'planning',
  Running: 'running',
  AwaitingApproval: 'awaiting_approval',
  Paused: 'paused',
  Completed: 'completed',
  Failed: 'failed',
  Cancelled: 'cancelled',
} as const;
export type AgentRunStatusType = (typeof AgentRunStatus)[keyof typeof AgentRunStatus];

export const AgentStepStatus = {
  Pending: 'pending',
  Running: 'running',
  AwaitingApproval: 'awaiting_approval',
  Completed: 'completed',
  Failed: 'failed',
  Skipped: 'skipped',
} as const;
export type AgentStepStatusType = (typeof AgentStepStatus)[keyof typeof AgentStepStatus];

export const AgentStepType = {
  Think: 'think',
  ToolCall: 'tool_call',
  Approval: 'approval',
  Message: 'message',
} as const;
export type AgentStepTypeValue = (typeof AgentStepType)[keyof typeof AgentStepType];

export const AgentEventType = {
  Plan: 'agent_plan',
  StepStart: 'agent_step_start',
  StepComplete: 'agent_step_complete',
  ApprovalRequired: 'agent_approval_required',
  Token: 'agent_token',
  Complete: 'agent_complete',
  Error: 'agent_error',
} as const;
export type AgentEventTypeValue = (typeof AgentEventType)[keyof typeof AgentEventType];

// --- Data interfaces ---

export interface AgentPlanStep {
  type: AgentStepTypeValue;
  description: string;
  tool_name?: string;
  input_json?: string;
}

export interface AgentStep {
  id: string;
  run_id: string;
  step_index: number;
  type: AgentStepTypeValue;
  description: string;
  status: AgentStepStatusType;
  input_json?: string;
  output_json?: string;
  tool_name?: string;
  message_id?: string;
  duration_ms?: number;
  created_at: string;
  completed_at?: string;
}

export interface AgentRun {
  id: string;
  conversation_id: string;
  status: AgentRunStatusType;
  goal: string;
  plan_json?: string;
  result_summary?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

export interface AgentRunWithSteps extends AgentRun {
  steps: AgentStep[];
}

export interface StartAgentRunRequest {
  goal: string;
  provider?: string;
  model?: string;
}

export interface AgentEvent {
  type: AgentEventTypeValue | string;
  run_id: string;
  step_id?: string;
  data?: unknown;
}

// Phase 7: Conversation Branching

export interface Branch {
  id: string;
  conversation_id: string;
  name: string;
  parent_branch: string;
  fork_message_id: string;
  created_at: string;
}

export interface CreateBranchRequest {
  name?: string;
  fork_message_id: string;
}

// ── Search ────────────────────────────────────────────────────────────────

export type SearchMode = 'hybrid' | 'keyword' | 'semantic';

export interface SearchResult {
  type: 'message' | 'chunk';
  conversation_id: string;
  message_id?: string;
  chunk_id?: string;
  content: string;
  score: number;
  role?: string;
  timestamp?: string;
}

export interface SearchResponse {
  results: SearchResult[];
  count: number;
  query: string;
  mode: SearchMode;
}

export interface ReindexStatus {
  total: number;
  embedded: number;
  status: 'running' | 'completed' | 'error';
}

// ── Workspaces ─────────────────────────────────────────────────────────────────────

export interface Workspace {
  id: string;
  name: string;
  description: string;
  color: string;
  icon: string;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface CreateWorkspaceRequest {
  name: string;
  description?: string;
  color?: string;
  icon?: string;
}

export interface UpdateWorkspaceRequest {
  name?: string;
  description?: string;
  color?: string;
  icon?: string;
  sort_order?: number;
}

export interface WorkspaceStats {
  conversation_count: number;
  message_count: number;
  template_count: number;
}

// Phase 10: Local Collaboration

export interface User {
  id: string;
  username: string;
  display_name: string;
  role: 'admin' | 'member' | 'viewer';
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  token: string;
  expires_at: string;
  user: User;
}

export interface AuthStatus {
  auth_enabled: boolean;
  has_users: boolean;
}

export interface RegisterRequest {
  username: string;
  display_name?: string;
  password: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface WorkspaceMember {
  workspace_id: string;
  user_id: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  joined_at: string;
  username?: string;
  display_name?: string;
}

export interface AddMemberRequest {
  user_id: string;
  role?: string;
}

export interface UpdateMemberRoleRequest {
  role: string;
}

// ── Phase 11: Plugin SDK ──

export interface PluginManifest {
  name: string;
  version: string;
  description?: string;
  author?: string;
  capabilities: string[];
  tools?: PluginToolDef[];
  runtime: 'executable' | 'wasm';
  entrypoint: string;
  permissions?: string[];
}

export interface PluginToolDef {
  name: string;
  description: string;
  parameters?: Record<string, unknown>;
}

export interface InstalledPlugin {
  name: string;
  version: string;
  manifest: PluginManifest;
  enabled: boolean;
  installed_at: string;
  running?: boolean;
}

export interface InstallPluginRequest {
  directory: string;
}

export interface UpdatePluginRequest {
  enabled: boolean;
}

// ── Phase 12: Evaluation Harness ──

export interface EvalSuite {
  name: string;
  version?: string;
  cases: EvalCase[];
}

export interface EvalCase {
  id: string;
  input: string;
  expected_keywords?: string[];
  expected_tool_calls?: string[];
  scoring?: Record<string, number>;
}

export interface EvalCaseResult {
  case_id: string;
  input: string;
  response: string;
  score: number;
  keyword_hits?: string[];
  keyword_misses?: string[];
  tool_calls_matched?: string[];
  breakdown?: Record<string, number>;
}

export interface EvalRun {
  id: string;
  suite_name: string;
  provider: string;
  model: string;
  total_score?: number;
  results_json: string;
  created_at: string;
}

export interface RunEvalRequest {
  provider: string;
  model: string;
  suite: EvalSuite;
}

// ── Image Edit Mode ──

export interface ImageSession {
  id: string;
  conversation_id: string;
  title: string;
  active_node_id?: string;
  created_at: string;
  updated_at: string;
}

export interface ImageNode {
  id: string;
  session_id: string;
  parent_node_id?: string;
  operation_type: 'generate' | 'edit' | 'variation';
  instruction: string;
  provider: string;
  model: string;
  seed?: number;
  params_json?: string;
  created_at: string;
}

export interface ImageNodeAsset {
  id: string;
  node_id: string;
  attachment_id: string;
  variant_index: number;
  is_selected: boolean;
  created_at: string;
}

export interface ImageMask {
  id: string;
  node_id: string;
  attachment_id: string;
  stroke_json?: string;
  created_at: string;
}

export interface ImageReference {
  id: string;
  node_id: string;
  attachment_id: string;
  ref_role: 'content' | 'style';
  sort_order: number;
}

export interface ImageNodeWithMask extends ImageNode {
  mask?: ImageMask;
}

export interface ImageSessionDetail {
  session: ImageSession;
  nodes: ImageNodeWithMask[];
}

export interface ImageEditGenerateRequest {
  prompt: string;
  size?: string;
  quality?: string;
  n?: number;
  seed?: number;
  creativity?: number;
  reference_image_ids?: string[];
  style_reference_ids?: string[];
  override?: {
    provider?: string;
    model?: string;
  };
}

export interface ImageEditEditRequest {
  instruction: string;
  base_image_attachment_id: string;
  mask_attachment_id?: string;
  size?: string;
  strength?: number;
  n?: number;
  reference_image_ids?: string[];
  style_reference_ids?: string[];
  override?: {
    provider?: string;
    model?: string;
  };
}

export interface ImageEditGenerateResponse {
  node: ImageNode;
  assets: ImageNodeAsset[];
}

export interface ModelImageCapabilities {
  supports_editing?: boolean;
  supports_masking?: boolean;
  supports_content_reference?: boolean;
  max_variants?: number;
  supported_sizes?: string[];
}

export interface ImageCapabilities {
  supports_generation: boolean;
  supports_editing: boolean;
  supports_masking: boolean;
  supports_variations: boolean;
  supports_seed: boolean;
  supports_guidance: boolean;
  supports_style_reference: boolean;
  supports_content_reference: boolean;
  max_reference_images: number;
  max_variants: number;
  supported_sizes: string[];
  image_models: string[];
  default_image_model: string;
  model_overrides?: Record<string, ModelImageCapabilities>;
}
