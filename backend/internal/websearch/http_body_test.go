package websearch

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

func gzipBytes(t *testing.T, payload string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadResponseBodyPlain(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	got, err := readResponseBody(resp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("got %q", got)
	}
}

func TestReadResponseBodyGzip(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Encoding", "gzip")
	resp := &http.Response{
		Header: header,
		Body:   io.NopCloser(bytes.NewReader(gzipBytes(t, `{"ok":true}`))),
	}
	got, err := readResponseBody(resp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("gzip body did not decode: %q", got)
	}
}

func TestReadResponseBodyGzipCaseInsensitiveHeader(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Encoding", " GZIP ")
	resp := &http.Response{
		Header: header,
		Body:   io.NopCloser(bytes.NewReader(gzipBytes(t, `{"ok":true}`))),
	}
	got, err := readResponseBody(resp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("gzip body did not decode: %q", got)
	}
}

func TestReadResponseBodyGzipHeaderButPlainBody(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Encoding", "gzip")
	resp := &http.Response{
		Header: header,
		Body:   io.NopCloser(strings.NewReader("not gzip")),
	}
	if _, err := readResponseBody(resp); err == nil {
		t.Fatal("a gzip header with a plain body must error rather than return garbage")
	}
}

func TestReadResponseBodyNilResponse(t *testing.T) {
	if _, err := readResponseBody(nil); err == nil {
		t.Fatal("nil response must error")
	}
	if _, err := readResponseBody(&http.Response{Header: http.Header{}}); err == nil {
		t.Fatal("nil body must error")
	}
}

func TestReadResponseBodyCapsSize(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(strings.Repeat("a", maxProviderResponseBytes+4096))),
	}
	got, err := readResponseBody(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxProviderResponseBytes {
		t.Errorf("expected the read to stop at %d bytes, got %d", maxProviderResponseBytes, len(got))
	}
}

func TestTruncateProviderError(t *testing.T) {
	if got := truncateProviderError("  short  "); got != "short" {
		t.Errorf("short bodies are trimmed and passed through, got %q", got)
	}
	long := truncateProviderError(strings.Repeat("x", maxProviderErrorChars*3))
	if len(long) > maxProviderErrorChars+32 {
		t.Errorf("long bodies must be truncated, got %d chars", len(long))
	}
	if !strings.HasSuffix(long, "[truncated]") {
		t.Errorf("truncation must be marked, got %q", long[len(long)-32:])
	}
}
