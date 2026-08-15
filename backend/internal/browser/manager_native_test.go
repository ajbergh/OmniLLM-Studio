package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/go-rod/rod/lib/launcher"
)

const browserNativeFixtureTimeout = 10 * time.Second

// TestRequestPerimeterNativeFixtureMatrix proves that Chromium-originated traffic
// cannot escape the browser request perimeter through common subresource and
// worker mechanisms. CI sets OMNILLM_BROWSER_TEST_REQUIRE=true so a missing or
// unusable Chromium binary is a hard failure rather than a skipped assurance test.
// OMNILLM_BROWSER_TEST_NO_SANDBOX is test-only: GitHub's Ubuntu hosted runner
// blocks Chromium's user-namespace sandbox before startup, so that runner disables
// the Chromium process sandbox only to exercise the independent egress boundary.
func TestRequestPerimeterNativeFixtureMatrix(t *testing.T) {
	browserPath := strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_TEST_EXEC_PATH"))
	requireBrowser := strings.EqualFold(strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_TEST_REQUIRE")), "true")
	testNoSandbox := strings.EqualFold(strings.TrimSpace(os.Getenv("OMNILLM_BROWSER_TEST_NO_SANDBOX")), "true")
	if browserPath == "" {
		var found bool
		browserPath, found = launcher.LookPath()
		if !found {
			if requireBrowser {
				t.Fatal("OMNILLM_BROWSER_TEST_REQUIRE=true but no Chromium-compatible browser was found")
			}
			t.Skip("no local Chromium-compatible browser found")
		}
	}
	if _, err := os.Stat(browserPath); err != nil {
		if requireBrowser {
			t.Fatalf("required Chromium-compatible browser is unavailable at %q: %v", browserPath, err)
		}
		t.Skipf("no local Chromium-compatible browser found at %q: %v", browserPath, err)
	}

	blockedHosts := []string{
		"fetch.invalid",
		"iframe.invalid",
		"media.invalid",
		"redirect.invalid",
		"websocket.invalid",
		"worker.invalid",
		"shared-worker.invalid",
		"service-worker.invalid",
	}
	blocked := make(map[string]struct{}, len(blockedHosts))
	for _, host := range blockedHosts {
		blocked[host] = struct{}{}
	}

	var observationsMu sync.Mutex
	observations := make(map[string]int, len(blockedHosts))
	record := func(host string) {
		observationsMu.Lock()
		observations[host]++
		observationsMu.Unlock()
	}

	var pageServer *httptest.Server
	pageServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `<!doctype html>
<html><body>
<iframe src="http://iframe.invalid/frame"></iframe>
<audio preload="auto" src="http://media.invalid/audio.mp3"></audio>
<img src="/redirect" alt="redirect fixture">
<script>
fetch('http://fetch.invalid/data').catch(() => {});
try { new WebSocket('ws://websocket.invalid/socket'); } catch (_) {}
try { new Worker('/worker.js'); } catch (_) {}
try {
  const shared = new SharedWorker('/shared-worker.js');
  shared.port.start();
} catch (_) {}
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/service-worker.js')
    .then(() => navigator.serviceWorker.ready)
    .then((registration) => {
      if (registration.active) registration.active.postMessage('probe');
    })
    .catch(() => {});
}
</script>
</body></html>`)
		case "/redirect":
			http.Redirect(w, r, "http://redirect.invalid/final", http.StatusFound)
		case "/worker.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(w, `fetch('http://worker.invalid/data').catch(() => {});`)
		case "/shared-worker.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(w, `onconnect = () => { fetch('http://shared-worker.invalid/data').catch(() => {}); };`)
		case "/service-worker.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(w, `self.addEventListener('message', () => { fetch('http://service-worker.invalid/data').catch(() => {}); });`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer pageServer.Close()

	pageURL, err := url.Parse(pageServer.URL)
	if err != nil {
		t.Fatalf("parse page URL: %v", err)
	}

	manager := NewManager(&config.Config{
		BrowserEnabled:     true,
		BrowserExecPath:    browserPath,
		BrowserCacheDir:    t.TempDir(),
		BrowserMaxSessions: 1,
		BrowserSessionTTL:  time.Minute,
		BrowserNoSandbox:   testNoSandbox,
	}, nil)
	manager.newLauncher = func() *launcher.Launcher {
		return launcher.New().Leakless(false)
	}
	manager.validate = func(_ context.Context, raw string) error {
		parsed, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if _, deny := blocked[parsed.Hostname()]; deny {
			record(parsed.Hostname())
			return errors.New("native fixture policy rejected destination")
		}
		return nil
	}
	manager.resolveTarget = func(ctx context.Context, host, port string) ([]net.IP, error) {
		if host == pageURL.Hostname() && port == pageURL.Port() {
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}
		if _, deny := blocked[host]; deny {
			record(host)
			return nil, errors.New("native fixture proxy policy rejected destination")
		}
		return resolvePublicTarget(ctx, host, port)
	}

	navigated := false
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := manager.Shutdown(shutdownCtx); err != nil && navigated {
			t.Errorf("shutdown browser: %v", err)
		}
	})

	if _, err := manager.NavigatePage(context.Background(), NavigateOptions{
		URL: pageServer.URL, Extract: "text",
	}); err != nil {
		message := strings.ToLower(err.Error())
		startupFailure := strings.Contains(message, "launch chromium") || strings.Contains(message, "connect chromium") || strings.Contains(message, "create stealth page")
		if startupFailure && !requireBrowser {
			t.Skipf("local browser could not start: %v", err)
		}
		t.Fatalf("navigate native browser fixture: %v", err)
	}
	navigated = true

	deadline := time.Now().Add(browserNativeFixtureTimeout)
	for {
		missing := missingBrowserFixtureHosts(blockedHosts, observations, &observationsMu)
		if len(missing) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Chromium fixture traffic did not reach the request perimeter for: %s", strings.Join(missing, ", "))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func missingBrowserFixtureHosts(hosts []string, observations map[string]int, mu *sync.Mutex) []string {
	mu.Lock()
	defer mu.Unlock()
	missing := make([]string, 0)
	for _, host := range hosts {
		if observations[host] == 0 {
			missing = append(missing, host)
		}
	}
	return missing
}
