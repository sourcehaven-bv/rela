package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPHandlerAcceptsProxiedHost pins that the HTTP transport serves a
// request whose Host is the public name while the listener is loopback. That
// is the production topology: rela-server binds 127.0.0.1 behind pratique,
// which forwards Host unchanged. The go-sdk's DNS-rebinding guard rejects
// exactly this combination with a 403 unless it is disabled.
func TestHTTPHandlerAcceptsProxiedHost(t *testing.T) {
	ts := httptest.NewServer(newDispatchServer(t).HTTPHandler())
	t.Cleanup(ts.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`
	req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "atlas.example.internal"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
