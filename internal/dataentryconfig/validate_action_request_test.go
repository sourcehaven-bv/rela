package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestValidateActions_RequestBlock pins the load-time rules for an action's
// opt-in `request:` block (TKT-EFMRQM). The header allowlist reuses the webhook
// floor, so a credential or proxy-asserted identity header is a load error
// exactly as it is on a declarative hook.
func TestValidateActions_RequestBlock(t *testing.T) {
	meta := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		request *ActionRequest
		wantErr string
	}{
		{
			name:    "no request block is valid",
			request: nil,
		},
		{
			name:    "body and query only",
			request: &ActionRequest{Body: true, Query: true},
		},
		{
			name:    "ordinary header allowed",
			request: &ActionRequest{Headers: []string{"X-Event-Type"}},
		},
		{
			name:    "authorization header refused",
			request: &ActionRequest{Headers: []string{"Authorization"}},
			wantErr: "may not expose header",
		},
		{
			name:    "cookie refused",
			request: &ActionRequest{Headers: []string{"Cookie"}},
			wantErr: "may not expose header",
		},
		{
			name:    "proxy identity family refused by prefix",
			request: &ActionRequest{Headers: []string{"X-Forwarded-Email"}},
			wantErr: "may not expose header",
		},
		{
			name:    "malformed header name refused",
			request: &ActionRequest{Headers: []string{"bad header"}},
			wantErr: "is not a valid HTTP header name",
		},
		{
			name:    "negative body cap refused",
			request: &ActionRequest{Body: true, MaxBodyBytes: -1},
			wantErr: "max_body_bytes must not be negative",
		},
		{
			name:    "over-ceiling body cap refused",
			request: &ActionRequest{Body: true, MaxBodyBytes: 1 << 30},
			wantErr: "exceeds the",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Actions: map[string]Action{
				"hook": {Script: "hook.lua", Request: tc.request},
			}}
			errs := validateActions(cfg, meta)
			joined := strings.Join(errs, "\n")
			if tc.wantErr == "" {
				if len(errs) > 0 {
					t.Fatalf("expected no errors, got: %s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("expected an error containing %q, got: %s", tc.wantErr, joined)
			}
		})
	}
}

// TestValidateActions_RequestBlockNeedsScript refuses `request:` on a
// declarative `set:` action: there is no script to receive it, so accepting it
// would leave an operator believing a body reached code that never runs.
func TestValidateActions_RequestBlockNeedsScript(t *testing.T) {
	cfg := &Config{Actions: map[string]Action{
		"decl": {Set: map[string]string{"status": "done"}, Request: &ActionRequest{Body: true}},
	}}
	errs := validateActions(cfg, &metamodel.Metamodel{})
	if !strings.Contains(strings.Join(errs, "\n"), "request: requires script") {
		t.Fatalf("expected a request-requires-script error, got: %v", errs)
	}
}
