package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestValidateConfig_ActionRequestHeaders pins that the request: header floor
// is reached from the REAL validation entry point, not only from
// validateActions called directly.
//
// The floor is a load-time check with no runtime counterpart — the handler
// projects whatever names the config holds, exactly as a declarative webhook
// does. That split is right, but it makes "is the validator actually wired into
// ValidateConfig?" the whole question, and a unit test that calls the validator
// by hand cannot answer it.
func TestValidateConfig_ActionRequestHeaders(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
			}},
		},
	}

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "authorization refused",
			yaml: `version: "1.0"
actions:
  hook:
    script: hook.lua
    request:
      body: true
      headers: [Authorization]
`,
			wantErr: "may not expose header",
		},
		{
			name: "proxy identity family refused by prefix",
			yaml: `version: "1.0"
actions:
  hook:
    script: hook.lua
    request:
      headers: [X-Forwarded-User]
`,
			wantErr: "may not expose header",
		},
		{
			name: "request on a set: action refused",
			yaml: `version: "1.0"
actions:
  hook:
    set:
      title: done
    request:
      body: true
`,
			wantErr: "request: requires script",
		},
		{
			name: "ordinary header accepted",
			yaml: `version: "1.0"
actions:
  hook:
    script: hook.lua
    request:
      body: true
      query: true
      headers: [X-Event-Type]
      max_body_bytes: 262144
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.yaml)
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			err := ValidateConfig(data, &cfg, meta)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no validation error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// TestValidateConfig_ActionRequestParsesFromYAML pins the YAML key names. A
// misnamed tag would leave every field at its zero value, which reads as "the
// operator did not opt in" — a config that loads clean and silently does
// nothing, rather than failing.
func TestValidateConfig_ActionRequestParsesFromYAML(t *testing.T) {
	const src = `version: "1.0"
actions:
  hook:
    script: hook.lua
    request:
      body: true
      query: true
      headers: [X-Event-Type]
      max_body_bytes: 4096
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	req := cfg.Actions["hook"].Request
	if req == nil {
		t.Fatal("request: block did not parse")
	}
	if !req.Body || !req.Query {
		t.Errorf("body/query = %v/%v, want true/true", req.Body, req.Query)
	}
	if len(req.Headers) != 1 || req.Headers[0] != "X-Event-Type" {
		t.Errorf("headers = %v", req.Headers)
	}
	if req.MaxBodyBytes != 4096 {
		t.Errorf("max_body_bytes = %d, want 4096", req.MaxBodyBytes)
	}
}
