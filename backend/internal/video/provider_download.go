package video

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const downloadMaxAttempts = 3

// downloadRetryBaseWait is a variable so tests can shorten retry backoff.
var downloadRetryBaseWait = 2 * time.Second

// downloadWithRetry fetches binary media with up to downloadMaxAttempts
// attempts, retrying transient network errors and retryable HTTP statuses
// (429 and 5xx) with linear backoff. Non-retryable statuses fail immediately.
// It returns the body bytes and the response Content-Type.
func downloadWithRetry(ctx context.Context, client *http.Client, url, label string, headers map[string]string) ([]byte, string, error) {
	var lastErr error
	for attempt := 0; attempt < downloadMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(time.Duration(attempt) * downloadRetryBaseWait):
			}
		}
		data, mimeType, retryable, err := downloadOnce(ctx, client, url, label, headers)
		if err == nil {
			return data, mimeType, nil
		}
		lastErr = err
		if !retryable {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("%s download failed after %d attempts: %w", label, downloadMaxAttempts, lastErr)
}

func downloadOnce(ctx context.Context, client *http.Client, url, label string, headers map[string]string) (data []byte, mimeType string, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", false, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", false, ctx.Err()
		}
		return nil, "", true, fmt.Errorf("download %s video: %w", label, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, "", retry, fmt.Errorf("%s video download returned %s: %s", label, resp.Status, responseSnippet(body))
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, maxProviderDownloadBytes))
	if err != nil {
		return nil, "", true, fmt.Errorf("read %s video download: %w", label, err)
	}
	return data, strings.TrimSpace(resp.Header.Get("Content-Type")), false, nil
}
