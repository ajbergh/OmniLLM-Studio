// Package api provides HTTP handlers and routing for the OmniLLM-Studio backend.
// It maps REST endpoints to underlying domain services and repositories.
package api

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"os"
	"path/filepath"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/agent"
	"github.com/ajbergh/omnillm-studio/internal/analytics"
	"github.com/ajbergh/omnillm-studio/internal/artifacts"
	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/bundle"
	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/ajbergh/omnillm-studio/internal/filelibrary"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/mcpclient"
	"github.com/ajbergh/omnillm-studio/internal/news"
	"github.com/ajbergh/omnillm-studio/internal/plugins"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/search"
	"github.com/ajbergh/omnillm-studio/internal/sports"
	"github.com/ajbergh/omnillm-studio/internal/templates"
	"github.com/ajbergh/omnillm-studio/internal/tools"
	"github.com/ajbergh/omnillm-studio/internal/urlcontext"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
	"github.com/ajbergh/omnillm-studio/internal/wordgen"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// SecurityHeaders adds standard security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// NewRouter creates the main HTTP router with all API routes.
func NewRouter(database *sql.DB, cfg *config.Config, version, commit string) http.Handler {
	handler, _ := NewRouterWithShutdown(database, cfg, version, commit)
	return handler
}

// NewRouterWithShutdown creates the main HTTP router and returns a cleanup hook
// for runtime services owned by the API layer.
func NewRouterWithShutdown(database *sql.DB, cfg *config.Config, version, commit string) (http.Handler, func(context.Context) error) {
	r := chi.NewRouter()
	shutdownFns := []func(context.Context) error{}

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(SecurityHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Repos
	convoRepo := repository.NewConversationRepo(database)
	msgRepo := repository.NewMessageRepo(database)
	providerRepo := repository.NewProviderRepo(database)
	settingsRepo := repository.NewSettingsRepo(database)
	attachRepo := repository.NewAttachmentRepo(database)
	featureFlagRepo := repository.NewFeatureFlagRepo(database)
	chunkRepo := repository.NewChunkRepo(database)
	embeddingRepo := repository.NewEmbeddingRepo(database) // legacy SQL embeddings, used only for lazy migration
	libraryFileRepo := repository.NewLibraryFileRepo(database)

	// Services
	llmService := llm.NewService(providerRepo, settingsRepo)

	// RAG vector store (chromem-go) + retriever
	vectorStore, err := rag.NewVectorStore(cfg.ChromemDir, cfg.ChromemCompress)
	if err != nil {
		log.Fatalf("init chromem vector store at %s: %v", cfg.ChromemDir, err)
	}
	ragRetriever := rag.NewChromemRetriever(llmService, chunkRepo, vectorStore).
		WithLegacyEmbeddingRepo(embeddingRepo)
	fileLibrarySvc := filelibrary.NewService(
		libraryFileRepo,
		attachRepo,
		chunkRepo,
		settingsRepo,
		providerRepo,
		convoRepo,
		llmService,
		vectorStore,
		cfg.AttachmentsDir,
	)

	// Web search orchestrator (auto-selects provider from settings:
	//   - Brave Search if brave_api_key is set
	//   - DuckDuckGo otherwise (zero-config, no API key needed)
	//   - Set web_search_provider = "none" to disable)
	wsProvider := websearch.NewProviderFromSettings(settingsRepo)
	jinaReader := websearch.NewJinaReaderFromSettings(settingsRepo)
	orchestrator := websearch.NewOrchestrator(wsProvider, llmService, jinaReader)

	// News Lookup Service (Actually Relevant API, no key required)
	newsCfg := news.LoadConfigFromEnv()
	newsSvc := news.NewService(newsCfg)

	// URL Context Resolver
	urlCtxCfg := urlcontext.ConfigFromEnv()
	urlCtxSvc := urlcontext.NewService(urlCtxCfg)

	wordGen := wordgen.NewGenerator(cfg.AttachmentsDir)
	artifactGen := artifacts.NewGenerator(cfg.AttachmentsDir)

	// Tool Calling Framework
	toolPermRepo := repository.NewToolPermissionRepo(database)
	toolRegistry := tools.NewRegistry()
	toolRegistry.MustRegister(tools.NewWebSearchTool(orchestrator, "", ""))
	toolRegistry.MustRegister(tools.NewCalculatorTool())
	toolRegistry.MustRegister(tools.NewURLFetchTool())
	toolRegistry.MustRegister(tools.NewWordDocTool(wordGen, attachRepo))
	toolRegistry.MustRegister(sports.NewSportsLookupTool(sports.NewESPNClient()))
	toolRegistry.MustRegister(tools.NewFetchURLContextTool(urlCtxSvc))
	toolRegistry.MustRegister(tools.NewGitHubRepoInspectTool(urlCtxSvc))
	toolRegistry.MustRegister(tools.NewFileSearchTool(fileLibrarySvc, "", "", ""))
	toolRegistry.MustRegister(tools.NewFileFetchTool(fileLibrarySvc, ""))
	toolRegistry.MustRegister(tools.NewFileIngestTool(fileLibrarySvc, "", "", ""))
	toolRegistry.MustRegister(tools.NewFileSummarizeTool(fileLibrarySvc, ""))
	toolRegistry.MustRegister(tools.NewFileCompareTool(fileLibrarySvc, ""))
	toolRegistry.MustRegister(tools.NewFileDeleteTool(fileLibrarySvc, ""))
	toolRegistry.MustRegister(tools.NewFileReindexTool(fileLibrarySvc, ""))
	toolExecutor := tools.NewExecutor(toolRegistry, toolPermRepo.PolicyResolver(), 0)
	toolHandler := NewToolHandler(toolRegistry, toolExecutor, toolPermRepo)

	// Handlers
	convoHandler := NewConversationHandler(convoRepo, vectorStore)
	msgHandler := NewMessageHandler(msgRepo, convoRepo, attachRepo, cfg.AttachmentsDir, llmService, orchestrator, ragRetriever, settingsRepo, providerRepo, chunkRepo, vectorStore, wordGen, artifactGen, featureFlagRepo, newsSvc, urlCtxSvc, toolRegistry, toolExecutor, fileLibrarySvc)
	providerHandler := NewProviderHandler(providerRepo)
	settingsHandler := NewSettingsHandler(settingsRepo, orchestrator)
	wsHandler := NewWebSearchHandler(orchestrator)
	titleHandler := NewTitleHandler(convoRepo, msgRepo, llmService)
	attachHandler := NewAttachmentHandler(attachRepo, convoRepo, cfg.AttachmentsDir)
	imageHandler := NewImageHandler(msgRepo, convoRepo, attachRepo, llmService, cfg.AttachmentsDir)
	featureFlagHandler := NewFeatureFlagHandler(featureFlagRepo)
	ragHandler := NewRAGHandler(chunkRepo, vectorStore, attachRepo, convoRepo, settingsRepo, providerRepo, llmService, cfg.AttachmentsDir)
	fileLibraryHandler := NewFileLibraryHandler(fileLibrarySvc, convoRepo)

	// Model Context Protocol
	mcpRepo := repository.NewMCPServerRepo(database)
	mcpManager := mcpclient.NewManager(mcpRepo, toolPermRepo, toolRegistry)
	mcpHandler := NewMCPHandler(mcpRepo, mcpManager)
	mcpManager.StartEnabled(context.Background())
	shutdownFns = append(shutdownFns, mcpManager.StopAll)

	// Usage & Cost Analytics
	pricingRepo := repository.NewPricingRepo(database)
	analytics.SeedDefaults(pricingRepo)
	analyticsSvc := analytics.NewService(database, pricingRepo)
	analyticsHandler := NewAnalyticsHandler(analyticsSvc, pricingRepo, convoRepo)

	// Prompt Templates
	templateRepo := repository.NewTemplateRepo(database)
	templates.SeedDefaults(templateRepo)
	templateHandler := NewTemplateHandler(templateRepo)

	// Import/Export
	bundleExporter := bundle.NewExporter(database, convoRepo, msgRepo, attachRepo, providerRepo, settingsRepo, vectorStore, cfg.AttachmentsDir, version)
	bundleImporter := bundle.NewImporter(database, cfg.AttachmentsDir, vectorStore)
	bundleHandler := NewBundleHandler(bundleExporter, bundleImporter)

	// Agent Mode
	agentRunRepo := repository.NewAgentRunRepo(database)
	agentStepRepo := repository.NewAgentStepRepo(database)
	agentPlanner := agent.NewPlanner(llmService, toolRegistry)
	agentRunner := agent.NewRunner(agentPlanner, llmService, toolExecutor, agentRunRepo, agentStepRepo, msgRepo)
	agentHandler := NewAgentHandler(agentRunner, agentRunRepo, agentStepRepo, msgRepo, convoRepo)

	// Conversation Branching
	branchRepo := repository.NewBranchRepo(database)
	branchHandler := NewBranchHandler(branchRepo, msgRepo, convoRepo)

	// Image Edit Sessions
	imgSessionRepo := repository.NewImageSessionRepo(database)
	imgNodeRepo := repository.NewImageNodeRepo(database)
	imgAssetRepo := repository.NewImageNodeAssetRepo(database)
	imgMaskRepo := repository.NewImageMaskRepo(database)
	imgRefRepo := repository.NewImageReferenceRepo(database)
	imageSessionHandler := NewImageSessionHandler(
		imgSessionRepo, imgNodeRepo, imgAssetRepo, imgMaskRepo, imgRefRepo,
		attachRepo, convoRepo, llmService, cfg.AttachmentsDir,
	)

	// Semantic Search
	msgEmbeddingRepo := repository.NewMessageEmbeddingRepo(database)
	searchService := search.NewService(database, llmService, msgRepo, msgEmbeddingRepo, settingsRepo)
	searchHandler := NewSearchHandler(searchService, convoRepo)

	// Workspaces
	workspaceRepo := repository.NewWorkspaceRepo(database)
	workspaceHandler := NewWorkspaceHandler(workspaceRepo)

	// Users & Auth (Local Collaboration)
	userRepo := repository.NewUserRepo(database)
	sessionRepo := repository.NewSessionRepo(database)
	authHandler := NewAuthHandler(userRepo, sessionRepo, cfg)
	wsMemberRepo := repository.NewWorkspaceMemberRepo(database)
	wsMemberHandler := NewWorkspaceMemberHandler(wsMemberRepo, userRepo)

	// Auth middleware: bypasses auth in solo mode (no users), enforces sessions in multi-user mode
	authMiddleware := auth.Middleware(userRepo, sessionRepo, userRepo.GetByID, cfg.BindAddress)

	// Plugin SDK
	pluginRepo := repository.NewPluginRepo(database)
	pluginDir := filepath.Join(os.Getenv("HOME"), ".omnillm-studio", "plugins")
	if d := os.Getenv("OMNILLM_PLUGIN_DIR"); d != "" {
		pluginDir = d
	}
	pluginLoader := plugins.NewLoader(pluginDir, pluginRepo)
	pluginHandler := NewPluginHandler(pluginRepo, pluginLoader, pluginDir)

	// Evaluation Harness
	evalRunRepo := repository.NewEvalRunRepo(database)
	evalHandler := NewEvalHandler(evalRunRepo, llmService)

	// Routes
	r.Route("/v1", func(r chi.Router) {
		// Health (public)
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
		})
		r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"version": version,
			})
		})

		// Auth (public — no auth middleware, rate-limited)
		authLimiter := newRateLimiter(1*time.Minute, 10)
		r.Route("/auth", func(r chi.Router) {
			r.Use(RateLimit(authLimiter))
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/logout", authHandler.Logout)
			r.Get("/status", authHandler.AuthStatus)
		})

		// --- All routes below require auth (in multi-user mode) ---
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			// Current user
			r.Get("/users/me", authHandler.Me)

			// Conversations
			r.Route("/conversations", func(r chi.Router) {
				r.Get("/", convoHandler.List)
				r.Post("/", convoHandler.Create)
				r.Get("/search", convoHandler.Search)

				r.Route("/{conversationId}", func(r chi.Router) {
					r.Get("/", convoHandler.Get)
					r.Patch("/", convoHandler.Update)
					r.Delete("/", convoHandler.Delete)

					// Auto-title generation
					r.Post("/title", titleHandler.Generate)

					// Messages
					r.Get("/messages", msgHandler.List)
					r.Post("/messages", msgHandler.Create)
					r.Post("/messages/stream", msgHandler.Stream)
					r.Post("/messages/image", imageHandler.Generate)

					// Message operations
					r.Delete("/messages/{messageId}", msgHandler.DeleteMessage)
					r.Patch("/messages/{messageId}", msgHandler.EditMessage)

					// Attachments (scoped to conversation)
					r.Get("/attachments", attachHandler.List)
					r.Post("/attachments", attachHandler.Upload)

					// RAG
					r.Get("/chunks", ragHandler.ListChunks)
					r.Post("/reindex", ragHandler.Reindex)

					// File Library (conversation shortcuts)
					r.Get("/file-library/files", fileLibraryHandler.ConversationListFiles)
					r.Post("/file-library/search", fileLibraryHandler.ConversationSearch)
					r.Post("/file-library/ingest-attachments", fileLibraryHandler.ConversationIngestAttachment)

					// Agent Mode
					r.Post("/agent/run", agentHandler.StartRun)
					r.Get("/agent/runs", agentHandler.ListRuns)

					// Branching
					r.Get("/branches", branchHandler.ListBranches)
					r.Post("/branches", branchHandler.CreateBranch)
					r.Route("/branches/{branchId}", func(r chi.Router) {
						r.Delete("/", branchHandler.DeleteBranch)
						r.Patch("/", branchHandler.RenameBranch)
					})
					r.Get("/messages/branch", branchHandler.ListBranchMessages)

					// Image Edit Sessions
					r.Route("/images/sessions", func(r chi.Router) {
						r.Post("/", imageSessionHandler.CreateSession)
						r.Get("/", imageSessionHandler.ListSessions)
						r.Route("/{sessionId}", func(r chi.Router) {
							r.Get("/", imageSessionHandler.GetSession)
							r.Patch("/", imageSessionHandler.RenameSession)
							r.Delete("/", imageSessionHandler.DeleteSession)
							r.Post("/generate", imageSessionHandler.Generate)
							r.Post("/edit", imageSessionHandler.Edit)
							r.Post("/mask", imageSessionHandler.UploadMask)
							r.Get("/assets", imageSessionHandler.GetAssets)
							r.Delete("/assets/{assetId}", imageSessionHandler.DeleteAsset)
							r.Put("/nodes/{nodeId}/select", imageSessionHandler.SelectVariant)
						})
					})
				})
			})

			// Global Image Sessions (all sessions across conversations)
			r.Post("/images/sessions", imageSessionHandler.CreateStandaloneSession)
			r.Get("/images/sessions", imageSessionHandler.ListAllSessions)

			// Attachment operations (global, by attachment ID)
			r.Route("/attachments/{attachmentId}", func(r chi.Router) {
				r.Get("/download", attachHandler.Download)
				r.Delete("/", attachHandler.Delete)
				r.Get("/chunks", ragHandler.ListAttachmentChunks)
				r.Post("/index", ragHandler.IndexAttachment)
			})

			// File Library
			r.Route("/file-library", func(r chi.Router) {
				r.Get("/files", fileLibraryHandler.ListFiles)
				r.Post("/files/ingest", fileLibraryHandler.IngestFile)
				r.Get("/files/{fileId}", fileLibraryHandler.Fetch)
				r.Patch("/files/{fileId}", fileLibraryHandler.Update)
				r.Delete("/files/{fileId}", fileLibraryHandler.Delete)
				r.Post("/files/{fileId}/reindex", fileLibraryHandler.Reindex)
				r.Post("/search", fileLibraryHandler.Search)
				r.Post("/summarize", fileLibraryHandler.Summarize)
				r.Post("/compare", fileLibraryHandler.Compare)
			})

			// Provider Profiles
			r.Route("/providers", func(r chi.Router) {
				r.Get("/", providerHandler.List)
				r.Get("/ollama/models", providerHandler.FetchOllamaModels)
				r.Get("/openrouter/models", providerHandler.FetchOpenRouterModels)
				r.Get("/{providerId}/image-capabilities", providerHandler.GetImageCapabilities)
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole("admin"))
					r.Post("/", providerHandler.Create)
					r.Route("/{providerId}", func(r chi.Router) {
						r.Patch("/", providerHandler.Update)
						r.Delete("/", providerHandler.Delete)
					})
				})
			})

			// Web Search
			r.Post("/websearch", wsHandler.Search)

			// Settings (read: any user; write: admin only)
			r.Get("/settings", settingsHandler.GetAll)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Patch("/settings", settingsHandler.Update)
			})

			// Feature Flags (admin only)
			r.Get("/features", featureFlagHandler.List)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Patch("/features/{key}", featureFlagHandler.Update)
			})

			// Tools
			r.Route("/tools", func(r chi.Router) {
				r.Get("/", toolHandler.ListTools)
				r.Post("/execute", toolHandler.ExecuteTool)
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole("admin"))
					r.Patch("/{toolName}/permission", toolHandler.UpdatePermission)
				})
			})

			// MCP Servers (admin only)
			r.Route("/mcp", func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Get("/audit", mcpHandler.ListAudit)
				r.Route("/servers", func(r chi.Router) {
					r.Get("/", mcpHandler.ListServers)
					r.Post("/", mcpHandler.CreateServer)
					r.Route("/{serverId}", func(r chi.Router) {
						r.Get("/", mcpHandler.GetServer)
						r.Patch("/", mcpHandler.UpdateServer)
						r.Delete("/", mcpHandler.DeleteServer)
						r.Post("/test", mcpHandler.TestServer)
						r.Post("/start", mcpHandler.StartServer)
						r.Post("/stop", mcpHandler.StopServer)
						r.Post("/restart", mcpHandler.RestartServer)
						r.Post("/refresh", mcpHandler.RefreshTools)
						r.Get("/tools", mcpHandler.ListTools)
						r.Patch("/tools/{toolName}", mcpHandler.UpdateToolPolicy)
					})
				})
			})

			// Analytics
			r.Route("/analytics", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole("admin"))
					r.Get("/usage", analyticsHandler.GetUsage)
				})
				r.Get("/conversations/{conversationId}/usage", analyticsHandler.GetConversationUsage)
			})

			// Pricing (admin only)
			r.Route("/pricing", func(r chi.Router) {
				r.Get("/", analyticsHandler.ListPricing)
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole("admin"))
					r.Put("/", analyticsHandler.UpsertPricing)
					r.Delete("/{pricingId}", analyticsHandler.DeletePricing)
				})
			})

			// Prompt Templates (read: any user; write: admin only)
			r.Route("/templates", func(r chi.Router) {
				r.Get("/", templateHandler.ListTemplates)
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole("admin"))
					r.Post("/", templateHandler.CreateTemplate)
				})
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", templateHandler.GetTemplate)
					r.Post("/interpolate", templateHandler.InterpolateTemplate)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole("admin"))
						r.Patch("/", templateHandler.UpdateTemplate)
						r.Delete("/", templateHandler.DeleteTemplate)
					})
				})
			})

			// Import/Export (admin only)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Post("/export", bundleHandler.Export)
				r.Post("/import", bundleHandler.Import)
				r.Post("/import/validate", bundleHandler.ValidateImport)
			})

			// Agent Runs (by run ID)
			r.Route("/agent/runs/{runId}", func(r chi.Router) {
				r.Get("/", agentHandler.GetRun)
				r.Post("/approve/{stepId}", agentHandler.ApproveStep)
				r.Post("/cancel", agentHandler.CancelRun)
				r.Post("/resume", agentHandler.ResumeRun)
			})

			// RAG (global / admin)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Post("/rag/reindex-all", ragHandler.ReindexAll)
			})

			// Search
			r.Get("/search", searchHandler.Search)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Post("/search/reindex", searchHandler.Reindex)
			})

			// Workspaces (admin only)
			r.Route("/workspaces", func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Get("/", workspaceHandler.List)
				r.Post("/", workspaceHandler.Create)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", workspaceHandler.Get)
					r.Get("/stats", workspaceHandler.GetStats)
					r.Patch("/", workspaceHandler.Update)
					r.Delete("/", workspaceHandler.Delete)

					// Workspace members
					r.Get("/members", wsMemberHandler.ListMembers)
					r.Post("/members", wsMemberHandler.AddMember)
					r.Patch("/members/{userId}", wsMemberHandler.UpdateMemberRole)
					r.Delete("/members/{userId}", wsMemberHandler.RemoveMember)
				})
			})

			// Plugins (admin only)
			r.Route("/plugins", func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Get("/", pluginHandler.ListPlugins)
				r.Post("/", pluginHandler.InstallPlugin)
				r.Route("/{name}", func(r chi.Router) {
					r.Patch("/", pluginHandler.UpdatePlugin)
					r.Delete("/", pluginHandler.UninstallPlugin)
				})
			})

			// Evaluation Harness (admin only)
			r.Route("/eval", func(r chi.Router) {
				r.Use(auth.RequireRole("admin"))
				r.Post("/run", evalHandler.RunEval)
				r.Get("/runs", evalHandler.ListRuns)
				r.Route("/runs/{id}", func(r chi.Router) {
					r.Get("/", evalHandler.GetRun)
					r.Delete("/", evalHandler.DeleteRun)
				})
			})

		}) // end auth group
	})

	return r, func(ctx context.Context) error {
		var firstErr error
		for _, shutdown := range shutdownFns {
			if err := shutdown(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
}
