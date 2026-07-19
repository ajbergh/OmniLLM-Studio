# Full Backend Test Diagnostic

Exit status: `1`

```text
2026/07/19 16:12:03 [db] applying migration V17: sessions
2026/07/19 16:12:03 [db] migration V17 applied successfully
2026/07/19 16:12:03 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 16:12:03 [db] migration V18 applied successfully
2026/07/19 16:12:03 [db] applying migration V19: installed_plugins
2026/07/19 16:12:03 [db] migration V19 applied successfully
2026/07/19 16:12:03 [db] applying migration V20: eval_runs
2026/07/19 16:12:03 [db] migration V20 applied successfully
2026/07/19 16:12:03 [db] applying migration V21: performance_indexes
2026/07/19 16:12:03 [db] migration V21 applied successfully
2026/07/19 16:12:03 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 16:12:03 [db] migration V22 applied successfully
2026/07/19 16:12:03 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 16:12:03 [db] migration V23 applied successfully
2026/07/19 16:12:03 [db] applying migration V24: image_node_assets_and_references
2026/07/19 16:12:03 [db] migration V24 applied successfully
2026/07/19 16:12:03 [db] applying migration V25: provider_default_image_model
2026/07/19 16:12:03 [db] migration V25 applied successfully
2026/07/19 16:12:03 [db] applying migration V26: conversation_kind
2026/07/19 16:12:03 [db] migration V26 applied successfully
2026/07/19 16:12:03 [db] applying migration V27: word_doc_generation_flag
2026/07/19 16:12:03 [db] migration V27 applied successfully
2026/07/19 16:12:03 [db] applying migration V28: sports_lookup_flag
2026/07/19 16:12:03 [db] migration V28 applied successfully
2026/07/19 16:12:03 [db] applying migration V29: news_lookup_flag
2026/07/19 16:12:03 [db] migration V29 applied successfully
2026/07/19 16:12:03 [db] applying migration V30: mcp_servers
2026/07/19 16:12:03 [db] migration V30 applied successfully
2026/07/19 16:12:03 [db] applying migration V31: mcp_audit_log
2026/07/19 16:12:03 [db] migration V31 applied successfully
2026/07/19 16:12:03 [db] applying migration V32: mcp_servers_headers
2026/07/19 16:12:03 [db] migration V32 applied successfully
2026/07/19 16:12:03 [db] applying migration V33: file_library_foundation
2026/07/19 16:12:03 [db] migration V33 applied successfully
2026/07/19 16:12:03 [db] applying migration V34: workspace_project_context
2026/07/19 16:12:03 [db] migration V34 applied successfully
2026/07/19 16:12:03 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 16:12:03 [db] migration V35 applied successfully
2026/07/19 16:12:03 [db] applying migration V36: music_studio
2026/07/19 16:12:03 [db] migration V36 applied successfully
2026/07/19 16:12:03 [db] applying migration V37: video_studio_foundation
2026/07/19 16:12:03 [db] migration V37 applied successfully
2026/07/19 16:12:03 [db] applying migration V38: video_studio_timelines
2026/07/19 16:12:03 [db] migration V38 applied successfully
2026/07/19 16:12:03 [db] applying migration V39: video_studio_render_jobs
2026/07/19 16:12:03 [db] migration V39 applied successfully
2026/07/19 16:12:03 [db] applying migration V40: video_generation_input_assets
2026/07/19 16:12:03 [db] migration V40 applied successfully
2026/07/19 16:12:03 [db] applying migration V41: video_render_job_metadata
2026/07/19 16:12:03 [db] migration V41 applied successfully
2026/07/19 16:12:03 [db] applying migration V42: agent_runtime
2026/07/19 16:12:03 [db] migration V42 applied successfully
2026/07/19 16:12:03 [db] applying migration V2: feature_flags
2026/07/19 16:12:03 [db] migration V2 applied successfully
2026/07/19 16:12:03 [db] applying migration V3: document_chunks
2026/07/19 16:12:03 [db] migration V3 applied successfully
2026/07/19 16:12:03 [db] applying migration V4: document_embeddings
2026/07/19 16:12:03 [db] migration V4 applied successfully
2026/07/19 16:12:03 [db] applying migration V5: tool_permissions
2026/07/19 16:12:03 [db] migration V5 applied successfully
2026/07/19 16:12:03 [db] applying migration V6: pricing_rules
2026/07/19 16:12:03 [db] migration V6 applied successfully
2026/07/19 16:12:03 [db] applying migration V7: prompt_templates
2026/07/19 16:12:03 [db] migration V7 applied successfully
2026/07/19 16:12:03 [db] applying migration V8: agent_runs
2026/07/19 16:12:03 [db] migration V8 applied successfully
2026/07/19 16:12:03 [db] applying migration V9: agent_steps
2026/07/19 16:12:03 [db] migration V9 applied successfully
2026/07/19 16:12:03 [db] applying migration V10: message_branches
2026/07/19 16:12:03 [db] migration V10 applied successfully
2026/07/19 16:12:03 [db] applying migration V11: conversation_branches
2026/07/19 16:12:03 [db] migration V11 applied successfully
2026/07/19 16:12:03 [db] applying migration V12: message_embeddings
2026/07/19 16:12:03 [db] migration V12 applied successfully
2026/07/19 16:12:03 [db] applying migration V13: workspaces
2026/07/19 16:12:03 [db] migration V13 applied successfully
2026/07/19 16:12:03 [db] applying migration V14: conversations_workspace
2026/07/19 16:12:03 [db] migration V14 applied successfully
2026/07/19 16:12:03 [db] applying migration V15: templates_workspace
2026/07/19 16:12:03 [db] migration V15 applied successfully
2026/07/19 16:12:03 [db] applying migration V16: users
2026/07/19 16:12:03 [db] migration V16 applied successfully
2026/07/19 16:12:03 [db] applying migration V17: sessions
2026/07/19 16:12:03 [db] migration V17 applied successfully
2026/07/19 16:12:03 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 16:12:03 [db] migration V18 applied successfully
2026/07/19 16:12:03 [db] applying migration V19: installed_plugins
2026/07/19 16:12:03 [db] migration V19 applied successfully
2026/07/19 16:12:03 [db] applying migration V20: eval_runs
2026/07/19 16:12:03 [db] migration V20 applied successfully
2026/07/19 16:12:03 [db] applying migration V21: performance_indexes
2026/07/19 16:12:03 [db] migration V21 applied successfully
2026/07/19 16:12:03 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 16:12:03 [db] migration V22 applied successfully
2026/07/19 16:12:03 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 16:12:03 [db] migration V23 applied successfully
2026/07/19 16:12:03 [db] applying migration V24: image_node_assets_and_references
2026/07/19 16:12:03 [db] migration V24 applied successfully
2026/07/19 16:12:03 [db] applying migration V25: provider_default_image_model
2026/07/19 16:12:03 [db] migration V25 applied successfully
2026/07/19 16:12:03 [db] applying migration V26: conversation_kind
2026/07/19 16:12:03 [db] migration V26 applied successfully
2026/07/19 16:12:03 [db] applying migration V27: word_doc_generation_flag
2026/07/19 16:12:03 [db] migration V27 applied successfully
2026/07/19 16:12:03 [db] applying migration V28: sports_lookup_flag
2026/07/19 16:12:03 [db] migration V28 applied successfully
2026/07/19 16:12:03 [db] applying migration V29: news_lookup_flag
2026/07/19 16:12:03 [db] migration V29 applied successfully
2026/07/19 16:12:03 [db] applying migration V30: mcp_servers
2026/07/19 16:12:03 [db] migration V30 applied successfully
2026/07/19 16:12:03 [db] applying migration V31: mcp_audit_log
2026/07/19 16:12:03 [db] migration V31 applied successfully
2026/07/19 16:12:03 [db] applying migration V32: mcp_servers_headers
2026/07/19 16:12:03 [db] migration V32 applied successfully
2026/07/19 16:12:03 [db] applying migration V33: file_library_foundation
2026/07/19 16:12:03 [db] migration V33 applied successfully
2026/07/19 16:12:03 [db] applying migration V34: workspace_project_context
2026/07/19 16:12:03 [db] migration V34 applied successfully
2026/07/19 16:12:03 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 16:12:03 [db] migration V35 applied successfully
2026/07/19 16:12:03 [db] applying migration V36: music_studio
2026/07/19 16:12:03 [db] migration V36 applied successfully
2026/07/19 16:12:03 [db] applying migration V37: video_studio_foundation
2026/07/19 16:12:03 [db] migration V37 applied successfully
2026/07/19 16:12:03 [db] applying migration V38: video_studio_timelines
2026/07/19 16:12:03 [db] migration V38 applied successfully
2026/07/19 16:12:03 [db] applying migration V39: video_studio_render_jobs
2026/07/19 16:12:03 [db] migration V39 applied successfully
2026/07/19 16:12:03 [db] applying migration V40: video_generation_input_assets
2026/07/19 16:12:03 [db] migration V40 applied successfully
2026/07/19 16:12:03 [db] applying migration V41: video_render_job_metadata
2026/07/19 16:12:03 [db] migration V41 applied successfully
2026/07/19 16:12:03 [db] applying migration V42: agent_runtime
2026/07/19 16:12:03 [db] migration V42 applied successfully
2026/07/19 16:12:03 [db] applying migration V2: feature_flags
2026/07/19 16:12:03 [db] migration V2 applied successfully
2026/07/19 16:12:03 [db] applying migration V3: document_chunks
2026/07/19 16:12:03 [db] migration V3 applied successfully
2026/07/19 16:12:03 [db] applying migration V4: document_embeddings
2026/07/19 16:12:03 [db] migration V4 applied successfully
2026/07/19 16:12:03 [db] applying migration V5: tool_permissions
2026/07/19 16:12:03 [db] migration V5 applied successfully
2026/07/19 16:12:03 [db] applying migration V6: pricing_rules
2026/07/19 16:12:03 [db] migration V6 applied successfully
2026/07/19 16:12:03 [db] applying migration V7: prompt_templates
2026/07/19 16:12:03 [db] migration V7 applied successfully
2026/07/19 16:12:03 [db] applying migration V8: agent_runs
2026/07/19 16:12:03 [db] migration V8 applied successfully
2026/07/19 16:12:03 [db] applying migration V9: agent_steps
2026/07/19 16:12:03 [db] migration V9 applied successfully
2026/07/19 16:12:03 [db] applying migration V10: message_branches
2026/07/19 16:12:03 [db] migration V10 applied successfully
2026/07/19 16:12:03 [db] applying migration V11: conversation_branches
2026/07/19 16:12:03 [db] migration V11 applied successfully
2026/07/19 16:12:03 [db] applying migration V12: message_embeddings
2026/07/19 16:12:03 [db] migration V12 applied successfully
2026/07/19 16:12:03 [db] applying migration V13: workspaces
2026/07/19 16:12:03 [db] migration V13 applied successfully
2026/07/19 16:12:03 [db] applying migration V14: conversations_workspace
2026/07/19 16:12:03 [db] migration V14 applied successfully
2026/07/19 16:12:03 [db] applying migration V15: templates_workspace
2026/07/19 16:12:03 [db] migration V15 applied successfully
2026/07/19 16:12:03 [db] applying migration V16: users
2026/07/19 16:12:03 [db] migration V16 applied successfully
2026/07/19 16:12:03 [db] applying migration V17: sessions
2026/07/19 16:12:03 [db] migration V17 applied successfully
2026/07/19 16:12:03 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 16:12:03 [db] migration V18 applied successfully
2026/07/19 16:12:03 [db] applying migration V19: installed_plugins
2026/07/19 16:12:03 [db] migration V19 applied successfully
2026/07/19 16:12:03 [db] applying migration V20: eval_runs
2026/07/19 16:12:03 [db] migration V20 applied successfully
2026/07/19 16:12:03 [db] applying migration V21: performance_indexes
2026/07/19 16:12:03 [db] migration V21 applied successfully
2026/07/19 16:12:03 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 16:12:03 [db] migration V22 applied successfully
2026/07/19 16:12:03 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 16:12:03 [db] migration V23 applied successfully
2026/07/19 16:12:03 [db] applying migration V24: image_node_assets_and_references
2026/07/19 16:12:03 [db] migration V24 applied successfully
2026/07/19 16:12:03 [db] applying migration V25: provider_default_image_model
2026/07/19 16:12:03 [db] migration V25 applied successfully
2026/07/19 16:12:03 [db] applying migration V26: conversation_kind
2026/07/19 16:12:03 [db] migration V26 applied successfully
2026/07/19 16:12:03 [db] applying migration V27: word_doc_generation_flag
2026/07/19 16:12:03 [db] migration V27 applied successfully
2026/07/19 16:12:03 [db] applying migration V28: sports_lookup_flag
2026/07/19 16:12:03 [db] migration V28 applied successfully
2026/07/19 16:12:03 [db] applying migration V29: news_lookup_flag
2026/07/19 16:12:03 [db] migration V29 applied successfully
2026/07/19 16:12:03 [db] applying migration V30: mcp_servers
2026/07/19 16:12:03 [db] migration V30 applied successfully
2026/07/19 16:12:03 [db] applying migration V31: mcp_audit_log
2026/07/19 16:12:03 [db] migration V31 applied successfully
2026/07/19 16:12:03 [db] applying migration V32: mcp_servers_headers
2026/07/19 16:12:03 [db] migration V32 applied successfully
2026/07/19 16:12:03 [db] applying migration V33: file_library_foundation
2026/07/19 16:12:03 [db] migration V33 applied successfully
2026/07/19 16:12:03 [db] applying migration V34: workspace_project_context
2026/07/19 16:12:03 [db] migration V34 applied successfully
2026/07/19 16:12:03 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 16:12:03 [db] migration V35 applied successfully
2026/07/19 16:12:03 [db] applying migration V36: music_studio
2026/07/19 16:12:03 [db] migration V36 applied successfully
2026/07/19 16:12:03 [db] applying migration V37: video_studio_foundation
2026/07/19 16:12:03 [db] migration V37 applied successfully
2026/07/19 16:12:03 [db] applying migration V38: video_studio_timelines
2026/07/19 16:12:03 [db] migration V38 applied successfully
2026/07/19 16:12:03 [db] applying migration V39: video_studio_render_jobs
2026/07/19 16:12:03 [db] migration V39 applied successfully
2026/07/19 16:12:03 [db] applying migration V40: video_generation_input_assets
2026/07/19 16:12:03 [db] migration V40 applied successfully
2026/07/19 16:12:03 [db] applying migration V41: video_render_job_metadata
2026/07/19 16:12:03 [db] migration V41 applied successfully
2026/07/19 16:12:03 [db] applying migration V42: agent_runtime
2026/07/19 16:12:03 [db] migration V42 applied successfully
panic: test timed out after 10m0s
	running tests:
		TestExecuteToolCallAskApprovalApproved (9m59s)

goroutine 931 [running]:
testing.(*M).startAlarm.func1()
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2682 +0x345
created by time.goFunc
	/opt/hostedtoolcache/go/1.25.0/x64/src/time/sleep.go:215 +0x2d

goroutine 1 [chan receive, 9 minutes]:
testing.(*T).Run(0xc000110540, {0xaf34d3?, 0xc0003b5b30?}, 0xd73d40)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2005 +0x485
testing.runTests.func1(0xc000110540)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2477 +0x37
testing.tRunner(0xc000110540, 0xc0003b5c70)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:1934 +0xea
testing.runTests(0xc000010678, {0x11f6140, 0x29, 0x29}, {0x7?, 0x482a72?, 0x1214540?})
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2475 +0x4b4
testing.(*M).Run(0xc0003d0320)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2337 +0x63a
main.main()
	_testmain.go:125 +0x9b

goroutine 886 [chan send, 9 minutes]:
github.com/ajbergh/omnillm-studio/internal/agent.TestExecuteToolCallAskApprovalApproved.func3({{0xad5f5a, 0xa}, {0xc0000c8840, 0x24}, {0xc0000c8960, 0x24}, {0xa87320, 0xc0001967e0}})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner_test.go:504 +0x45
github.com/ajbergh/omnillm-studio/internal/agent.emit(0xc0001c4850, {{0xad5f5a, 0xa}, {0xc0000c8840, 0x24}, {0xc0000c8960, 0x24}, {0xa87320, 0xc0001967e0}})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner.go:836 +0xb6
github.com/ajbergh/omnillm-studio/internal/agent.(*Runner).executeToolCall.func1({{0xadac7d, 0xe}, {0xc0000c8960, 0x24}, {0xc0001c2ac0, 0xd}, {{0xacd8d4, 0x5}, {0x0, 0x0}, ...}, ...})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner.go:477 +0x1a5
github.com/ajbergh/omnillm-studio/internal/tools.emitEvent({0xdf78b8, 0xc0000ba5a0}, {{0xadac7d, 0xe}, {0xc0000c8960, 0x24}, {0xc0001c2ac0, 0xd}, {{0xacd8d4, 0x5}, ...}, ...})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/tools/context.go:85 +0xd8
github.com/ajbergh/omnillm-studio/internal/tools.(*Executor).Execute(0xc0001a8920, {0xdf78b8, 0xc0000ba5a0}, {{0xc0000c8960, 0x24}, {0xc0001c2ac0, 0xd}, {0xc0001c2ad0, 0xd, 0x10}})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/tools/executor.go:178 +0x1688
github.com/ajbergh/omnillm-studio/internal/agent.(*Runner).executeToolCall(0xc0001a7200, {0xdf7848, 0x1236360}, 0xc0001c4840, 0xc0003d0e60, 0xc0001269a0, 0xc0001c4850)
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner.go:482 +0x3d5
github.com/ajbergh/omnillm-studio/internal/agent.TestExecuteToolCallAskApprovalApproved(0xc0003f8e00)
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner_test.go:503 +0x329
testing.tRunner(0xc0003f8e00, 0xd73d40)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:1934 +0xea
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:1997 +0x465

goroutine 887 [select, 9 minutes]:
database/sql.(*DB).connectionOpener(0xc0001941a0, {0xdf78f0, 0xc00009bef0})
	/opt/hostedtoolcache/go/1.25.0/x64/src/database/sql/sql.go:1261 +0x87
created by database/sql.OpenDB in goroutine 886
	/opt/hostedtoolcache/go/1.25.0/x64/src/database/sql/sql.go:841 +0x130
FAIL	github.com/ajbergh/omnillm-studio/internal/agent	600.089s
?   	github.com/ajbergh/omnillm-studio/internal/analytics	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/api	0.267s
?   	github.com/ajbergh/omnillm-studio/internal/apps	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/artifacts	0.010s
?   	github.com/ajbergh/omnillm-studio/internal/auth	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/browser	0.004s
ok  	github.com/ajbergh/omnillm-studio/internal/bundle	0.107s
?   	github.com/ajbergh/omnillm-studio/internal/config	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/crypto	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/db	0.053s
ok  	github.com/ajbergh/omnillm-studio/internal/document	0.007s
?   	github.com/ajbergh/omnillm-studio/internal/eval	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/filelibrary	0.007s
?   	github.com/ajbergh/omnillm-studio/internal/jobs	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/llm	0.004s
ok  	github.com/ajbergh/omnillm-studio/internal/mcpclient	0.018s
?   	github.com/ajbergh/omnillm-studio/internal/memory	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/models	0.003s
?   	github.com/ajbergh/omnillm-studio/internal/music	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/plugins	0.410s
ok  	github.com/ajbergh/omnillm-studio/internal/rag	0.451s
ok  	github.com/ajbergh/omnillm-studio/internal/repository	0.608s
ok  	github.com/ajbergh/omnillm-studio/internal/router	0.008s
?   	github.com/ajbergh/omnillm-studio/internal/search	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/sports	0.144s
?   	github.com/ajbergh/omnillm-studio/internal/tasks	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/tasktools	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/templates	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/tools	0.009s
ok  	github.com/ajbergh/omnillm-studio/internal/urlcontext	0.005s
ok  	github.com/ajbergh/omnillm-studio/internal/video	0.090s
ok  	github.com/ajbergh/omnillm-studio/internal/websearch	0.006s
ok  	github.com/ajbergh/omnillm-studio/internal/wordgen	0.007s
FAIL
```
