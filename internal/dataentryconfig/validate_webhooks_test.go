package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// hookTestMeta is a metamodel with the properties the hooks below reference.
func hookTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"incident": {
				Label: "Incident",
				Properties: map[string]metamodel.PropertyDef{
					"title":   {Type: "string"},
					"host":    {Type: "string"},
					"service": {Type: "string"},
					"status":  {Type: "string"},
				},
			},
		},
	}
}

// TestValidateWebhooks covers the load-time contract. Every case here is a LOAD
// ERROR by design: a webhook that silently misfires loses data from a producer
// that generally does not retry, so failing the load is the safe direction.
func TestValidateWebhooks(t *testing.T) {
	tests := []struct {
		name    string
		hooks   map[string]Webhook
		wantErr string // substring; empty means the config must validate
	}{
		{
			name: "valid find-or-create",
			hooks: map[string]Webhook{"icinga-alert": {
				Find: &WebhookFind{Type: "incident", Match: []string{"host", "service"}},
				CreateIfMissing: &WebhookCreate{
					Properties: map[string]string{"title": "{{body.host}}", "status": "open"},
				},
				Then: []WebhookStep{{
					AppendSection: &WebhookAppendSection{Section: "Notifications", Content: "- {{body.state}}"},
				}},
			}},
		},
		{
			name: "valid always-create",
			hooks: map[string]Webhook{"intake": {
				CreateIfMissing: &WebhookCreate{
					Type: "incident", Properties: map[string]string{"title": "{{body.subject}}"},
				},
			}},
		},
		{
			name: "valid find-and-update-only needs no match key",
			hooks: map[string]Webhook{"resolve": {
				Find: &WebhookFind{Type: "incident", Match: []string{"host"}},
				Then: []WebhookStep{{Set: map[string]string{"status": "closed"}}},
			}},
		},
		{
			name:    "hook id with a path separator is refused",
			hooks:   map[string]Webhook{"bad/id": {CreateIfMissing: &WebhookCreate{Type: "incident"}}},
			wantErr: "invalid hook ID",
		},
		{
			name:    "hook id with traversal is refused",
			hooks:   map[string]Webhook{"..": {CreateIfMissing: &WebhookCreate{Type: "incident"}}},
			wantErr: "invalid hook ID",
		},
		{
			name:    "hook id with uppercase is refused",
			hooks:   map[string]Webhook{"Alert": {CreateIfMissing: &WebhookCreate{Type: "incident"}}},
			wantErr: "invalid hook ID",
		},
		{
			name:    "neither find nor create is refused",
			hooks:   map[string]Webhook{"empty": {}},
			wantErr: "neither find nor create_if_missing",
		},
		{
			name:    "unknown find type is refused",
			hooks:   map[string]Webhook{"h": {Find: &WebhookFind{Type: "nope", Match: []string{"host"}}}},
			wantErr: "is not a known entity type",
		},
		{
			name:    "unknown create type is refused",
			hooks:   map[string]Webhook{"h": {CreateIfMissing: &WebhookCreate{Type: "nope"}}},
			wantErr: "is not a known entity type",
		},
		{
			name: "unknown match property is refused",
			hooks: map[string]Webhook{"h": {
				Find: &WebhookFind{Type: "incident", Match: []string{"nosuchprop"}},
			}},
			wantErr: "unknown property",
		},
		{
			name: "unknown created property is refused",
			hooks: map[string]Webhook{"h": {
				CreateIfMissing: &WebhookCreate{Type: "incident", Properties: map[string]string{"bogus": "x"}},
			}},
			wantErr: "unknown property",
		},
		{
			// The core of the three-workflow rule: a match key is required only
			// when a create can happen, or every delivery mints a duplicate.
			name: "create_if_missing without a match key is refused",
			hooks: map[string]Webhook{"h": {
				Find:            &WebhookFind{Type: "incident"},
				CreateIfMissing: &WebhookCreate{Properties: map[string]string{"title": "x"}},
			}},
			wantErr: "a match key is required",
		},
		{
			name: "find.values naming a non-match property is refused",
			hooks: map[string]Webhook{"h": {
				Find: &WebhookFind{
					Type: "incident", Match: []string{"host"},
					Values: map[string]string{"service": "{{body.svc}}"},
				},
			}},
			wantErr: "not in find.match",
		},
		{
			name: "a step with no action is refused",
			hooks: map[string]Webhook{"h": {
				Find: &WebhookFind{Type: "incident", Match: []string{"host"}},
				Then: []WebhookStep{{}},
			}},
			wantErr: "declares no action",
		},
		{
			name: "a step with two actions is refused",
			hooks: map[string]Webhook{"h": {
				Find: &WebhookFind{Type: "incident", Match: []string{"host"}},
				Then: []WebhookStep{{
					AppendSection: &WebhookAppendSection{Section: "S", Content: "c"},
					Set:           map[string]string{"status": "x"},
				}},
			}},
			wantErr: "more than one action",
		},
		{
			name: "append_section without content is refused",
			hooks: map[string]Webhook{"h": {
				Find: &WebhookFind{Type: "incident", Match: []string{"host"}},
				Then: []WebhookStep{{AppendSection: &WebhookAppendSection{Section: "S"}}},
			}},
			wantErr: "requires content",
		},
		{
			name: "set of an unknown property is refused",
			hooks: map[string]Webhook{"h": {
				Find: &WebhookFind{Type: "incident", Match: []string{"host"}},
				Then: []WebhookStep{{Set: map[string]string{"bogus": "x"}}},
			}},
			wantErr: "unknown property",
		},
		{
			name: "a non-2xx respond status is refused",
			hooks: map[string]Webhook{"h": {
				CreateIfMissing: &WebhookCreate{Type: "incident"},
				Respond:         WebhookRespond{Status: 500},
			}},
			wantErr: "must be a 2xx",
		},
		{
			name: "an oversized body cap is refused",
			hooks: map[string]Webhook{"h": {
				CreateIfMissing: &WebhookCreate{Type: "incident"},
				MaxBodyBytes:    1 << 30,
			}},
			wantErr: "exceeds the",
		},
		{
			name: "a negative body cap is refused",
			hooks: map[string]Webhook{"h": {
				CreateIfMissing: &WebhookCreate{Type: "incident"},
				MaxBodyBytes:    -1,
			}},
			wantErr: "must not be negative",
		},
		{
			name: "a malformed header name is refused",
			hooks: map[string]Webhook{"h": {
				CreateIfMissing: &WebhookCreate{Type: "incident"},
				Headers:         []string{"X Bad Header"},
			}},
			wantErr: "not a valid HTTP header name",
		},
		{
			name: "a benign header is allowed",
			hooks: map[string]Webhook{"h": {
				CreateIfMissing: &WebhookCreate{Type: "incident"},
				Headers:         []string{"X-Delivery-Id"},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Webhooks: tc.hooks}
			errs := validateWebhooks(cfg, hookTestMeta())
			joined := strings.Join(errs, "\n")

			if tc.wantErr == "" {
				if len(errs) > 0 {
					t.Fatalf("expected the config to validate, got:\n%s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("expected an error containing %q, got:\n%s", tc.wantErr, joined)
			}
		})
	}
}

// TestValidateWebhooks_ForbiddenHeadersRefused pins the credential floor: these
// header names are refused however the operator writes them, because
// interpolating one into entity content persists a secret into the graph where
// it is served back on every read.
func TestValidateWebhooks_ForbiddenHeadersRefused(t *testing.T) {
	forbidden := []string{
		"Authorization",
		"authorization", // case must not evade the check
		"Cookie",
		"Proxy-Authorization",
		"X-Forwarded-User",
		"X-Auth-Request-Access-Token",
		"X-Remote-User",
	}

	for _, header := range forbidden {
		t.Run(header, func(t *testing.T) {
			cfg := &Config{Webhooks: map[string]Webhook{"h": {
				CreateIfMissing: &WebhookCreate{Type: "incident"},
				Headers:         []string{header},
			}}}
			errs := validateWebhooks(cfg, hookTestMeta())
			joined := strings.Join(errs, "\n")
			if !strings.Contains(joined, "may not expose header") {
				t.Fatalf("header %q was accepted; it carries credentials or a proxy-asserted identity.\nErrors: %s",
					header, joined)
			}
		})
	}
}

// TestValidateWebhooks_ErrorsAreDeterministic pins that repeated validation of
// the same config yields the same ordered errors, so a config failure is
// reproducible rather than varying with map iteration order.
func TestValidateWebhooks_ErrorsAreDeterministic(t *testing.T) {
	cfg := &Config{Webhooks: map[string]Webhook{
		"aaa": {Find: &WebhookFind{Type: "nope", Match: []string{"host"}}},
		"bbb": {Find: &WebhookFind{Type: "nope", Match: []string{"host"}}},
		"ccc": {Find: &WebhookFind{Type: "nope", Match: []string{"host"}}},
	}}

	first := strings.Join(validateWebhooks(cfg, hookTestMeta()), "\n")
	for range 8 {
		if got := strings.Join(validateWebhooks(cfg, hookTestMeta()), "\n"); got != first {
			t.Fatalf("validation order is not deterministic:\nfirst:\n%s\nlater:\n%s", first, got)
		}
	}
}

// TestConfig_WebhooksIsAValidTopLevelKey pins that `webhooks:` survives the
// strict unknown-key check — without this the whole feature would be rejected
// at load with "unknown key".
func TestConfig_WebhooksIsAValidTopLevelKey(t *testing.T) {
	if !validTopLevelKeys["webhooks"] {
		t.Fatal("`webhooks` must be a recognized top-level data-entry.yaml key")
	}
}
