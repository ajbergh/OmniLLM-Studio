package browser

import (
	"context"
	"fmt"
	"log"
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

const browserOperationTimeout = 45 * time.Second

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
	Sandboxed      bool   `json:"sandboxed"`
}

// Manager owns the shared Chromium process. Every page is created in a separate
// incognito BrowserContext so cookies and storage cannot cross sessions or users.
type Manager struct {
	cfg  *config.Config
	repo *repository.BrowserSessionRepo

	mu         sync.RWMutex
	launchMu   sync.Mutex
	browser    *rod.Browser
	launcher   *launcher.Launcher
	profileDir string
	sessions   map[string]*Session

	initOnce sync.Once
	initErr  error
	stopOnce sync.Once
	stopCh   chan struct{}
}

type pageLease struct {
	page           *rod.Page
	browserContext *rod.Browser
	session        *Session
	ephemeral      bool
	releaseOnce    sync.Once
}

func (l *pageLease) release() {
	if l == nil {
		return
	}
	l.releaseOnce.Do(func() {
		if l.session != nil {
			l.session.opMu.Unlock()
		}
		if l.ephemeral {
			_ = l.page.Close()
			disposeBrowserContext(l.browserContext)
		}
	})
}

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
	result, err := m.NavigatePage(ctx, NavigateOptions{URL: rawURL, Extract: "text", MaxChars: DefaultMaxExtractChars})
	if err != nil {
		return "", "", err
	}
	return result.Content, result.Title, nil
}

func (m *Manager) NavigatePage(ctx context.Context, opts NavigateOptions) (*NavigateResult, error) {
	if err := m.ensureEnabled(); err != nil {
		return nil, err
	}
	if err := validateURL(ctx, opts.URL); err != nil {
		return nil, err
	}
	lease, err := m.pageForNavigation(ctx, opts.UserID, opts.SessionID)
	if err != nil {
		return nil, err
	}
	defer lease.release()
	page := lease.page.Context(ctx).Timeout(browserOperationTimeout)

	emitProgress(ctx, "browser_navigating", map[string]any{"url": opts.URL, "session_id": opts.SessionID})
	if err := page.Navigate(opts.URL); err != nil {
		return nil, err
	}
	_ = page.WaitLoad()
	_ = page.WaitIdle(1500 * time.Millisecond)
	finalURL, err := m.validatePageDestination(ctx, lease, opts.URL)
	if err != nil {
		return nil, err
	}
	if opts.WaitFor != "" {
		if _, err := page.Element(opts.WaitFor); err != nil {
			return nil, fmt.Errorf("wait for selector %q: %w", opts.WaitFor, err)
		}
	}

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

	m.touchSession(lease.session, finalURL)
	content := text
	if extract == "html" {
		content = html
	} else if extract == "both" && html != "" {
		if content != "" {
			content += "\n\n--- HTML ---\n\n"
		}
		content += html
	}
	return &NavigateResult{
		Content: content, HTML: html, URL: finalURL, Title: pageTitle(page, finalURL),
		SessionID: opts.SessionID, CharCount: len(content),
	}, nil
}

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
	lease, err := m.pageForNavigation(ctx, opts.UserID, opts.SessionID)
	if err != nil {
		return nil, "", "", err
	}
	defer lease.release()
	page := lease.page.Context(ctx).Timeout(browserOperationTimeout)
	if opts.URL != "" {
		emitProgress(ctx, "browser_navigating", map[string]any{"url": opts.URL, "session_id": opts.SessionID})
		if err := page.Navigate(opts.URL); err != nil {
			return nil, "", "", err
		}
		_ = page.WaitLoad()
		_ = page.WaitIdle(1500 * time.Millisecond)
	}
	finalURL, err := m.validatePageDestination(ctx, lease, opts.URL)
	if err != nil {
		return nil, "", "", err
	}
	png, err := TakeScreenshot(page, opts.Selector, opts.FullPage)
	if err != nil {
		return nil, "", "", err
	}
	m.touchSession(lease.session, finalURL)
	emitProgress(ctx, "browser_screenshot_done", map[string]any{"url": finalURL, "session_id": opts.SessionID})
	return png, finalURL, opts.SessionID, nil
}

func (m *Manager) Interact(ctx context.Context, userID, sessionID, action, selector, value string) (string, string, error) {
	if err := m.ensureEnabled(); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(sessionID) == "" {
		return "", "", fmt.Errorf("session_id is required")
	}
	if strings.TrimSpace(selector) == "" && !strings.EqualFold(action, "scroll") {
		return "", "", fmt.Errorf("selector is required")
	}
	lease, err := m.pageForNavigation(ctx, userID, sessionID)
	if err != nil {
		return "", "", err
	}
	defer lease.release()
	page := lease.page.Context(ctx).Timeout(browserOperationTimeout)

	var element *rod.Element
	if selector != "" {
		element, err = page.Element(selector)
		if err != nil && !strings.EqualFold(action, "scroll") {
			return "", "", err
		}
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "click":
		err = element.Click(proto.InputMouseButtonLeft, 1)
	case "type":
		err = element.Input(value)
	case "select":
		err = element.Select([]string{value}, true, rod.SelectorTypeText)
	case "hover":
		err = element.Hover()
	case "scroll":
		if element != nil {
			err = element.ScrollIntoView()
		} else {
			_, err = page.Eval(`() => window.scrollBy(0, window.innerHeight * 0.8)`)
		}
	default:
		return "", "", fmt.Errorf("unsupported browser action %q", action)
	}
	if err != nil {
		return "", "", err
	}
	_ = page.WaitIdle(1500 * time.Millisecond)
	finalURL, err := m.validatePageDestination(ctx, lease, m.sessionCurrentURL(lease.session))
	if err != nil {
		return "", "", err
	}
	m.touchSession(lease.session, finalURL)
	emitProgress(ctx, "browser_interact_done", map[string]any{"action": action, "selector": selector, "session_id": sessionID})
	return finalURL, pageTitle(page, finalURL), nil
}

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
	lease, err := m.pageForNavigation(ctx, opts.UserID, opts.SessionID)
	if err != nil {
		return nil, "", "", err
	}
	defer lease.release()
	page := lease.page.Context(ctx).Timeout(browserOperationTimeout)
	if opts.URL != "" {
		emitProgress(ctx, "browser_navigating", map[string]any{"url": opts.URL, "session_id": opts.SessionID})
		if err := page.Navigate(opts.URL); err != nil {
			return nil, "", "", err
		}
		_ = page.WaitLoad()
		_ = page.WaitIdle(1500 * time.Millisecond)
	}
	finalURL, err := m.validatePageDestination(ctx, lease, opts.URL)
	if err != nil {
		return nil, "", "", err
	}
	pdf, err := SavePDF(page)
	if err != nil {
		return nil, "", "", err
	}
	m.touchSession(lease.session, finalURL)
	return pdf, finalURL, opts.SessionID, nil
}

func (m *Manager) CreateSession(ctx context.Context, userID string) (*models.BrowserSession, error) {
	if err := m.ensureEnabled(); err != nil {
		return nil, err
	}
	browserContext, page, err := m.newIsolatedPage(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	session := &Session{
		ID: "sess_" + uuid.New().String(), UserID: userID,
		BrowserContext: browserContext, Page: page, CreatedAt: now, LastUsedAt: now,
	}

	m.mu.Lock()
	if m.cfg.BrowserMaxSessions > 0 && m.countSessionsForUserLocked(userID) >= m.cfg.BrowserMaxSessions {
		m.mu.Unlock()
		_ = page.Close()
		disposeBrowserContext(browserContext)
		return nil, fmt.Errorf("maximum browser sessions reached for user (%d)", m.cfg.BrowserMaxSessions)
	}
	m.sessions[session.ID] = session
	m.mu.Unlock()

	model := &models.BrowserSession{
		ID: session.ID, UserID: userID, CreatedAt: now, LastUsedAt: now,
		CurrentURL: "", Metadata: "{}",
	}
	if m.repo != nil {
		if err := m.repo.Create(model); err != nil {
			_ = m.closeSessionInternal(session.ID)
			return nil, err
		}
	}
	return model, nil
}

func (m *Manager) CloseSession(ctx context.Context, userID, id string) error {
	_ = ctx
	session, err := m.getSession(userID, id)
	if err != nil {
		return err
	}
	if err := m.closeSessionInternal(session.ID); err != nil {
		return err
	}
	if m.repo != nil {
		return m.repo.Delete(id)
	}
	return nil
}

func (m *Manager) ListSessions(userID string) []models.BrowserSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]models.BrowserSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		if userID != "" && session.UserID != userID {
			continue
		}
		out = append(out, models.BrowserSession{
			ID: session.ID, UserID: session.UserID, CreatedAt: session.CreatedAt,
			LastUsedAt: session.LastUsedAt, CurrentURL: session.CurrentURL, Metadata: "{}",
		})
	}
	return out
}

func (m *Manager) Status() Status {
	if m == nil || m.cfg == nil {
		return Status{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Status{
		Enabled: m.cfg.BrowserEnabled, CacheDir: m.cfg.BrowserCacheDir,
		ExecPath: m.cfg.BrowserExecPath, ActiveSessions: len(m.sessions),
		BrowserRunning: m.browser != nil, Sandboxed: !m.cfg.BrowserNoSandbox,
	}
}

func (m *Manager) Shutdown(ctx context.Context) error {
	var firstErr error
	m.stopOnce.Do(func() {
		close(m.stopCh)
		m.mu.Lock()
		sessions := make([]*Session, 0, len(m.sessions))
		for _, session := range m.sessions {
			sessions = append(sessions, session)
		}
		m.sessions = make(map[string]*Session)
		browser := m.browser
		browserLauncher := m.launcher
		profileDir := m.profileDir
		m.browser, m.launcher, m.profileDir = nil, nil, ""
		m.mu.Unlock()

		for _, session := range sessions {
			session.opMu.Lock()
			_ = session.Page.Close()
			disposeBrowserContext(session.BrowserContext)
			session.opMu.Unlock()
		}
		if m.repo != nil {
			_ = m.repo.DeleteAll()
		}
		if browser != nil {
			if err := browser.Close(); err != nil {
				firstErr = err
			}
		}
		if browserLauncher != nil {
			browserLauncher.Cleanup()
		}
		if profileDir != "" {
			_ = os.RemoveAll(profileDir)
		}
		if ctx.Err() != nil && firstErr == nil {
			firstErr = ctx.Err()
		}
	})
	return firstErr
}

func (m *Manager) pageForNavigation(ctx context.Context, userID, sessionID string) (*pageLease, error) {
	if sessionID != "" {
		session, err := m.getSession(userID, sessionID)
		if err != nil {
			return nil, err
		}
		session.opMu.Lock()
		return &pageLease{page: session.Page, browserContext: session.BrowserContext, session: session}, nil
	}
	browserContext, page, err := m.newIsolatedPage(ctx)
	if err != nil {
		return nil, err
	}
	return &pageLease{page: page, browserContext: browserContext, ephemeral: true}, nil
}

func (m *Manager) getSession(userID, id string) (*Session, error) {
	m.mu.RLock()
	session := m.sessions[id]
	m.mu.RUnlock()
	if session == nil || (userID != "" && session.UserID != userID) {
		return nil, fmt.Errorf("browser session not found: %s", id)
	}
	return session, nil
}

func (m *Manager) closeSessionInternal(id string) error {
	m.mu.Lock()
	session := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if session == nil {
		return nil
	}
	session.opMu.Lock()
	defer session.opMu.Unlock()
	pageErr := session.Page.Close()
	disposeBrowserContext(session.BrowserContext)
	return pageErr
}

func (m *Manager) countSessionsForUserLocked(userID string) int {
	count := 0
	for _, session := range m.sessions {
		if session.UserID == userID {
			count++
		}
	}
	return count
}

func (m *Manager) touchSession(session *Session, currentURL string) {
	if session == nil {
		return
	}
	now := time.Now().UTC()
	m.mu.Lock()
	if current := m.sessions[session.ID]; current == session {
		session.LastUsedAt = now
		session.CurrentURL = currentURL
	}
	m.mu.Unlock()
	if m.repo != nil {
		if err := m.repo.UpdateLastUsed(session.ID, currentURL); err != nil {
			log.Printf("[browser] update session %s: %v", session.ID, err)
		}
	}
}

func (m *Manager) sessionCurrentURL(session *Session) string {
	if session == nil {
		return "about:blank"
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return session.CurrentURL
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
		browser := m.browser
		m.mu.RUnlock()
		return browser, nil
	}
	m.mu.RUnlock()

	m.launchMu.Lock()
	defer m.launchMu.Unlock()
	m.mu.RLock()
	if m.browser != nil {
		browser := m.browser
		m.mu.RUnlock()
		return browser, nil
	}
	m.mu.RUnlock()

	if err := os.MkdirAll(m.cfg.BrowserCacheDir, 0700); err != nil {
		return nil, fmt.Errorf("create browser cache dir: %w", err)
	}
	if m.cfg.BrowserExecPath == "" {
		launcher.DefaultBrowserDir = m.cfg.BrowserCacheDir
		emitProgress(ctx, "browser_downloading", map[string]any{"progress_percent": 0})
	}
	profileDir := filepath.Join(m.cfg.BrowserCacheDir, "profiles", uuid.New().String())
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		return nil, fmt.Errorf("create browser profile: %w", err)
	}
	browserLauncher := launcher.New().
		Context(context.Background()).
		Headless(true).
		UserDataDir(profileDir).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-extensions").
		Set("disable-dev-shm-usage").
		Set("lang", "en-US")
	if m.cfg.BrowserNoSandbox {
		browserLauncher = browserLauncher.NoSandbox(true)
	}
	if m.cfg.BrowserExecPath != "" {
		browserLauncher = browserLauncher.Bin(m.cfg.BrowserExecPath)
	}
	controlURL, err := browserLauncher.Launch()
	if err != nil {
		_ = os.RemoveAll(profileDir)
		return nil, fmt.Errorf("launch chromium: %w", err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		browserLauncher.Cleanup()
		_ = os.RemoveAll(profileDir)
		return nil, fmt.Errorf("connect chromium: %w", err)
	}
	if m.cfg.BrowserExecPath == "" {
		emitProgress(ctx, "browser_downloading", map[string]any{"progress_percent": 100})
	}
	m.mu.Lock()
	m.browser = browser
	m.launcher = browserLauncher
	m.profileDir = profileDir
	m.mu.Unlock()
	return browser, nil
}

func (m *Manager) newIsolatedPage(ctx context.Context) (*rod.Browser, *rod.Page, error) {
	browser, err := m.ensureBrowser(ctx)
	if err != nil {
		return nil, nil, err
	}
	browserContext, err := browser.Incognito()
	if err != nil {
		return nil, nil, fmt.Errorf("create isolated browser context: %w", err)
	}
	page, err := stealth.Page(browserContext)
	if err != nil {
		disposeBrowserContext(browserContext)
		return nil, nil, fmt.Errorf("create stealth page: %w", err)
	}
	if err := applyStealthProfile(page.Timeout(browserOperationTimeout)); err != nil {
		_ = page.Close()
		disposeBrowserContext(browserContext)
		return nil, nil, fmt.Errorf("apply browser profile: %w", err)
	}
	return browserContext, page, nil
}

func disposeBrowserContext(browserContext *rod.Browser) {
	if browserContext == nil || browserContext.BrowserContextID == "" {
		return
	}
	_ = proto.TargetDisposeBrowserContext{BrowserContextID: browserContext.BrowserContextID}.Call(browserContext)
}

func (m *Manager) validatePageDestination(ctx context.Context, lease *pageLease, fallback string) (string, error) {
	finalURL := pageURL(lease.page, fallback)
	if strings.HasPrefix(finalURL, "about:blank") && fallback == "" {
		return finalURL, nil
	}
	if err := validateURL(ctx, finalURL); err != nil {
		if lease.session != nil {
			id := lease.session.ID
			// The current operation owns opMu. Remove the session now; release()
			// unlocks it, and a goroutine disposes the compromised context afterward.
			m.mu.Lock()
			delete(m.sessions, id)
			m.mu.Unlock()
			go func(session *Session) {
				session.opMu.Lock()
				defer session.opMu.Unlock()
				_ = session.Page.Close()
				disposeBrowserContext(session.BrowserContext)
				if m.repo != nil {
					_ = m.repo.Delete(session.ID)
				}
			}(lease.session)
		}
		return "", fmt.Errorf("browser navigation reached an unsafe destination: %w", err)
	}
	return finalURL, nil
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
		session.opMu.Lock()
		_ = session.Page.Close()
		disposeBrowserContext(session.BrowserContext)
		session.opMu.Unlock()
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

var ErrBotProtection = fmt.Errorf("bot protection challenge detected")

func validateURL(ctx context.Context, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must start with http:// or https://")
	}
	if parsed.User != nil {
		return fmt.Errorf("URL credentials are not allowed")
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
	for _, resolved := range ips {
		if isBlockedIP(resolved.IP) {
			return fmt.Errorf("blocked private/internal IP %s resolved from %s", resolved.IP, host)
		}
	}
	return nil
}

var blockedCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.168.0.0/16",
		"198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4", "::/128", "::1/128",
		"fc00::/7", "fe80::/10", "ff00::/8",
	}
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network != nil {
			networks = append(networks, network)
		}
	}
	return networks
}()

func isBlockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, network := range blockedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
