package browser

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/google/uuid"
)

// NavigateOptions configures a browser navigation.
type NavigateOptions struct {
	URL       string
	SessionID string
	UserID    string
	WaitFor   string
	Extract   string
	MaxChars  int
}

// NavigateResult is returned after browser navigation and extraction.
type NavigateResult struct {
	Content   string `json:"content,omitempty"`
	HTML      string `json:"html,omitempty"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	CharCount int    `json:"char_count"`
}

// ScreenshotOptions configures browser screenshot capture.
type ScreenshotOptions struct {
	URL       string
	SessionID string
	UserID    string
	FullPage  bool
	Selector  string
}

// PDFOptions configures browser PDF capture.
type PDFOptions struct {
	URL       string
	SessionID string
	UserID    string
}

// Status describes the browser runtime for settings and diagnostics.
type Status struct {
	Enabled        bool   `json:"enabled"`
	CacheDir       string `json:"cache_dir"`
	ExecPath       string `json:"exec_path,omitempty"`
	ActiveSessions int    `json:"active_sessions"`
	BrowserRunning bool   `json:"browser_running"`
}

// Manager owns the shared browser process and live page sessions.
type Manager struct {
	cfg  *config.Config
	repo *repository.BrowserSessionRepo

	mu       sync.RWMutex
	browser  *rod.Browser
	launcher *launcher.Launcher
	sessions map[string]*Session

	initOnce sync.Once
	initErr  error
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewManager creates a browser manager. It does not launch Chromium.
func NewManager(cfg *config.Config, repo *repository.BrowserSessionRepo) *Manager {
	if cfg == nil {
		cfg = config.Load()
	}
	return &Manager{
		cfg:      cfg,
		repo:     repo,
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
	}
}

// Init purges stale persisted rows and starts idle-session eviction.
func (m *Manager) Init() error {
	m.initOnce.Do(func() {
		if m.repo != nil {
			if err := m.repo.DeleteAll(); err != nil {
				m.initErr = err
				return
			}
		}
		go m.evictLoop()
	})
	return m.initErr
}

// Navigate satisfies urlcontext.BrowserNavigator for one-shot URL reads.
func (m *Manager) Navigate(ctx context.Context, rawURL string) (string, string, error) {
	res, err := m.NavigatePage(ctx, NavigateOptions{
		URL:      rawURL,
		Extract:  "text",
		MaxChars: DefaultMaxExtractChars,
	})
	if err != nil {
		return "", "", err
	}
	return res.Content, res.Title, nil
}

// NavigatePage navigates using a persistent session when SessionID is set, or
// an ephemeral page that is closed immediately after extraction.
func (m *Manager) NavigatePage(ctx context.Context, opts NavigateOptions) (*NavigateResult, error) {
	if err := m.ensureEnabled(); err != nil {
		return nil, err
	}
	if err := validateURL(ctx, opts.URL); err != nil {
		return nil, err
	}

	page, session, ephemeral, err := m.pageForNavigation(ctx, opts.UserID, opts.SessionID)
	if err != nil {
		return nil, err
	}
	if ephemeral {
		defer page.Close()
	}

	emitProgress(ctx, "browser_navigating", map[string]any{
		"url":        opts.URL,
		"session_id": opts.SessionID,
	})

	if err := page.Navigate(opts.URL); err != nil {
		return nil, err
	}
	_ = page.WaitLoad()
	_ = page.WaitIdle(1500 * time.Millisecond)
	if opts.WaitFor != "" {
		if _, err := page.Element(opts.WaitFor); err != nil {
			return nil, fmt.Errorf("wait for selector %q: %w", opts.WaitFor, err)
		}
	}

	title := pageTitle(page, opts.URL)
	finalURL := pageURL(page, opts.URL)

	extract := strings.ToLower(strings.TrimSpace(opts.Extract))
	if extract == "" {
		extract = "text"
	}
	maxChars := opts.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultMaxExtractChars
	}

	var text, html string
	if extract == "text" || extract == "both" {
		text, err = ExtractText(page, maxChars)
		if err != nil {
			return nil, fmt.Errorf("extract text: %w", err)
		}
		if detectBotProtection(text) {
			return nil, ErrBotProtection
		}
	}
	if extract == "html" || extract == "both" {
		html, err = ExtractHTML(page, maxChars)
		if err != nil {
			return nil, fmt.Errorf("extract html: %w", err)
		}
	}

	if session != nil {
		m.touchSession(session, finalURL)
	}

	content := text
	if extract == "html" {
		content = html
	}
	if extract == "both" {
		content = text
		if content != "" && html != "" {
			content += "\n\n--- HTML ---\n\n" + html
		} else if html != "" {
			content = html
		}
	}

	return &NavigateResult{
		Content:   content,
		HTML:      html,
		URL:       finalURL,
		Title:     title,
		SessionID: opts.SessionID,
		CharCount: len(content),
	}, nil
}

// Screenshot captures a page or element screenshot.
func (m *Manager) Screenshot(ctx context.Context, opts ScreenshotOptions) ([]byte, string, string, error) {
	if err := m.ensureEnabled(); err != nil {
		return nil, "", "", err
	}
	if opts.SessionID == "" && opts.URL == "" {
		return nil, "", "", fmt.Errorf("url or session_id is required")
	}
	if opts.URL != "" {
		if err := validateURL(ctx, opts.URL); err != nil {
			return nil, "", "", err
		}
	}

	page, session, ephemeral, err := m.pageForNavigation(ctx, opts.UserID, opts.SessionID)
	if err != nil {
		return nil, "", "", err
	}
	if ephemeral {
		defer page.Close()
	}
	if opts.URL != "" {
		emitProgress(ctx, "browser_navigating", map[string]any{"url": opts.URL, "session_id": opts.SessionID})
		if err := page.Navigate(opts.URL); err != nil {
			return nil, "", "", err
		}
		_ = page.WaitLoad()
		_ = page.WaitIdle(1500 * time.Millisecond)
	}

	png, err := TakeScreenshot(page, opts.Selector, opts.FullPage)
	if err != nil {
		return nil, "", "", err
	}
	finalURL := pageURL(page, opts.URL)
	if session != nil {
		m.touchSession(session, finalURL)
	}
	emitProgress(ctx, "browser_screenshot_done", map[string]any{"url": finalURL, "session_id": opts.SessionID})
	return png, finalURL, opts.SessionID, nil
}

// Interact performs a UI action on a persistent session page.
func (m *Manager) Interact(ctx context.Context, userID, sessionID, action, selector, value string) (string, string, error) {
	if err := m.ensureEnabled(); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(sessionID) == "" {
		return "", "", fmt.Errorf("session_id is required")
	}
	if strings.TrimSpace(selector) == "" && strings.ToLower(action) != "scroll" {
		return "", "", fmt.Errorf("selector is required")
	}

	session, err := m.getSession(userID, sessionID)
	if err != nil {
		return "", "", err
	}
	page := session.Page

	el, err := page.Element(selector)
	if err != nil && strings.ToLower(action) != "scroll" {
		return "", "", err
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "click":
		if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return "", "", err
		}
	case "type":
		if err := el.Input(value); err != nil {
			return "", "", err
		}
	case "select":
		if err := el.Select([]string{value}, true, rod.SelectorTypeText); err != nil {
			return "", "", err
		}
	case "hover":
		if err := el.Hover(); err != nil {
			return "", "", err
		}
	case "scroll":
		if selector != "" {
			el, err := page.Element(selector)
			if err != nil {
				return "", "", err
			}
			if err := el.ScrollIntoView(); err != nil {
				return "", "", err
			}
		} else {
			if _, err := page.Eval(`() => window.scrollBy(0, window.innerHeight * 0.8)`); err != nil {
				return "", "", err
			}
		}
	default:
		return "", "", fmt.Errorf("unsupported browser action %q", action)
	}

	_ = page.WaitIdle(1500 * time.Millisecond)
	finalURL := pageURL(page, session.CurrentURL)
	title := pageTitle(page, finalURL)
	m.touchSession(session, finalURL)
	emitProgress(ctx, "browser_interact_done", map[string]any{
		"action":     action,
		"selector":   selector,
		"session_id": sessionID,
	})
	return finalURL, title, nil
}

// PDFSnapshot renders a page to PDF bytes.
func (m *Manager) PDFSnapshot(ctx context.Context, opts PDFOptions) ([]byte, string, string, error) {
	if err := m.ensureEnabled(); err != nil {
		return nil, "", "", err
	}
	if opts.SessionID == "" && opts.URL == "" {
		return nil, "", "", fmt.Errorf("url or session_id is required")
	}
	if opts.URL != "" {
		if err := validateURL(ctx, opts.URL); err != nil {
			return nil, "", "", err
		}
	}

	page, session, ephemeral, err := m.pageForNavigation(ctx, opts.UserID, opts.SessionID)
	if err != nil {
		return nil, "", "", err
	}
	if ephemeral {
		defer page.Close()
	}
	if opts.URL != "" {
		emitProgress(ctx, "browser_navigating", map[string]any{"url": opts.URL, "session_id": opts.SessionID})
		if err := page.Navigate(opts.URL); err != nil {
			return nil, "", "", err
		}
		_ = page.WaitLoad()
		_ = page.WaitIdle(1500 * time.Millisecond)
	}

	pdf, err := SavePDF(page)
	if err != nil {
		return nil, "", "", err
	}
	finalURL := pageURL(page, opts.URL)
	if session != nil {
		m.touchSession(session, finalURL)
	}
	return pdf, finalURL, opts.SessionID, nil
}

// CreateSession creates a live persistent browser page for multi-step browsing.
func (m *Manager) CreateSession(ctx context.Context, userID string) (*models.BrowserSession, error) {
	if err := m.ensureEnabled(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	sessionCount := len(m.sessions)
	m.mu.RUnlock()
	if m.cfg.BrowserMaxSessions > 0 && sessionCount >= m.cfg.BrowserMaxSessions {
		return nil, fmt.Errorf("maximum browser sessions reached (%d)", m.cfg.BrowserMaxSessions)
	}

	page, err := m.newPage(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	id := "sess_" + uuid.New().String()
	session := &Session{
		ID:         id,
		UserID:     userID,
		Page:       page,
		CreatedAt:  now,
		LastUsedAt: now,
	}
	model := &models.BrowserSession{
		ID:         id,
		UserID:     userID,
		CreatedAt:  now,
		LastUsedAt: now,
		CurrentURL: "",
		Metadata:   "{}",
	}
	if m.repo != nil {
		if err := m.repo.Create(model); err != nil {
			_ = page.Close()
			return nil, err
		}
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()
	return model, nil
}

// CloseSession closes a live page and removes its DB row.
func (m *Manager) CloseSession(ctx context.Context, userID, id string) error {
	_ = ctx
	session, err := m.getSession(userID, id)
	if err != nil {
		return err
	}
	_ = session.Page.Close()

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	if m.repo != nil {
		return m.repo.Delete(id)
	}
	return nil
}

// ListSessions returns active in-memory sessions for the user.
func (m *Manager) ListSessions(userID string) []models.BrowserSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]models.BrowserSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		if userID != "" && s.UserID != userID {
			continue
		}
		out = append(out, models.BrowserSession{
			ID:         s.ID,
			UserID:     s.UserID,
			CreatedAt:  s.CreatedAt,
			LastUsedAt: s.LastUsedAt,
			CurrentURL: s.CurrentURL,
			Metadata:   "{}",
		})
	}
	return out
}

// Status returns browser runtime status without launching Chromium.
func (m *Manager) Status() Status {
	if m == nil || m.cfg == nil {
		return Status{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Status{
		Enabled:        m.cfg.BrowserEnabled,
		CacheDir:       m.cfg.BrowserCacheDir,
		ExecPath:       m.cfg.BrowserExecPath,
		ActiveSessions: len(m.sessions),
		BrowserRunning: m.browser != nil,
	}
}

// Shutdown closes all sessions and the shared browser process.
func (m *Manager) Shutdown(ctx context.Context) error {
	var err error
	m.stopOnce.Do(func() {
		close(m.stopCh)

		m.mu.Lock()
		sessions := make([]*Session, 0, len(m.sessions))
		for _, session := range m.sessions {
			sessions = append(sessions, session)
		}
		m.sessions = make(map[string]*Session)
		b := m.browser
		l := m.launcher
		m.browser = nil
		m.launcher = nil
		m.mu.Unlock()

		for _, session := range sessions {
			_ = session.Page.Close()
		}
		if m.repo != nil {
			_ = m.repo.DeleteAll()
		}
		if b != nil {
			err = b.Close()
		}
		if l != nil {
			l.Cleanup()
		}

		select {
		case <-ctx.Done():
			if err == nil {
				err = ctx.Err()
			}
		default:
		}
	})
	return err
}

func (m *Manager) pageForNavigation(ctx context.Context, userID, sessionID string) (*rod.Page, *Session, bool, error) {
	if sessionID != "" {
		session, err := m.getSession(userID, sessionID)
		if err != nil {
			return nil, nil, false, err
		}
		return session.Page, session, false, nil
	}
	page, err := m.newPage(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	return page, nil, true, nil
}

func (m *Manager) getSession(userID, id string) (*Session, error) {
	m.mu.RLock()
	session := m.sessions[id]
	m.mu.RUnlock()
	if session == nil {
		return nil, fmt.Errorf("browser session not found: %s", id)
	}
	if userID != "" && session.UserID != userID {
		return nil, fmt.Errorf("browser session not found: %s", id)
	}
	return session, nil
}

func (m *Manager) touchSession(session *Session, currentURL string) {
	now := time.Now().UTC()
	session.LastUsedAt = now
	session.CurrentURL = currentURL
	if m.repo != nil {
		if err := m.repo.UpdateLastUsed(session.ID, currentURL); err != nil {
			log.Printf("[browser] update session %s: %v", session.ID, err)
		}
	}
}

func (m *Manager) ensureEnabled() error {
	if m == nil || m.cfg == nil || !m.cfg.BrowserEnabled {
		return fmt.Errorf("headless browser is disabled by OMNILLM_BROWSER_ENABLED")
	}
	return nil
}

func (m *Manager) ensureBrowser(ctx context.Context) (*rod.Browser, error) {
	if err := m.ensureEnabled(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	if m.browser != nil {
		b := m.browser
		m.mu.RUnlock()
		return b, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.browser != nil {
		return m.browser, nil
	}

	if err := os.MkdirAll(m.cfg.BrowserCacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create browser cache dir: %w", err)
	}

	if m.cfg.BrowserExecPath == "" {
		launcher.DefaultBrowserDir = m.cfg.BrowserCacheDir
		emitProgress(ctx, "browser_downloading", map[string]any{"progress_percent": 0})
	}

	userDataDir := filepath.Join(m.cfg.BrowserCacheDir, "user-data")
	l := launcher.New().
		Context(ctx).
		Headless(true).
		NoSandbox(true).
		UserDataDir(userDataDir).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-extensions").
		Set("disable-dev-shm-usage").
		Set("lang", "en-US")
	if m.cfg.BrowserExecPath != "" {
		l = l.Bin(m.cfg.BrowserExecPath)
	}

	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch chromium: %w", err)
	}
	browser := rod.New().ControlURL(controlURL).Context(ctx)
	if err := browser.Connect(); err != nil {
		l.Cleanup()
		return nil, fmt.Errorf("connect chromium: %w", err)
	}

	if m.cfg.BrowserExecPath == "" {
		emitProgress(ctx, "browser_downloading", map[string]any{"progress_percent": 100})
	}
	m.browser = browser
	m.launcher = l
	return browser, nil
}

func (m *Manager) newPage(ctx context.Context) (*rod.Page, error) {
	b, err := m.ensureBrowser(ctx)
	if err != nil {
		return nil, err
	}
	page, err := stealth.Page(b)
	if err != nil {
		return nil, fmt.Errorf("create stealth page: %w", err)
	}
	page = page.Context(ctx).Timeout(45 * time.Second)
	if err := applyStealthProfile(page); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("apply browser profile: %w", err)
	}
	return page, nil
}

func (m *Manager) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.evictExpired()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) evictExpired() {
	ttl := m.cfg.BrowserSessionTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	cutoff := time.Now().UTC().Add(-ttl)

	var expired []*Session
	m.mu.Lock()
	for id, session := range m.sessions {
		if session.LastUsedAt.Before(cutoff) {
			expired = append(expired, session)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()

	for _, session := range expired {
		_ = session.Page.Close()
		if m.repo != nil {
			_ = m.repo.Delete(session.ID)
		}
	}
	if m.repo != nil {
		_ = m.repo.CleanupExpired(cutoff)
	}
}

func pageTitle(page *rod.Page, fallback string) string {
	info, err := page.Info()
	if err == nil && strings.TrimSpace(info.Title) != "" {
		return info.Title
	}
	return fallback
}

func pageURL(page *rod.Page, fallback string) string {
	info, err := page.Info()
	if err == nil && strings.TrimSpace(info.URL) != "" {
		return info.URL
	}
	return fallback
}

// ErrBotProtection indicates that the loaded document appears to be an
// anti-bot challenge rather than the requested page content.
var ErrBotProtection = fmt.Errorf("bot protection challenge detected")

func validateURL(ctx context.Context, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must start with http:// or https://")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("DNS resolution returned no IPs")
	}
	for _, ipAddr := range ips {
		if isBlockedIP(ipAddr.IP) {
			return fmt.Errorf("blocked private/internal IP %s resolved from %s", ipAddr.IP, host)
		}
	}
	return nil
}

var blockedCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"0.0.0.0/8",
		"100.64.0.0/10",
		"192.0.0.0/24",
		"198.18.0.0/15",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network != nil {
			nets = append(nets, network)
		}
	}
	return nets
}()

func isBlockedIP(ip net.IP) bool {
	for _, network := range blockedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
