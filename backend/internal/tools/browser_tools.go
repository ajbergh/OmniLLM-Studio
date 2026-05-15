package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/browser"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

const headlessBrowserFlag = "headless_browser"

type browserFeatureFlags interface {
	IsEnabled(key string) bool
}

type browserToolBase struct {
	mgr         *browser.Manager
	featureRepo browserFeatureFlags
}

func newBrowserToolBase(mgr *browser.Manager, featureRepo *repository.FeatureFlagRepo) browserToolBase {
	return browserToolBase{mgr: mgr, featureRepo: featureRepo}
}

func (b browserToolBase) enabled() bool {
	return b.mgr != nil && (b.featureRepo == nil || b.featureRepo.IsEnabled(headlessBrowserFlag))
}

func (b browserToolBase) ensureEnabledResult() *ToolResult {
	if b.enabled() {
		return nil
	}
	return &ToolResult{
		Content: "Headless browser is not enabled. An administrator can enable it in Settings -> Features.",
		IsError: true,
		Metadata: map[string]interface{}{
			"error_type": "feature_disabled",
		},
	}
}

func browserErrorResult(err error) *ToolResult {
	if errors.Is(err, browser.ErrBotProtection) {
		return &ToolResult{
			Content: "The page returned a bot-protection or verification challenge instead of readable content.",
			IsError: true,
			Metadata: map[string]interface{}{
				"error_type": "bot_protection",
			},
		}
	}
	return &ToolResult{
		Content: err.Error(),
		IsError: true,
	}
}

// BrowserNavigateTool navigates to a URL with JavaScript enabled and extracts content.
type BrowserNavigateTool struct {
	browserToolBase
}

func NewBrowserNavigateTool(mgr *browser.Manager, featureRepo *repository.FeatureFlagRepo) *BrowserNavigateTool {
	return &BrowserNavigateTool{browserToolBase: newBrowserToolBase(mgr, featureRepo)}
}

type browserNavigateArgs struct {
	URL       string `json:"url"`
	SessionID string `json:"session_id,omitempty"`
	WaitFor   string `json:"wait_for,omitempty"`
	Extract   string `json:"extract,omitempty"`
}

func (t *BrowserNavigateTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "The URL to navigate to."},
			"session_id": {"type": "string", "description": "Optional persistent browser session ID. Omit for one-shot reads."},
			"wait_for": {"type": "string", "description": "Optional CSS selector to wait for before extracting content."},
			"extract": {"type": "string", "enum": ["text", "html", "both"], "default": "text"}
		},
		"required": ["url"]
	}`)
	return ToolDefinition{
		Name:        "browser_navigate",
		Description: "Navigate to a URL using a full headless browser with JavaScript execution and extract page text. Use this after web_search to read full article content, for JavaScript-rendered SPAs where fetch_url_context is empty, or for dynamic pages. Do not use for GitHub repos, simple static HTML pages, or URLs already supplied by the user and fetched into URL context. For research tasks, read 2-4 pages maximum per conversation turn.",
		Parameters:  schema,
		Category:    "browser",
		Enabled:     t.enabled(),
	}
}

func (t *BrowserNavigateTool) Validate(args json.RawMessage) error {
	var a browserNavigateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(a.URL) == "" {
		return fmt.Errorf("url is required")
	}
	switch strings.ToLower(strings.TrimSpace(a.Extract)) {
	case "", "text", "html", "both":
		return nil
	default:
		return fmt.Errorf("extract must be text, html, or both")
	}
}

func (t *BrowserNavigateTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if res := t.ensureEnabledResult(); res != nil {
		return res, nil
	}
	var a browserNavigateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	res, err := t.mgr.NavigatePage(ctx, browser.NavigateOptions{
		URL:       a.URL,
		SessionID: a.SessionID,
		UserID:    auth.UserIDFromContext(ctx),
		WaitFor:   a.WaitFor,
		Extract:   a.Extract,
		MaxChars:  browserMaxChars(ctx),
	})
	if err != nil {
		return browserErrorResult(err), nil
	}
	payload, _ := json.Marshal(res)
	return &ToolResult{
		Content: string(payload),
		Metadata: map[string]interface{}{
			"url":        res.URL,
			"title":      res.Title,
			"session_id": res.SessionID,
			"char_count": res.CharCount,
		},
	}, nil
}

func browserMaxChars(ctx context.Context) int {
	if strings.EqualFold(browser.ProviderTypeFromContext(ctx), "ollama") {
		return 10000
	}
	return browser.DefaultMaxExtractChars
}

// BrowserScreenshotTool captures a screenshot from a URL or browser session.
type BrowserScreenshotTool struct {
	browserToolBase
}

func NewBrowserScreenshotTool(mgr *browser.Manager, featureRepo *repository.FeatureFlagRepo) *BrowserScreenshotTool {
	return &BrowserScreenshotTool{browserToolBase: newBrowserToolBase(mgr, featureRepo)}
}

type browserScreenshotArgs struct {
	URL       string `json:"url,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	FullPage  bool   `json:"full_page,omitempty"`
	Selector  string `json:"selector,omitempty"`
}

func (t *BrowserScreenshotTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string"},
			"session_id": {"type": "string"},
			"full_page": {"type": "boolean", "default": false},
			"selector": {"type": "string", "description": "Optional CSS selector for element-only screenshots."}
		}
	}`)
	return ToolDefinition{
		Name:        "browser_screenshot",
		Description: "Take a screenshot of a web page or specific page element. Use when the user explicitly asks to see what a page looks like or visual layout matters more than text. Returns a PNG image rendered inline in chat.",
		Parameters:  schema,
		Category:    "browser",
		Enabled:     t.enabled(),
	}
}

func (t *BrowserScreenshotTool) Validate(args json.RawMessage) error {
	var a browserScreenshotArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(a.URL) == "" && strings.TrimSpace(a.SessionID) == "" {
		return fmt.Errorf("url or session_id is required")
	}
	return nil
}

func (t *BrowserScreenshotTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if res := t.ensureEnabledResult(); res != nil {
		return res, nil
	}
	var a browserScreenshotArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	png, finalURL, sessionID, err := t.mgr.Screenshot(ctx, browser.ScreenshotOptions{
		URL:       a.URL,
		SessionID: a.SessionID,
		UserID:    auth.UserIDFromContext(ctx),
		FullPage:  a.FullPage,
		Selector:  a.Selector,
	})
	if err != nil {
		return browserErrorResult(err), nil
	}
	encoded := base64.StdEncoding.EncodeToString(png)
	payload := map[string]interface{}{
		"url":        finalURL,
		"session_id": sessionID,
		"bytes":      len(png),
		"status":     "screenshot captured",
	}
	content, _ := json.Marshal(payload)
	return &ToolResult{
		Content: string(content),
		Metadata: map[string]interface{}{
			"screenshot_base64": encoded,
			"url":               finalURL,
			"session_id":        sessionID,
			"bytes":             len(png),
		},
	}, nil
}

// BrowserInteractTool performs an action on a persistent browser session.
type BrowserInteractTool struct {
	browserToolBase
}

func NewBrowserInteractTool(mgr *browser.Manager, featureRepo *repository.FeatureFlagRepo) *BrowserInteractTool {
	return &BrowserInteractTool{browserToolBase: newBrowserToolBase(mgr, featureRepo)}
}

type browserInteractArgs struct {
	SessionID string `json:"session_id"`
	Action    string `json:"action"`
	Selector  string `json:"selector,omitempty"`
	Value     string `json:"value,omitempty"`
}

func (t *BrowserInteractTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {"type": "string"},
			"action": {"type": "string", "enum": ["click", "type", "select", "scroll", "hover"]},
			"selector": {"type": "string"},
			"value": {"type": "string"}
		},
		"required": ["session_id", "action"]
	}`)
	return ToolDefinition{
		Name:        "browser_interact",
		Description: "Interact with an element on a page in an active browser session. Supports click, type, select, scroll, and hover. Create a session first with browser_session, navigate with browser_navigate using that session_id, then perform actions.",
		Parameters:  schema,
		Category:    "browser",
		Enabled:     t.enabled(),
	}
}

func (t *BrowserInteractTool) Validate(args json.RawMessage) error {
	var a browserInteractArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(a.SessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "click", "type", "select", "scroll", "hover":
		return nil
	default:
		return fmt.Errorf("unsupported action")
	}
}

func (t *BrowserInteractTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if res := t.ensureEnabledResult(); res != nil {
		return res, nil
	}
	var a browserInteractArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	currentURL, title, err := t.mgr.Interact(ctx, auth.UserIDFromContext(ctx), a.SessionID, a.Action, a.Selector, a.Value)
	if err != nil {
		return browserErrorResult(err), nil
	}
	payload := map[string]interface{}{
		"success":     true,
		"current_url": currentURL,
		"page_title":  title,
	}
	content, _ := json.Marshal(payload)
	return &ToolResult{Content: string(content), Metadata: payload}, nil
}

// BrowserPDFTool renders a page to PDF bytes.
type BrowserPDFTool struct {
	browserToolBase
}

func NewBrowserPDFTool(mgr *browser.Manager, featureRepo *repository.FeatureFlagRepo) *BrowserPDFTool {
	return &BrowserPDFTool{browserToolBase: newBrowserToolBase(mgr, featureRepo)}
}

type browserPDFArgs struct {
	URL       string `json:"url,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func (t *BrowserPDFTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string"},
			"session_id": {"type": "string"}
		}
	}`)
	return ToolDefinition{
		Name:        "browser_pdf",
		Description: "Render a web page as a PDF. Use when the user wants to save or archive a page.",
		Parameters:  schema,
		Category:    "browser",
		Enabled:     t.enabled(),
	}
}

func (t *BrowserPDFTool) Validate(args json.RawMessage) error {
	var a browserPDFArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(a.URL) == "" && strings.TrimSpace(a.SessionID) == "" {
		return fmt.Errorf("url or session_id is required")
	}
	return nil
}

func (t *BrowserPDFTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if res := t.ensureEnabledResult(); res != nil {
		return res, nil
	}
	var a browserPDFArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	pdf, finalURL, sessionID, err := t.mgr.PDFSnapshot(ctx, browser.PDFOptions{
		URL:       a.URL,
		SessionID: a.SessionID,
		UserID:    auth.UserIDFromContext(ctx),
	})
	if err != nil {
		return browserErrorResult(err), nil
	}
	encoded := base64.StdEncoding.EncodeToString(pdf)
	filename := browserPDFFilename(finalURL)
	payload := map[string]interface{}{
		"url":        finalURL,
		"session_id": sessionID,
		"bytes":      len(pdf),
		"filename":   filename,
		"status":     "pdf captured",
	}
	content, _ := json.Marshal(payload)
	metadata := map[string]interface{}{
		"pdf_base64": encoded,
		"url":        finalURL,
		"session_id": sessionID,
		"bytes":      len(pdf),
		"filename":   filename,
	}
	return &ToolResult{Content: string(content), Metadata: metadata}, nil
}

func browserPDFFilename(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return "browser-page.pdf"
	}
	host := strings.NewReplacer(".", "-", ":", "-").Replace(parsed.Hostname())
	return fmt.Sprintf("%s.pdf", host)
}

// BrowserSessionTool manages persistent browser sessions.
type BrowserSessionTool struct {
	browserToolBase
}

func NewBrowserSessionTool(mgr *browser.Manager, featureRepo *repository.FeatureFlagRepo) *BrowserSessionTool {
	return &BrowserSessionTool{browserToolBase: newBrowserToolBase(mgr, featureRepo)}
}

type browserSessionArgs struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id,omitempty"`
}

func (t *BrowserSessionTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["create", "close", "list", "status"]},
			"session_id": {"type": "string"}
		},
		"required": ["action"]
	}`)
	return ToolDefinition{
		Name:        "browser_session",
		Description: "Manage persistent browser sessions for multi-step navigation. Create a session when you need to maintain state across multiple browser interactions. For one-time page reads, call browser_navigate without a session_id.",
		Parameters:  schema,
		Category:    "browser",
		Enabled:     t.enabled(),
	}
}

func (t *BrowserSessionTool) Validate(args json.RawMessage) error {
	var a browserSessionArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create", "list":
		return nil
	case "close", "status":
		if strings.TrimSpace(a.SessionID) == "" {
			return fmt.Errorf("session_id is required for %s", a.Action)
		}
		return nil
	default:
		return fmt.Errorf("unsupported browser session action")
	}
}

func (t *BrowserSessionTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if res := t.ensureEnabledResult(); res != nil {
		return res, nil
	}
	var a browserSessionArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	userID := auth.UserIDFromContext(ctx)
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create":
		session, err := t.mgr.CreateSession(ctx, userID)
		if err != nil {
			return browserErrorResult(err), nil
		}
		content, _ := json.Marshal(map[string]interface{}{"session_id": session.ID, "session": session})
		return &ToolResult{Content: string(content), Metadata: map[string]interface{}{"session_id": session.ID}}, nil
	case "close":
		if err := t.mgr.CloseSession(ctx, userID, a.SessionID); err != nil {
			return browserErrorResult(err), nil
		}
		content, _ := json.Marshal(map[string]interface{}{"closed": true, "session_id": a.SessionID})
		return &ToolResult{Content: string(content), Metadata: map[string]interface{}{"closed": true, "session_id": a.SessionID}}, nil
	case "list":
		sessions := t.mgr.ListSessions(userID)
		content, _ := json.Marshal(map[string]interface{}{"sessions": sessions})
		return &ToolResult{Content: string(content), Metadata: map[string]interface{}{"count": len(sessions)}}, nil
	case "status":
		for _, session := range t.mgr.ListSessions(userID) {
			if session.ID == a.SessionID {
				content, _ := json.Marshal(session)
				return &ToolResult{Content: string(content), Metadata: map[string]interface{}{"session_id": session.ID, "current_url": session.CurrentURL}}, nil
			}
		}
		return &ToolResult{Content: "browser session not found", IsError: true}, nil
	default:
		return &ToolResult{Content: "unsupported browser session action", IsError: true}, nil
	}
}
