# RAG backend validation diagnostic

- compile_status: 1
- focused_status: 1
- race_status: 1

## Compile all packages
```text
# github.com/ajbergh/omnillm-studio/internal/api
internal/api/rag_handler.go:4:2: "log" imported and not used
FAIL	github.com/ajbergh/omnillm-studio/cmd/desktop [build failed]
?   	github.com/ajbergh/omnillm-studio/cmd/playwrightseed	[no test files]
?   	github.com/ajbergh/omnillm-studio/cmd/playwrightseedchat	[no test files]
FAIL	github.com/ajbergh/omnillm-studio/cmd/server [build failed]
ok  	github.com/ajbergh/omnillm-studio/internal/agent	0.009s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/analytics	[no test files]
FAIL	github.com/ajbergh/omnillm-studio/internal/api [build failed]
?   	github.com/ajbergh/omnillm-studio/internal/apps	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/artifacts	0.006s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/auth	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/browser	0.004s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/bundle	0.009s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/config	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/crypto	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/db	0.005s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/document	0.005s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/eval	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/filelibrary	0.006s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/jobs	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/llm	0.004s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/mcpclient	0.009s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/memory	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/models	0.005s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/music	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/plugins	0.004s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/rag	0.009s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/repository	0.015s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/router	0.012s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/search	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/sports	0.013s [no tests to run]
?   	github.com/ajbergh/omnillm-studio/internal/tasks	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/tasktools	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/templates	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/tools	0.009s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/urlcontext	0.012s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/video	0.004s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/websearch	0.004s [no tests to run]
ok  	github.com/ajbergh/omnillm-studio/internal/wordgen	0.004s [no tests to run]
FAIL
```

## Focused RAG and API tests
```text
# github.com/ajbergh/omnillm-studio/internal/api [github.com/ajbergh/omnillm-studio/internal/api.test]
internal/api/rag_handler.go:4:2: "log" imported and not used
--- FAIL: TestNewLLMEmbeddingFunc_Success (0.00s)
    embed_test.go:44: unexpected embedding: [0.26726124 0.5345225 0.80178374]
--- FAIL: TestNewLLMEmbeddingFunc_RetryOnTransient (0.29s)
    embed_test.go:63: unexpected embedding: [1]
2026/07/19 15:30:07 [db] applying migration V2: feature_flags
2026/07/19 15:30:07 [db] migration V2 applied successfully
2026/07/19 15:30:07 [db] applying migration V3: document_chunks
2026/07/19 15:30:07 [db] migration V3 applied successfully
2026/07/19 15:30:07 [db] applying migration V4: document_embeddings
2026/07/19 15:30:07 [db] migration V4 applied successfully
2026/07/19 15:30:07 [db] applying migration V5: tool_permissions
2026/07/19 15:30:07 [db] migration V5 applied successfully
2026/07/19 15:30:07 [db] applying migration V6: pricing_rules
2026/07/19 15:30:07 [db] migration V6 applied successfully
2026/07/19 15:30:07 [db] applying migration V7: prompt_templates
2026/07/19 15:30:07 [db] migration V7 applied successfully
2026/07/19 15:30:07 [db] applying migration V8: agent_runs
2026/07/19 15:30:07 [db] migration V8 applied successfully
2026/07/19 15:30:07 [db] applying migration V9: agent_steps
2026/07/19 15:30:07 [db] migration V9 applied successfully
2026/07/19 15:30:07 [db] applying migration V10: message_branches
2026/07/19 15:30:07 [db] migration V10 applied successfully
2026/07/19 15:30:07 [db] applying migration V11: conversation_branches
2026/07/19 15:30:07 [db] migration V11 applied successfully
2026/07/19 15:30:07 [db] applying migration V12: message_embeddings
2026/07/19 15:30:07 [db] migration V12 applied successfully
2026/07/19 15:30:07 [db] applying migration V13: workspaces
2026/07/19 15:30:07 [db] migration V13 applied successfully
2026/07/19 15:30:07 [db] applying migration V14: conversations_workspace
2026/07/19 15:30:07 [db] migration V14 applied successfully
2026/07/19 15:30:07 [db] applying migration V15: templates_workspace
2026/07/19 15:30:07 [db] migration V15 applied successfully
2026/07/19 15:30:07 [db] applying migration V16: users
2026/07/19 15:30:07 [db] migration V16 applied successfully
2026/07/19 15:30:07 [db] applying migration V17: sessions
2026/07/19 15:30:07 [db] migration V17 applied successfully
2026/07/19 15:30:07 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 15:30:07 [db] migration V18 applied successfully
2026/07/19 15:30:07 [db] applying migration V19: installed_plugins
2026/07/19 15:30:07 [db] migration V19 applied successfully
2026/07/19 15:30:07 [db] applying migration V20: eval_runs
2026/07/19 15:30:07 [db] migration V20 applied successfully
2026/07/19 15:30:07 [db] applying migration V21: performance_indexes
2026/07/19 15:30:07 [db] migration V21 applied successfully
2026/07/19 15:30:07 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 15:30:07 [db] migration V22 applied successfully
2026/07/19 15:30:07 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 15:30:07 [db] migration V23 applied successfully
2026/07/19 15:30:07 [db] applying migration V24: image_node_assets_and_references
2026/07/19 15:30:07 [db] migration V24 applied successfully
2026/07/19 15:30:07 [db] applying migration V25: provider_default_image_model
2026/07/19 15:30:07 [db] migration V25 applied successfully
2026/07/19 15:30:07 [db] applying migration V26: conversation_kind
2026/07/19 15:30:07 [db] migration V26 applied successfully
2026/07/19 15:30:07 [db] applying migration V27: word_doc_generation_flag
2026/07/19 15:30:07 [db] migration V27 applied successfully
2026/07/19 15:30:07 [db] applying migration V28: sports_lookup_flag
2026/07/19 15:30:07 [db] migration V28 applied successfully
2026/07/19 15:30:07 [db] applying migration V29: news_lookup_flag
2026/07/19 15:30:07 [db] migration V29 applied successfully
2026/07/19 15:30:07 [db] applying migration V30: mcp_servers
2026/07/19 15:30:07 [db] migration V30 applied successfully
2026/07/19 15:30:07 [db] applying migration V31: mcp_audit_log
2026/07/19 15:30:07 [db] migration V31 applied successfully
2026/07/19 15:30:07 [db] applying migration V32: mcp_servers_headers
2026/07/19 15:30:07 [db] migration V32 applied successfully
2026/07/19 15:30:07 [db] applying migration V33: file_library_foundation
2026/07/19 15:30:07 [db] migration V33 applied successfully
2026/07/19 15:30:07 [db] applying migration V34: workspace_project_context
2026/07/19 15:30:07 [db] migration V34 applied successfully
2026/07/19 15:30:07 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 15:30:07 [db] migration V35 applied successfully
2026/07/19 15:30:07 [db] applying migration V36: music_studio
2026/07/19 15:30:07 [db] migration V36 applied successfully
2026/07/19 15:30:07 [db] applying migration V37: video_studio_foundation
2026/07/19 15:30:07 [db] migration V37 applied successfully
2026/07/19 15:30:07 [db] applying migration V38: video_studio_timelines
2026/07/19 15:30:07 [db] migration V38 applied successfully
2026/07/19 15:30:07 [db] applying migration V39: video_studio_render_jobs
2026/07/19 15:30:07 [db] migration V39 applied successfully
2026/07/19 15:30:07 [db] applying migration V40: video_generation_input_assets
2026/07/19 15:30:07 [db] migration V40 applied successfully
2026/07/19 15:30:07 [db] applying migration V41: video_render_job_metadata
2026/07/19 15:30:07 [db] migration V41 applied successfully
2026/07/19 15:30:07 [db] applying migration V42: agent_runtime
2026/07/19 15:30:07 [db] migration V42 applied successfully
2026/07/19 15:30:07 [rag] lazy-migrated 2 legacy embeddings into chromem for conversation e19fa4ce-3cc6-4ef4-b562-3aaf8bf894da
2026/07/19 15:30:07 [db] applying migration V2: feature_flags
2026/07/19 15:30:07 [db] migration V2 applied successfully
2026/07/19 15:30:07 [db] applying migration V3: document_chunks
2026/07/19 15:30:07 [db] migration V3 applied successfully
2026/07/19 15:30:07 [db] applying migration V4: document_embeddings
2026/07/19 15:30:07 [db] migration V4 applied successfully
2026/07/19 15:30:07 [db] applying migration V5: tool_permissions
2026/07/19 15:30:07 [db] migration V5 applied successfully
2026/07/19 15:30:07 [db] applying migration V6: pricing_rules
2026/07/19 15:30:07 [db] migration V6 applied successfully
2026/07/19 15:30:07 [db] applying migration V7: prompt_templates
2026/07/19 15:30:07 [db] migration V7 applied successfully
2026/07/19 15:30:07 [db] applying migration V8: agent_runs
2026/07/19 15:30:07 [db] migration V8 applied successfully
2026/07/19 15:30:07 [db] applying migration V9: agent_steps
2026/07/19 15:30:07 [db] migration V9 applied successfully
2026/07/19 15:30:07 [db] applying migration V10: message_branches
2026/07/19 15:30:07 [db] migration V10 applied successfully
2026/07/19 15:30:07 [db] applying migration V11: conversation_branches
2026/07/19 15:30:07 [db] migration V11 applied successfully
2026/07/19 15:30:07 [db] applying migration V12: message_embeddings
2026/07/19 15:30:07 [db] migration V12 applied successfully
2026/07/19 15:30:07 [db] applying migration V13: workspaces
2026/07/19 15:30:07 [db] migration V13 applied successfully
2026/07/19 15:30:07 [db] applying migration V14: conversations_workspace
2026/07/19 15:30:07 [db] migration V14 applied successfully
2026/07/19 15:30:07 [db] applying migration V15: templates_workspace
2026/07/19 15:30:07 [db] migration V15 applied successfully
2026/07/19 15:30:07 [db] applying migration V16: users
2026/07/19 15:30:07 [db] migration V16 applied successfully
2026/07/19 15:30:07 [db] applying migration V17: sessions
2026/07/19 15:30:07 [db] migration V17 applied successfully
2026/07/19 15:30:07 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 15:30:07 [db] migration V18 applied successfully
2026/07/19 15:30:07 [db] applying migration V19: installed_plugins
2026/07/19 15:30:07 [db] migration V19 applied successfully
2026/07/19 15:30:07 [db] applying migration V20: eval_runs
2026/07/19 15:30:07 [db] migration V20 applied successfully
2026/07/19 15:30:07 [db] applying migration V21: performance_indexes
2026/07/19 15:30:07 [db] migration V21 applied successfully
2026/07/19 15:30:07 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 15:30:07 [db] migration V22 applied successfully
2026/07/19 15:30:07 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 15:30:07 [db] migration V23 applied successfully
2026/07/19 15:30:07 [db] applying migration V24: image_node_assets_and_references
2026/07/19 15:30:07 [db] migration V24 applied successfully
2026/07/19 15:30:07 [db] applying migration V25: provider_default_image_model
2026/07/19 15:30:07 [db] migration V25 applied successfully
2026/07/19 15:30:07 [db] applying migration V26: conversation_kind
2026/07/19 15:30:07 [db] migration V26 applied successfully
2026/07/19 15:30:07 [db] applying migration V27: word_doc_generation_flag
2026/07/19 15:30:07 [db] migration V27 applied successfully
2026/07/19 15:30:07 [db] applying migration V28: sports_lookup_flag
2026/07/19 15:30:07 [db] migration V28 applied successfully
2026/07/19 15:30:07 [db] applying migration V29: news_lookup_flag
2026/07/19 15:30:07 [db] migration V29 applied successfully
2026/07/19 15:30:07 [db] applying migration V30: mcp_servers
2026/07/19 15:30:07 [db] migration V30 applied successfully
2026/07/19 15:30:07 [db] applying migration V31: mcp_audit_log
2026/07/19 15:30:07 [db] migration V31 applied successfully
2026/07/19 15:30:07 [db] applying migration V32: mcp_servers_headers
2026/07/19 15:30:07 [db] migration V32 applied successfully
2026/07/19 15:30:07 [db] applying migration V33: file_library_foundation
2026/07/19 15:30:07 [db] migration V33 applied successfully
2026/07/19 15:30:07 [db] applying migration V34: workspace_project_context
2026/07/19 15:30:07 [db] migration V34 applied successfully
2026/07/19 15:30:07 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 15:30:07 [db] migration V35 applied successfully
2026/07/19 15:30:07 [db] applying migration V36: music_studio
2026/07/19 15:30:07 [db] migration V36 applied successfully
2026/07/19 15:30:07 [db] applying migration V37: video_studio_foundation
2026/07/19 15:30:07 [db] migration V37 applied successfully
2026/07/19 15:30:07 [db] applying migration V38: video_studio_timelines
2026/07/19 15:30:07 [db] migration V38 applied successfully
2026/07/19 15:30:07 [db] applying migration V39: video_studio_render_jobs
2026/07/19 15:30:07 [db] migration V39 applied successfully
2026/07/19 15:30:07 [db] applying migration V40: video_generation_input_assets
2026/07/19 15:30:07 [db] migration V40 applied successfully
2026/07/19 15:30:07 [db] applying migration V41: video_render_job_metadata
2026/07/19 15:30:07 [db] migration V41 applied successfully
2026/07/19 15:30:07 [db] applying migration V42: agent_runtime
2026/07/19 15:30:07 [db] migration V42 applied successfully
FAIL
FAIL	github.com/ajbergh/omnillm-studio/internal/rag	0.427s
ok  	github.com/ajbergh/omnillm-studio/internal/repository	0.570s
ok  	github.com/ajbergh/omnillm-studio/internal/filelibrary	0.007s
ok  	github.com/ajbergh/omnillm-studio/internal/document	0.006s
FAIL	github.com/ajbergh/omnillm-studio/internal/api [build failed]
FAIL
```

## Focused race tests
```text
--- FAIL: TestNewLLMEmbeddingFunc_Success (0.00s)
    embed_test.go:44: unexpected embedding: [0.26726124 0.5345225 0.80178374]
--- FAIL: TestNewLLMEmbeddingFunc_RetryOnTransient (0.29s)
    embed_test.go:63: unexpected embedding: [1]
2026/07/19 15:30:11 [db] applying migration V2: feature_flags
2026/07/19 15:30:11 [db] migration V2 applied successfully
2026/07/19 15:30:11 [db] applying migration V3: document_chunks
2026/07/19 15:30:11 [db] migration V3 applied successfully
2026/07/19 15:30:11 [db] applying migration V4: document_embeddings
2026/07/19 15:30:11 [db] migration V4 applied successfully
2026/07/19 15:30:11 [db] applying migration V5: tool_permissions
2026/07/19 15:30:11 [db] migration V5 applied successfully
2026/07/19 15:30:11 [db] applying migration V6: pricing_rules
2026/07/19 15:30:11 [db] migration V6 applied successfully
2026/07/19 15:30:11 [db] applying migration V7: prompt_templates
2026/07/19 15:30:11 [db] migration V7 applied successfully
2026/07/19 15:30:11 [db] applying migration V8: agent_runs
2026/07/19 15:30:11 [db] migration V8 applied successfully
2026/07/19 15:30:11 [db] applying migration V9: agent_steps
2026/07/19 15:30:11 [db] migration V9 applied successfully
2026/07/19 15:30:11 [db] applying migration V10: message_branches
2026/07/19 15:30:11 [db] migration V10 applied successfully
2026/07/19 15:30:11 [db] applying migration V11: conversation_branches
2026/07/19 15:30:11 [db] migration V11 applied successfully
2026/07/19 15:30:11 [db] applying migration V12: message_embeddings
2026/07/19 15:30:12 [db] migration V12 applied successfully
2026/07/19 15:30:12 [db] applying migration V13: workspaces
2026/07/19 15:30:12 [db] migration V13 applied successfully
2026/07/19 15:30:12 [db] applying migration V14: conversations_workspace
2026/07/19 15:30:12 [db] migration V14 applied successfully
2026/07/19 15:30:12 [db] applying migration V15: templates_workspace
2026/07/19 15:30:12 [db] migration V15 applied successfully
2026/07/19 15:30:12 [db] applying migration V16: users
2026/07/19 15:30:12 [db] migration V16 applied successfully
2026/07/19 15:30:12 [db] applying migration V17: sessions
2026/07/19 15:30:12 [db] migration V17 applied successfully
2026/07/19 15:30:12 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 15:30:12 [db] migration V18 applied successfully
2026/07/19 15:30:12 [db] applying migration V19: installed_plugins
2026/07/19 15:30:12 [db] migration V19 applied successfully
2026/07/19 15:30:12 [db] applying migration V20: eval_runs
2026/07/19 15:30:12 [db] migration V20 applied successfully
2026/07/19 15:30:12 [db] applying migration V21: performance_indexes
2026/07/19 15:30:12 [db] migration V21 applied successfully
2026/07/19 15:30:12 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 15:30:12 [db] migration V22 applied successfully
2026/07/19 15:30:12 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 15:30:12 [db] migration V23 applied successfully
2026/07/19 15:30:12 [db] applying migration V24: image_node_assets_and_references
2026/07/19 15:30:12 [db] migration V24 applied successfully
2026/07/19 15:30:12 [db] applying migration V25: provider_default_image_model
2026/07/19 15:30:12 [db] migration V25 applied successfully
2026/07/19 15:30:12 [db] applying migration V26: conversation_kind
2026/07/19 15:30:12 [db] migration V26 applied successfully
2026/07/19 15:30:12 [db] applying migration V27: word_doc_generation_flag
2026/07/19 15:30:12 [db] migration V27 applied successfully
2026/07/19 15:30:12 [db] applying migration V28: sports_lookup_flag
2026/07/19 15:30:12 [db] migration V28 applied successfully
2026/07/19 15:30:12 [db] applying migration V29: news_lookup_flag
2026/07/19 15:30:12 [db] migration V29 applied successfully
2026/07/19 15:30:12 [db] applying migration V30: mcp_servers
2026/07/19 15:30:12 [db] migration V30 applied successfully
2026/07/19 15:30:12 [db] applying migration V31: mcp_audit_log
2026/07/19 15:30:12 [db] migration V31 applied successfully
2026/07/19 15:30:12 [db] applying migration V32: mcp_servers_headers
2026/07/19 15:30:12 [db] migration V32 applied successfully
2026/07/19 15:30:12 [db] applying migration V33: file_library_foundation
2026/07/19 15:30:12 [db] migration V33 applied successfully
2026/07/19 15:30:12 [db] applying migration V34: workspace_project_context
2026/07/19 15:30:12 [db] migration V34 applied successfully
2026/07/19 15:30:12 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 15:30:12 [db] migration V35 applied successfully
2026/07/19 15:30:12 [db] applying migration V36: music_studio
2026/07/19 15:30:12 [db] migration V36 applied successfully
2026/07/19 15:30:12 [db] applying migration V37: video_studio_foundation
2026/07/19 15:30:12 [db] migration V37 applied successfully
2026/07/19 15:30:12 [db] applying migration V38: video_studio_timelines
2026/07/19 15:30:12 [db] migration V38 applied successfully
2026/07/19 15:30:12 [db] applying migration V39: video_studio_render_jobs
2026/07/19 15:30:12 [db] migration V39 applied successfully
2026/07/19 15:30:12 [db] applying migration V40: video_generation_input_assets
2026/07/19 15:30:12 [db] migration V40 applied successfully
2026/07/19 15:30:12 [db] applying migration V41: video_render_job_metadata
2026/07/19 15:30:12 [db] migration V41 applied successfully
2026/07/19 15:30:12 [db] applying migration V42: agent_runtime
2026/07/19 15:30:12 [db] migration V42 applied successfully
2026/07/19 15:30:12 [rag] lazy-migrated 2 legacy embeddings into chromem for conversation 54c1ae10-e9a3-463a-b235-4aadab8cd3b2
2026/07/19 15:30:12 [db] applying migration V2: feature_flags
2026/07/19 15:30:12 [db] migration V2 applied successfully
2026/07/19 15:30:12 [db] applying migration V3: document_chunks
2026/07/19 15:30:12 [db] migration V3 applied successfully
2026/07/19 15:30:12 [db] applying migration V4: document_embeddings
2026/07/19 15:30:12 [db] migration V4 applied successfully
2026/07/19 15:30:12 [db] applying migration V5: tool_permissions
2026/07/19 15:30:12 [db] migration V5 applied successfully
2026/07/19 15:30:12 [db] applying migration V6: pricing_rules
2026/07/19 15:30:12 [db] migration V6 applied successfully
2026/07/19 15:30:12 [db] applying migration V7: prompt_templates
2026/07/19 15:30:12 [db] migration V7 applied successfully
2026/07/19 15:30:12 [db] applying migration V8: agent_runs
2026/07/19 15:30:12 [db] migration V8 applied successfully
2026/07/19 15:30:12 [db] applying migration V9: agent_steps
2026/07/19 15:30:12 [db] migration V9 applied successfully
2026/07/19 15:30:12 [db] applying migration V10: message_branches
2026/07/19 15:30:12 [db] migration V10 applied successfully
2026/07/19 15:30:12 [db] applying migration V11: conversation_branches
2026/07/19 15:30:12 [db] migration V11 applied successfully
2026/07/19 15:30:12 [db] applying migration V12: message_embeddings
2026/07/19 15:30:12 [db] migration V12 applied successfully
2026/07/19 15:30:12 [db] applying migration V13: workspaces
2026/07/19 15:30:12 [db] migration V13 applied successfully
2026/07/19 15:30:12 [db] applying migration V14: conversations_workspace
2026/07/19 15:30:13 [db] migration V14 applied successfully
2026/07/19 15:30:13 [db] applying migration V15: templates_workspace
2026/07/19 15:30:13 [db] migration V15 applied successfully
2026/07/19 15:30:13 [db] applying migration V16: users
2026/07/19 15:30:13 [db] migration V16 applied successfully
2026/07/19 15:30:13 [db] applying migration V17: sessions
2026/07/19 15:30:13 [db] migration V17 applied successfully
2026/07/19 15:30:13 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 15:30:13 [db] migration V18 applied successfully
2026/07/19 15:30:13 [db] applying migration V19: installed_plugins
2026/07/19 15:30:13 [db] migration V19 applied successfully
2026/07/19 15:30:13 [db] applying migration V20: eval_runs
2026/07/19 15:30:13 [db] migration V20 applied successfully
2026/07/19 15:30:13 [db] applying migration V21: performance_indexes
2026/07/19 15:30:13 [db] migration V21 applied successfully
2026/07/19 15:30:13 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 15:30:13 [db] migration V22 applied successfully
2026/07/19 15:30:13 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 15:30:13 [db] migration V23 applied successfully
2026/07/19 15:30:13 [db] applying migration V24: image_node_assets_and_references
2026/07/19 15:30:13 [db] migration V24 applied successfully
2026/07/19 15:30:13 [db] applying migration V25: provider_default_image_model
2026/07/19 15:30:13 [db] migration V25 applied successfully
2026/07/19 15:30:13 [db] applying migration V26: conversation_kind
2026/07/19 15:30:13 [db] migration V26 applied successfully
2026/07/19 15:30:13 [db] applying migration V27: word_doc_generation_flag
2026/07/19 15:30:13 [db] migration V27 applied successfully
2026/07/19 15:30:13 [db] applying migration V28: sports_lookup_flag
2026/07/19 15:30:13 [db] migration V28 applied successfully
2026/07/19 15:30:13 [db] applying migration V29: news_lookup_flag
2026/07/19 15:30:13 [db] migration V29 applied successfully
2026/07/19 15:30:13 [db] applying migration V30: mcp_servers
2026/07/19 15:30:13 [db] migration V30 applied successfully
2026/07/19 15:30:13 [db] applying migration V31: mcp_audit_log
2026/07/19 15:30:13 [db] migration V31 applied successfully
2026/07/19 15:30:13 [db] applying migration V32: mcp_servers_headers
2026/07/19 15:30:13 [db] migration V32 applied successfully
2026/07/19 15:30:13 [db] applying migration V33: file_library_foundation
2026/07/19 15:30:13 [db] migration V33 applied successfully
2026/07/19 15:30:13 [db] applying migration V34: workspace_project_context
2026/07/19 15:30:13 [db] migration V34 applied successfully
2026/07/19 15:30:13 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 15:30:13 [db] migration V35 applied successfully
2026/07/19 15:30:13 [db] applying migration V36: music_studio
2026/07/19 15:30:13 [db] migration V36 applied successfully
2026/07/19 15:30:13 [db] applying migration V37: video_studio_foundation
2026/07/19 15:30:13 [db] migration V37 applied successfully
2026/07/19 15:30:13 [db] applying migration V38: video_studio_timelines
2026/07/19 15:30:13 [db] migration V38 applied successfully
2026/07/19 15:30:13 [db] applying migration V39: video_studio_render_jobs
2026/07/19 15:30:13 [db] migration V39 applied successfully
2026/07/19 15:30:13 [db] applying migration V40: video_generation_input_assets
2026/07/19 15:30:13 [db] migration V40 applied successfully
2026/07/19 15:30:13 [db] applying migration V41: video_render_job_metadata
2026/07/19 15:30:13 [db] migration V41 applied successfully
2026/07/19 15:30:13 [db] applying migration V42: agent_runtime
2026/07/19 15:30:13 [db] migration V42 applied successfully
FAIL
FAIL	github.com/ajbergh/omnillm-studio/internal/rag	2.478s
ok  	github.com/ajbergh/omnillm-studio/internal/repository	14.963s
ok  	github.com/ajbergh/omnillm-studio/internal/filelibrary	1.025s
ok  	github.com/ajbergh/omnillm-studio/internal/document	1.030s
FAIL
```
