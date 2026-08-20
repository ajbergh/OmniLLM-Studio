package websearch

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxProviderResponseBytes bounds how much of a search provider response is
// read into memory. Brave web+news payloads are well under this; the cap exists
// so a misbehaving or hostile endpoint cannot exhaust process memory.
const maxProviderResponseBytes = 8 << 20 // 8 MiB

// maxProviderErrorChars bounds how much of an error body is echoed into a Go
// error string. Provider errors reach logs, never clients, but they still must
// not carry an unbounded payload.
const maxProviderErrorChars = 512

// readResponseBody reads a search provider response, transparently decoding a
// gzip Content-Encoding.
//
// net/http performs transparent gzip decompression only when the transport
// added Accept-Encoding itself. A caller that sets that header — or an
// intermediary that compresses regardless — leaves the body encoded, so the
// encoding has to be handled here rather than assumed away.
func readResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("empty response")
	}
	var reader io.Reader = io.LimitReader(resp.Body, maxProviderResponseBytes)
	if strings.EqualFold(strings.TrimSpace(resp.Header.Get("Content-Encoding")), "gzip") {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gz.Close()
		reader = io.LimitReader(gz, maxProviderResponseBytes)
	}
	return io.ReadAll(reader)
}

// truncateProviderError bounds an upstream error body before it is wrapped into
// a Go error. Callers must not forward the result to API clients.
func truncateProviderError(body string) string {
	body = strings.TrimSpace(body)
	if len(body) <= maxProviderErrorChars {
		return body
	}
	return body[:maxProviderErrorChars] + "…[truncated]"
}
