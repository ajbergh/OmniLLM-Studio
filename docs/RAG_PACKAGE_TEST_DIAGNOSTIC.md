# Package-Bounded Backend Test Diagnostic

Exit status: `1`

```text
2026/07/19 16:12:07 [db] migration V19 applied successfully
2026/07/19 16:12:07 [db] applying migration V20: eval_runs
2026/07/19 16:12:07 [db] migration V20 applied successfully
2026/07/19 16:12:07 [db] applying migration V21: performance_indexes
2026/07/19 16:12:07 [db] migration V21 applied successfully
2026/07/19 16:12:07 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 16:12:07 [db] migration V22 applied successfully
2026/07/19 16:12:07 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 16:12:07 [db] migration V23 applied successfully
2026/07/19 16:12:07 [db] applying migration V24: image_node_assets_and_references
2026/07/19 16:12:07 [db] migration V24 applied successfully
2026/07/19 16:12:07 [db] applying migration V25: provider_default_image_model
2026/07/19 16:12:07 [db] migration V25 applied successfully
2026/07/19 16:12:07 [db] applying migration V26: conversation_kind
2026/07/19 16:12:07 [db] migration V26 applied successfully
2026/07/19 16:12:07 [db] applying migration V27: word_doc_generation_flag
2026/07/19 16:12:07 [db] migration V27 applied successfully
2026/07/19 16:12:07 [db] applying migration V28: sports_lookup_flag
2026/07/19 16:12:07 [db] migration V28 applied successfully
2026/07/19 16:12:07 [db] applying migration V29: news_lookup_flag
2026/07/19 16:12:07 [db] migration V29 applied successfully
2026/07/19 16:12:07 [db] applying migration V30: mcp_servers
2026/07/19 16:12:07 [db] migration V30 applied successfully
2026/07/19 16:12:07 [db] applying migration V31: mcp_audit_log
2026/07/19 16:12:07 [db] migration V31 applied successfully
2026/07/19 16:12:07 [db] applying migration V32: mcp_servers_headers
2026/07/19 16:12:07 [db] migration V32 applied successfully
2026/07/19 16:12:07 [db] applying migration V33: file_library_foundation
2026/07/19 16:12:07 [db] migration V33 applied successfully
2026/07/19 16:12:07 [db] applying migration V34: workspace_project_context
2026/07/19 16:12:07 [db] migration V34 applied successfully
2026/07/19 16:12:07 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 16:12:07 [db] migration V35 applied successfully
2026/07/19 16:12:07 [db] applying migration V36: music_studio
2026/07/19 16:12:07 [db] migration V36 applied successfully
2026/07/19 16:12:07 [db] applying migration V37: video_studio_foundation
2026/07/19 16:12:07 [db] migration V37 applied successfully
2026/07/19 16:12:07 [db] applying migration V38: video_studio_timelines
2026/07/19 16:12:07 [db] migration V38 applied successfully
2026/07/19 16:12:07 [db] applying migration V39: video_studio_render_jobs
2026/07/19 16:12:07 [db] migration V39 applied successfully
2026/07/19 16:12:07 [db] applying migration V40: video_generation_input_assets
2026/07/19 16:12:07 [db] migration V40 applied successfully
2026/07/19 16:12:07 [db] applying migration V41: video_render_job_metadata
2026/07/19 16:12:07 [db] migration V41 applied successfully
2026/07/19 16:12:07 [db] applying migration V42: agent_runtime
2026/07/19 16:12:07 [db] migration V42 applied successfully
panic: test timed out after 1m30s
	running tests:
		TestExecuteToolCallAskApprovalApproved (1m29s)

goroutine 895 [running]:
testing.(*M).startAlarm.func1()
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2682 +0x345
created by time.goFunc
	/opt/hostedtoolcache/go/1.25.0/x64/src/time/sleep.go:215 +0x2d

goroutine 1 [chan receive]:
testing.(*T).Run(0xc000110540, {0xaf34d3?, 0xc0003b5b30?}, 0xd73d40)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2005 +0x485
testing.runTests.func1(0xc000110540)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2477 +0x37
testing.tRunner(0xc000110540, 0xc0003b5c70)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:1934 +0xea
testing.runTests(0xc000010678, {0x11f6140, 0x29, 0x29}, {0x7?, 0xc00036d080?, 0x1214540?})
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2475 +0x4b4
testing.(*M).Run(0xc0003cc460)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:2337 +0x63a
main.main()
	_testmain.go:125 +0x9b

goroutine 689 [chan send]:
github.com/ajbergh/omnillm-studio/internal/agent.TestExecuteToolCallAskApprovalApproved.func3({{0xad5f5a, 0xa}, {0xc000182f30, 0x24}, {0xc000183050, 0x24}, {0xa87320, 0xc00018cb40}})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner_test.go:504 +0x45
github.com/ajbergh/omnillm-studio/internal/agent.emit(0xc0002007f0, {{0xad5f5a, 0xa}, {0xc000182f30, 0x24}, {0xc000183050, 0x24}, {0xa87320, 0xc00018cb40}})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner.go:836 +0xb6
github.com/ajbergh/omnillm-studio/internal/agent.(*Runner).executeToolCall.func1({{0xadac7d, 0xe}, {0xc000183050, 0x24}, {0xc0001feea0, 0xd}, {{0xacd8d4, 0x5}, {0x0, 0x0}, ...}, ...})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner.go:477 +0x1a5
github.com/ajbergh/omnillm-studio/internal/tools.emitEvent({0xdf78b8, 0xc00032b890}, {{0xadac7d, 0xe}, {0xc000183050, 0x24}, {0xc0001feea0, 0xd}, {{0xacd8d4, 0x5}, ...}, ...})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/tools/context.go:85 +0xd8
github.com/ajbergh/omnillm-studio/internal/tools.(*Executor).Execute(0xc0001f6c60, {0xdf78b8, 0xc00032b890}, {{0xc000183050, 0x24}, {0xc0001feea0, 0xd}, {0xc0001feeb0, 0xd, 0x10}})
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/tools/executor.go:178 +0x1688
github.com/ajbergh/omnillm-studio/internal/agent.(*Runner).executeToolCall(0xc0001e7c80, {0xdf7848, 0x1236360}, 0xc0002007e0, 0xc0001f46e0, 0xc0001904d0, 0xc0002007f0)
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner.go:482 +0x3d5
github.com/ajbergh/omnillm-studio/internal/agent.TestExecuteToolCallAskApprovalApproved(0xc0001a61c0)
	/home/runner/work/OmniLLM-Studio/OmniLLM-Studio/backend/internal/agent/runner_test.go:503 +0x329
testing.tRunner(0xc0001a61c0, 0xd73d40)
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:1934 +0xea
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.25.0/x64/src/testing/testing.go:1997 +0x465

goroutine 898 [select]:
database/sql.(*DB).connectionOpener(0xc00058d790, {0xdf78f0, 0xc0001e43c0})
	/opt/hostedtoolcache/go/1.25.0/x64/src/database/sql/sql.go:1261 +0x87
created by database/sql.OpenDB in goroutine 689
	/opt/hostedtoolcache/go/1.25.0/x64/src/database/sql/sql.go:841 +0x130
FAIL	github.com/ajbergh/omnillm-studio/internal/agent	90.011s
?   	github.com/ajbergh/omnillm-studio/internal/analytics	[no test files]
2026/07/19 16:12:08 [db] applying migration V2: feature_flags
2026/07/19 16:12:08 [db] migration V2 applied successfully
2026/07/19 16:12:08 [db] applying migration V3: document_chunks
2026/07/19 16:12:08 [db] migration V3 applied successfully
2026/07/19 16:12:08 [db] applying migration V4: document_embeddings
2026/07/19 16:12:08 [db] migration V4 applied successfully
2026/07/19 16:12:08 [db] applying migration V5: tool_permissions
2026/07/19 16:12:08 [db] migration V5 applied successfully
2026/07/19 16:12:08 [db] applying migration V6: pricing_rules
2026/07/19 16:12:08 [db] migration V6 applied successfully
2026/07/19 16:12:08 [db] applying migration V7: prompt_templates
2026/07/19 16:12:08 [db] migration V7 applied successfully
2026/07/19 16:12:08 [db] applying migration V8: agent_runs
2026/07/19 16:12:08 [db] migration V8 applied successfully
2026/07/19 16:12:08 [db] applying migration V9: agent_steps
2026/07/19 16:12:08 [db] migration V9 applied successfully
2026/07/19 16:12:08 [db] applying migration V10: message_branches
2026/07/19 16:12:08 [db] migration V10 applied successfully
2026/07/19 16:12:08 [db] applying migration V11: conversation_branches
2026/07/19 16:12:08 [db] migration V11 applied successfully
2026/07/19 16:12:08 [db] applying migration V12: message_embeddings
2026/07/19 16:12:08 [db] migration V12 applied successfully
2026/07/19 16:12:08 [db] applying migration V13: workspaces
2026/07/19 16:12:08 [db] migration V13 applied successfully
2026/07/19 16:12:08 [db] applying migration V14: conversations_workspace
2026/07/19 16:12:08 [db] migration V14 applied successfully
2026/07/19 16:12:08 [db] applying migration V15: templates_workspace
2026/07/19 16:12:08 [db] migration V15 applied successfully
2026/07/19 16:12:08 [db] applying migration V16: users
2026/07/19 16:12:08 [db] migration V16 applied successfully
2026/07/19 16:12:08 [db] applying migration V17: sessions
2026/07/19 16:12:08 [db] migration V17 applied successfully
2026/07/19 16:12:08 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 16:12:08 [db] migration V18 applied successfully
2026/07/19 16:12:08 [db] applying migration V19: installed_plugins
2026/07/19 16:12:08 [db] migration V19 applied successfully
2026/07/19 16:12:08 [db] applying migration V20: eval_runs
2026/07/19 16:12:08 [db] migration V20 applied successfully
2026/07/19 16:12:08 [db] applying migration V21: performance_indexes
2026/07/19 16:12:08 [db] migration V21 applied successfully
2026/07/19 16:12:08 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 16:12:08 [db] migration V22 applied successfully
2026/07/19 16:12:08 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 16:12:08 [db] migration V23 applied successfully
2026/07/19 16:12:08 [db] applying migration V24: image_node_assets_and_references
2026/07/19 16:12:08 [db] migration V24 applied successfully
2026/07/19 16:12:08 [db] applying migration V25: provider_default_image_model
2026/07/19 16:12:08 [db] migration V25 applied successfully
2026/07/19 16:12:08 [db] applying migration V26: conversation_kind
2026/07/19 16:12:08 [db] migration V26 applied successfully
2026/07/19 16:12:08 [db] applying migration V27: word_doc_generation_flag
2026/07/19 16:12:08 [db] migration V27 applied successfully
2026/07/19 16:12:08 [db] applying migration V28: sports_lookup_flag
2026/07/19 16:12:08 [db] migration V28 applied successfully
2026/07/19 16:12:08 [db] applying migration V29: news_lookup_flag
2026/07/19 16:12:08 [db] migration V29 applied successfully
2026/07/19 16:12:08 [db] applying migration V30: mcp_servers
2026/07/19 16:12:08 [db] migration V30 applied successfully
2026/07/19 16:12:08 [db] applying migration V31: mcp_audit_log
2026/07/19 16:12:08 [db] migration V31 applied successfully
2026/07/19 16:12:08 [db] applying migration V32: mcp_servers_headers
2026/07/19 16:12:08 [db] migration V32 applied successfully
2026/07/19 16:12:08 [db] applying migration V33: file_library_foundation
2026/07/19 16:12:08 [db] migration V33 applied successfully
2026/07/19 16:12:08 [db] applying migration V34: workspace_project_context
2026/07/19 16:12:08 [db] migration V34 applied successfully
2026/07/19 16:12:08 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 16:12:08 [db] migration V35 applied successfully
2026/07/19 16:12:08 [db] applying migration V36: music_studio
2026/07/19 16:12:08 [db] migration V36 applied successfully
2026/07/19 16:12:08 [db] applying migration V37: video_studio_foundation
2026/07/19 16:12:08 [db] migration V37 applied successfully
2026/07/19 16:12:08 [db] applying migration V38: video_studio_timelines
2026/07/19 16:12:08 [db] migration V38 applied successfully
2026/07/19 16:12:08 [db] applying migration V39: video_studio_render_jobs
2026/07/19 16:12:08 [db] migration V39 applied successfully
2026/07/19 16:12:08 [db] applying migration V40: video_generation_input_assets
2026/07/19 16:12:08 [db] migration V40 applied successfully
2026/07/19 16:12:08 [db] applying migration V41: video_render_job_metadata
2026/07/19 16:12:08 [db] migration V41 applied successfully
2026/07/19 16:12:08 [db] applying migration V42: agent_runtime
2026/07/19 16:12:08 [db] migration V42 applied successfully
2026/07/19 16:12:08 [websearch] no search API key configured; using DuckDuckGo (zero-config)
2026/07/19 16:12:08 [websearch] Jina Reader enabled (maxLen=3000 chars per page)
2026/07/19 16:12:08 [analytics] seeded 80 new default pricing rules
2026/07/19 16:12:08 [templates] seeding 5 built-in templates
2026/07/19 16:12:08 [WARN] Running in solo mode (no users). Register an account to enable authentication.
2026/07/19 16:12:08 "GET http://example.com/v1/apps/catalog HTTP/1.1" from 192.0.2.1:1234 - 200 2343B in 163.005µs
2026/07/19 16:12:08 "GET http://example.com/v1/apps/connections HTTP/1.1" from 192.0.2.1:1234 - 200 3B in 179.015µs
2026/07/19 16:12:08 "GET http://example.com/v1/jobs/ HTTP/1.1" from 192.0.2.1:1234 - 200 3B in 262.883µs
2026/07/19 16:12:08 "GET http://example.com/v1/memories/ HTTP/1.1" from 192.0.2.1:1234 - 200 3B in 415.82µs
2026/07/19 16:12:08 "GET http://example.com/v1/tasks/ HTTP/1.1" from 192.0.2.1:1234 - 200 3B in 224.19µs
2026/07/19 16:12:08 "GET http://example.com/v1/tools/approvals HTTP/1.1" from 192.0.2.1:1234 - 200 3B in 103.064µs
2026/07/19 16:12:08 "GET http://example.com/v1/eval/agent/scenarios HTTP/1.1" from 192.0.2.1:1234 - 200 1802B in 99.697µs
2026/07/19 16:12:08 "GET http://example.com/v1/agent/runs/missing/events HTTP/1.1" from 192.0.2.1:1234 - 404 32B in 94.296µs
2026/07/19 16:12:08 "GET http://example.com/v1/tools/ HTTP/1.1" from 192.0.2.1:1234 - 200 26702B in 5.716483ms
2026/07/19 16:12:08 [db] applying migration V2: feature_flags
2026/07/19 16:12:08 [db] migration V2 applied successfully
2026/07/19 16:12:08 [db] applying migration V3: document_chunks
2026/07/19 16:12:08 [db] migration V3 applied successfully
2026/07/19 16:12:08 [db] applying migration V4: document_embeddings
2026/07/19 16:12:08 [db] migration V4 applied successfully
2026/07/19 16:12:08 [db] applying migration V5: tool_permissions
2026/07/19 16:12:08 [db] migration V5 applied successfully
2026/07/19 16:12:08 [db] applying migration V6: pricing_rules
2026/07/19 16:12:08 [db] migration V6 applied successfully
2026/07/19 16:12:08 [db] applying migration V7: prompt_templates
2026/07/19 16:12:08 [db] migration V7 applied successfully
2026/07/19 16:12:08 [db] applying migration V8: agent_runs
2026/07/19 16:12:08 [db] migration V8 applied successfully
2026/07/19 16:12:08 [db] applying migration V9: agent_steps
2026/07/19 16:12:08 [db] migration V9 applied successfully
2026/07/19 16:12:08 [db] applying migration V10: message_branches
2026/07/19 16:12:08 [db] migration V10 applied successfully
2026/07/19 16:12:08 [db] applying migration V11: conversation_branches
2026/07/19 16:12:08 [db] migration V11 applied successfully
2026/07/19 16:12:08 [db] applying migration V12: message_embeddings
2026/07/19 16:12:08 [db] migration V12 applied successfully
2026/07/19 16:12:08 [db] applying migration V13: workspaces
2026/07/19 16:12:08 [db] migration V13 applied successfully
2026/07/19 16:12:08 [db] applying migration V14: conversations_workspace
2026/07/19 16:12:08 [db] migration V14 applied successfully
2026/07/19 16:12:08 [db] applying migration V15: templates_workspace
2026/07/19 16:12:08 [db] migration V15 applied successfully
2026/07/19 16:12:08 [db] applying migration V16: users
2026/07/19 16:12:08 [db] migration V16 applied successfully
2026/07/19 16:12:08 [db] applying migration V17: sessions
2026/07/19 16:12:08 [db] migration V17 applied successfully
2026/07/19 16:12:08 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 16:12:08 [db] migration V18 applied successfully
2026/07/19 16:12:08 [db] applying migration V19: installed_plugins
2026/07/19 16:12:08 [db] migration V19 applied successfully
2026/07/19 16:12:08 [db] applying migration V20: eval_runs
2026/07/19 16:12:08 [db] migration V20 applied successfully
2026/07/19 16:12:08 [db] applying migration V21: performance_indexes
2026/07/19 16:12:08 [db] migration V21 applied successfully
2026/07/19 16:12:08 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 16:12:08 [db] migration V22 applied successfully
2026/07/19 16:12:08 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 16:12:08 [db] migration V23 applied successfully
2026/07/19 16:12:08 [db] applying migration V24: image_node_assets_and_references
2026/07/19 16:12:08 [db] migration V24 applied successfully
2026/07/19 16:12:08 [db] applying migration V25: provider_default_image_model
2026/07/19 16:12:08 [db] migration V25 applied successfully
2026/07/19 16:12:08 [db] applying migration V26: conversation_kind
2026/07/19 16:12:08 [db] migration V26 applied successfully
2026/07/19 16:12:08 [db] applying migration V27: word_doc_generation_flag
2026/07/19 16:12:08 [db] migration V27 applied successfully
2026/07/19 16:12:08 [db] applying migration V28: sports_lookup_flag
2026/07/19 16:12:08 [db] migration V28 applied successfully
2026/07/19 16:12:08 [db] applying migration V29: news_lookup_flag
2026/07/19 16:12:08 [db] migration V29 applied successfully
2026/07/19 16:12:08 [db] applying migration V30: mcp_servers
2026/07/19 16:12:08 [db] migration V30 applied successfully
2026/07/19 16:12:08 [db] applying migration V31: mcp_audit_log
2026/07/19 16:12:08 [db] migration V31 applied successfully
2026/07/19 16:12:08 [db] applying migration V32: mcp_servers_headers
2026/07/19 16:12:08 [db] migration V32 applied successfully
2026/07/19 16:12:08 [db] applying migration V33: file_library_foundation
2026/07/19 16:12:08 [db] migration V33 applied successfully
2026/07/19 16:12:08 [db] applying migration V34: workspace_project_context
2026/07/19 16:12:08 [db] migration V34 applied successfully
2026/07/19 16:12:08 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 16:12:08 [db] migration V35 applied successfully
2026/07/19 16:12:08 [db] applying migration V36: music_studio
2026/07/19 16:12:08 [db] migration V36 applied successfully
2026/07/19 16:12:08 [db] applying migration V37: video_studio_foundation
2026/07/19 16:12:08 [db] migration V37 applied successfully
2026/07/19 16:12:08 [db] applying migration V38: video_studio_timelines
2026/07/19 16:12:08 [db] migration V38 applied successfully
2026/07/19 16:12:08 [db] applying migration V39: video_studio_render_jobs
2026/07/19 16:12:08 [db] migration V39 applied successfully
2026/07/19 16:12:08 [db] applying migration V40: video_generation_input_assets
2026/07/19 16:12:08 [db] migration V40 applied successfully
2026/07/19 16:12:08 [db] applying migration V41: video_render_job_metadata
2026/07/19 16:12:08 [db] migration V41 applied successfully
2026/07/19 16:12:08 [db] applying migration V42: agent_runtime
2026/07/19 16:12:08 [db] migration V42 applied successfully
2026/07/19 16:12:08 [websearch] no search API key configured; using DuckDuckGo (zero-config)
2026/07/19 16:12:08 [websearch] Jina Reader enabled (maxLen=3000 chars per page)
2026/07/19 16:12:08 [analytics] seeded 80 new default pricing rules
2026/07/19 16:12:08 [templates] seeding 5 built-in templates
2026/07/19 16:12:08 [db] applying migration V2: feature_flags
2026/07/19 16:12:08 [db] migration V2 applied successfully
2026/07/19 16:12:08 [db] applying migration V3: document_chunks
2026/07/19 16:12:08 [db] migration V3 applied successfully
2026/07/19 16:12:08 [db] applying migration V4: document_embeddings
2026/07/19 16:12:08 [db] migration V4 applied successfully
2026/07/19 16:12:08 [db] applying migration V5: tool_permissions
2026/07/19 16:12:08 [db] migration V5 applied successfully
2026/07/19 16:12:08 [db] applying migration V6: pricing_rules
2026/07/19 16:12:08 [db] migration V6 applied successfully
2026/07/19 16:12:08 [db] applying migration V7: prompt_templates
2026/07/19 16:12:08 [db] migration V7 applied successfully
2026/07/19 16:12:08 [db] applying migration V8: agent_runs
2026/07/19 16:12:08 [db] migration V8 applied successfully
2026/07/19 16:12:08 [db] applying migration V9: agent_steps
2026/07/19 16:12:08 [db] migration V9 applied successfully
2026/07/19 16:12:08 [db] applying migration V10: message_branches
2026/07/19 16:12:08 [db] migration V10 applied successfully
2026/07/19 16:12:08 [db] applying migration V11: conversation_branches
2026/07/19 16:12:08 [db] migration V11 applied successfully
2026/07/19 16:12:08 [db] applying migration V12: message_embeddings
2026/07/19 16:12:08 [db] migration V12 applied successfully
2026/07/19 16:12:08 [db] applying migration V13: workspaces
2026/07/19 16:12:08 [db] migration V13 applied successfully
2026/07/19 16:12:08 [db] applying migration V14: conversations_workspace
2026/07/19 16:12:08 [db] migration V14 applied successfully
2026/07/19 16:12:08 [db] applying migration V15: templates_workspace
2026/07/19 16:12:08 [db] migration V15 applied successfully
2026/07/19 16:12:08 [db] applying migration V16: users
2026/07/19 16:12:08 [db] migration V16 applied successfully
2026/07/19 16:12:08 [db] applying migration V17: sessions
2026/07/19 16:12:08 [db] migration V17 applied successfully
2026/07/19 16:12:08 [db] applying migration V18: workspace_members_and_user_refs
2026/07/19 16:12:08 [db] migration V18 applied successfully
2026/07/19 16:12:08 [db] applying migration V19: installed_plugins
2026/07/19 16:12:08 [db] migration V19 applied successfully
2026/07/19 16:12:08 [db] applying migration V20: eval_runs
2026/07/19 16:12:08 [db] migration V20 applied successfully
2026/07/19 16:12:08 [db] applying migration V21: performance_indexes
2026/07/19 16:12:08 [db] migration V21 applied successfully
2026/07/19 16:12:08 [db] applying migration V22: agent_runs_awaiting_approval
2026/07/19 16:12:08 [db] migration V22 applied successfully
2026/07/19 16:12:08 [db] applying migration V23: image_sessions_and_nodes
2026/07/19 16:12:08 [db] migration V23 applied successfully
2026/07/19 16:12:08 [db] applying migration V24: image_node_assets_and_references
2026/07/19 16:12:08 [db] migration V24 applied successfully
2026/07/19 16:12:08 [db] applying migration V25: provider_default_image_model
2026/07/19 16:12:08 [db] migration V25 applied successfully
2026/07/19 16:12:08 [db] applying migration V26: conversation_kind
2026/07/19 16:12:08 [db] migration V26 applied successfully
2026/07/19 16:12:08 [db] applying migration V27: word_doc_generation_flag
2026/07/19 16:12:08 [db] migration V27 applied successfully
2026/07/19 16:12:08 [db] applying migration V28: sports_lookup_flag
2026/07/19 16:12:08 [db] migration V28 applied successfully
2026/07/19 16:12:08 [db] applying migration V29: news_lookup_flag
2026/07/19 16:12:08 [db] migration V29 applied successfully
2026/07/19 16:12:08 [db] applying migration V30: mcp_servers
2026/07/19 16:12:08 [db] migration V30 applied successfully
2026/07/19 16:12:08 [db] applying migration V31: mcp_audit_log
2026/07/19 16:12:08 [db] migration V31 applied successfully
2026/07/19 16:12:08 [db] applying migration V32: mcp_servers_headers
2026/07/19 16:12:08 [db] migration V32 applied successfully
2026/07/19 16:12:08 [db] applying migration V33: file_library_foundation
2026/07/19 16:12:08 [db] migration V33 applied successfully
2026/07/19 16:12:08 [db] applying migration V34: workspace_project_context
2026/07/19 16:12:08 [db] migration V34 applied successfully
2026/07/19 16:12:08 [db] applying migration V35: browser_sessions_and_flag
2026/07/19 16:12:08 [db] migration V35 applied successfully
2026/07/19 16:12:08 [db] applying migration V36: music_studio
2026/07/19 16:12:08 [db] migration V36 applied successfully
2026/07/19 16:12:08 [db] applying migration V37: video_studio_foundation
2026/07/19 16:12:08 [db] migration V37 applied successfully
2026/07/19 16:12:08 [db] applying migration V38: video_studio_timelines
2026/07/19 16:12:08 [db] migration V38 applied successfully
2026/07/19 16:12:08 [db] applying migration V39: video_studio_render_jobs
2026/07/19 16:12:08 [db] migration V39 applied successfully
2026/07/19 16:12:08 [db] applying migration V40: video_generation_input_assets
2026/07/19 16:12:08 [db] migration V40 applied successfully
2026/07/19 16:12:08 [db] applying migration V41: video_render_job_metadata
2026/07/19 16:12:09 [db] migration V41 applied successfully
2026/07/19 16:12:09 [db] applying migration V42: agent_runtime
2026/07/19 16:12:09 [db] migration V42 applied successfully
2026/07/19 16:12:09 [websearch] no search API key configured; using DuckDuckGo (zero-config)
2026/07/19 16:12:09 [websearch] Jina Reader enabled (maxLen=3000 chars per page)
2026/07/19 16:12:09 [analytics] seeded 80 new default pricing rules
2026/07/19 16:12:09 [templates] seeding 5 built-in templates
2026/07/19 16:12:09 init scheduled agent tasks: recover scheduled tasks: SQL logic error: no such table: scheduled_tasks (1)
FAIL	github.com/ajbergh/omnillm-studio/internal/api	0.267s
?   	github.com/ajbergh/omnillm-studio/internal/apps	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/artifacts	0.010s
?   	github.com/ajbergh/omnillm-studio/internal/auth	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/browser	0.004s
ok  	github.com/ajbergh/omnillm-studio/internal/bundle	0.141s
?   	github.com/ajbergh/omnillm-studio/internal/config	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/crypto	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/db	0.067s
ok  	github.com/ajbergh/omnillm-studio/internal/document	0.005s
?   	github.com/ajbergh/omnillm-studio/internal/eval	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/filelibrary	0.007s
?   	github.com/ajbergh/omnillm-studio/internal/jobs	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/llm	0.003s
ok  	github.com/ajbergh/omnillm-studio/internal/mcpclient	0.019s
?   	github.com/ajbergh/omnillm-studio/internal/memory	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/models	0.002s
?   	github.com/ajbergh/omnillm-studio/internal/music	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/plugins	0.371s
ok  	github.com/ajbergh/omnillm-studio/internal/rag	0.461s
ok  	github.com/ajbergh/omnillm-studio/internal/repository	0.683s
ok  	github.com/ajbergh/omnillm-studio/internal/router	0.006s
?   	github.com/ajbergh/omnillm-studio/internal/search	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/sports	0.181s
?   	github.com/ajbergh/omnillm-studio/internal/tasks	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/tasktools	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/templates	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/tools	0.008s
ok  	github.com/ajbergh/omnillm-studio/internal/urlcontext	0.005s
ok  	github.com/ajbergh/omnillm-studio/internal/video	0.097s
ok  	github.com/ajbergh/omnillm-studio/internal/websearch	0.005s
ok  	github.com/ajbergh/omnillm-studio/internal/wordgen	0.007s
FAIL
```
