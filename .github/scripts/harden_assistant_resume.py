from pathlib import Path


def rep(path: str, old: str, new: str, count: int = 1):
    p = Path(path)
    source = p.read_text()
    found = source.count(old)
    if found < count:
        raise SystemExit(f"{path}: expected {count} of {old!r}, found {found}")
    p.write_text(source.replace(old, new, count))
    print(path, "replaced", count, old[:80])


rep(
    "backend/internal/models/models.go",
    '\tResultSummary  string     `json:"result_summary"`\n',
    '\tResultSummary      string     `json:"result_summary"`\n'
    '\tAssistantProfileID string     `json:"assistant_profile_id,omitempty"`\n',
)
rep(
    "backend/internal/db/db.go",
    "CREATE INDEX IF NOT EXISTS idx_assistant_profiles_workspace ON assistant_profiles(workspace_id, name);\n",
    "CREATE INDEX IF NOT EXISTS idx_assistant_profiles_workspace ON assistant_profiles(workspace_id, name);\n"
    "ALTER TABLE agent_runs ADD COLUMN assistant_profile_id TEXT NOT NULL DEFAULT '';\n",
)

p = Path("backend/internal/repository/agent_run.go")
s = p.read_text()
s = s.replace(
    "func (r *AgentRunRepo) Create(conversationID, goal string) (*models.AgentRun, error) {",
    "func (r *AgentRunRepo) Create(conversationID, goal string, assistantProfileIDs ...string) (*models.AgentRun, error) {",
    1,
)
s = s.replace(
    "\tid := uuid.New().String()\n\tnow := time.Now().UTC()\n",
    "\tid := uuid.New().String()\n\tnow := time.Now().UTC()\n"
    "\tassistantProfileID := \"\"\n"
    "\tif len(assistantProfileIDs) > 0 {\n\t\tassistantProfileID = assistantProfileIDs[0]\n\t}\n",
    1,
)
s = s.replace(
    "INSERT INTO agent_runs (id, conversation_id, status, goal, plan_json, result_summary, created_at, updated_at)\n"
    "\t\tVALUES (?, ?, 'planning', ?, '[]', '', ?, ?)",
    "INSERT INTO agent_runs (id, conversation_id, status, goal, plan_json, result_summary, assistant_profile_id, created_at, updated_at)\n"
    "\t\tVALUES (?, ?, 'planning', ?, '[]', '', ?, ?, ?)",
    1,
)
s = s.replace("`, id, conversationID, goal, now, now)", "`, id, conversationID, goal, assistantProfileID, now, now)", 1)
s = s.replace(
    "SELECT id, conversation_id, status, goal, plan_json, result_summary, created_at, updated_at, completed_at",
    "SELECT id, conversation_id, status, goal, plan_json, result_summary, assistant_profile_id, created_at, updated_at, completed_at",
)
s = s.replace(
    "&run.ResultSummary, &run.CreatedAt, &run.UpdatedAt, &run.CompletedAt,",
    "&run.ResultSummary, &run.AssistantProfileID, &run.CreatedAt, &run.UpdatedAt, &run.CompletedAt,",
)
p.write_text(s)

rep(
    "backend/internal/agent/runner.go",
    "type RunOptions struct {\n\tProfile RunProfile\n\tBudgets *RunBudgets\n}",
    "type RunOptions struct {\n"
    "\tProfile            RunProfile\n"
    "\tBudgets            *RunBudgets\n"
    "\tAssistantProfileID string\n"
    "}",
)
rep(
    "backend/internal/agent/runner.go",
    "\trun, err := r.runRepo.Create(conversationID, goal)\n",
    "\trun, err := r.runRepo.Create(conversationID, goal, options.AssistantProfileID)\n",
)
rep(
    "backend/internal/api/agent_handler.go",
    "agent.RunOptions{Profile: req.Profile, Budgets: req.Budgets}",
    "agent.RunOptions{Profile: req.Profile, Budgets: req.Budgets, AssistantProfileID: req.AssistantProfileID}",
)

p = Path("backend/internal/api/agent_handler.go")
s = p.read_text()
marker = "\tflusher, onEvent, ok := prepareAgentSSE(w)\n"
first = s.find(marker)
second = s.find(marker, first + 1)
if second < 0:
    raise SystemExit("second prepareAgentSSE marker not found")
resume_block = '''\tif run.AssistantProfileID != "" && h.assistantRepo != nil {
\t\tuserID := auth.ScopeUserIDFromContext(r.Context())
\t\tsavedProfile, profileErr := h.assistantRepo.Get(userID, run.AssistantProfileID)
\t\tif profileErr != nil {
\t\t\trespondInternalError(w, profileErr)
\t\t\treturn
\t\t}
\t\tif savedProfile == nil {
\t\t\trespondError(w, http.StatusConflict, "saved assistant profile is no longer available")
\t\t\treturn
\t\t}
\t\tif req.Provider == "" && savedProfile.Provider != "" {
\t\t\tprovider = savedProfile.Provider
\t\t}
\t\tif req.Model == "" && savedProfile.Model != "" {
\t\t\tmodel = savedProfile.Model
\t\t}
\t\tif strings.TrimSpace(savedProfile.SystemPrompt) != "" {
\t\t\thistory = append([]llm.ChatMessage{{Role: "system", Content: "ASSISTANT PROFILE INSTRUCTIONS:\\n" + savedProfile.SystemPrompt}}, history...)
\t\t}
\t\tif h.skillRepo != nil {
\t\t\tfor _, skillID := range savedProfile.SkillIDs {
\t\t\t\tskill, skillErr := h.skillRepo.Get(userID, skillID)
\t\t\t\tif skillErr == nil && skill != nil && skill.Enabled {
\t\t\t\t\thistory = append([]llm.ChatMessage{{Role: "system", Content: "ATTACHED SKILL " + skill.Name + ":\\n" + skill.BodyMarkdown}}, history...)
\t\t\t\t}
\t\t\t}
\t\t}
\t\tr = r.WithContext(agent.ContextWithAllowedTools(r.Context(), savedProfile.ToolNames))
\t}
'''
s = s[:second] + resume_block + s[second:]
p.write_text(s)
