package tokenstore_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

func TestParseConnections(t *testing.T) {
	const ok = `connections:
  basecamp:
    token_url: https://launchpad.37signals.com/authorization/token
    client_id_secret: basecamp_client_id
    client_secret_secret: basecamp_client_secret
    style: launchpad
    user_agent: "rela-basecamp (ops@example.com)"
`
	conns, err := tokenstore.ParseConnections([]byte(ok))
	if err != nil {
		t.Fatal(err)
	}
	c := conns["basecamp"]
	if c.Style != tokenstore.StyleLaunchpad || c.ClientIDSecret != "basecamp_client_id" || c.UserAgent == "" {
		t.Fatalf("parsed %+v", c)
	}

	if conns, err := tokenstore.ParseConnections(nil); err != nil || len(conns) != 0 {
		t.Fatalf("empty file: %v, %v", conns, err)
	}

	tests := []struct {
		name, replace, with, wantErr string
	}{
		{"http to a remote host", "https://launchpad.37signals.com", "http://launchpad.37signals.com", "https"},
		{"other scheme", "https://launchpad.37signals.com", "ftp://x", "https"},
		{"relative URL", "https://launchpad.37signals.com/authorization/token", "/token", "absolute"},
		{"userinfo", "https://launchpad", "https://u:p@launchpad", "userinfo"},
		{"bad style", "style: launchpad", "style: magic", "style"},
		{"missing style", "    style: launchpad\n", "", "style"},
		{"bad secret name", "basecamp_client_id", "has space", "client_id_secret"},
		{"token key as client secret", "basecamp_client_secret", "token_key", "token store key"},
		{"bad connection name", "  basecamp:", "  Basecamp:", "invalid connection name"},
		{"unknown field", "style: launchpad", "style: launchpad\n    scope: x", "scope"},
		{"control char in user agent", `"rela-basecamp (ops@example.com)"`, `"a\tb"`, "user_agent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tokenstore.ParseConnections([]byte(strings.Replace(ok, tc.replace, tc.with, 1)))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}

	for _, loop := range []string{"http://127.0.0.1:8080/t", "http://localhost/t", "http://[::1]:9/t"} {
		doc := strings.Replace(ok, "https://launchpad.37signals.com/authorization/token", loop, 1)
		if _, err := tokenstore.ParseConnections([]byte(doc)); err != nil {
			t.Errorf("loopback %s refused: %v", loop, err)
		}
	}
}
