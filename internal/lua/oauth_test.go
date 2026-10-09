package lua_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

type fakeOAuth struct {
	calls []string
	err   error
}

func (f *fakeOAuth) AccessToken(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, "access "+name)
	return "tok-" + name, f.err
}

func (f *fakeOAuth) Invalidate(_ context.Context, name, rejected string) error {
	f.calls = append(f.calls, "invalidate "+name+" "+rejected)
	return f.err
}

// connectorCtx is the context a connector script runs under.
func connectorCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "integration:crm", Tool: principal.ToolScheduler})
}

func runOAuth(ctx context.Context, t *testing.T, deps lua.WriteDeps, caps lua.Capabilities, src string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rt := lua.NewWriter(deps, &out, lua.WithContext(ctx), lua.WithPrincipal(principal.From(ctx)),
		lua.WithCapabilities(caps))
	defer rt.Close()
	err := rt.RunString(src) //nolint:contextcheck // ctx reaches the runtime through lua.WithContext above
	return out.String(), err
}

func TestOAuth_Bindings(t *testing.T) {
	granted := lua.Capabilities{Tokens: []string{"crm"}}
	tests := []struct {
		name      string
		caps      lua.Capabilities
		noBroker  bool
		err       error
		src       string
		want      string
		wantCalls string
	}{
		{
			name: "access token", caps: granted,
			src:  `rela.output(rela.oauth.access_token("crm"))`,
			want: "tok-crm", wantCalls: "access crm",
		},
		{
			name: "invalidate", caps: granted,
			src:  `rela.output(tostring(rela.oauth.invalidate("crm", "old")))`,
			want: "true", wantCalls: "invalidate crm old",
		},
		{
			name: "connection not granted", caps: granted,
			src:  `local t, e = rela.oauth.access_token("jira"); rela.output(tostring(t) .. " " .. e.kind)`,
			want: "nil denied",
		},
		{
			name: "all tokens", caps: lua.Capabilities{AllTokens: true},
			src:  `rela.output(rela.oauth.access_token("jira"))`,
			want: "tok-jira", wantCalls: "access jira",
		},
		{
			name: "no token store", caps: granted, noBroker: true,
			src:  `local _, e = rela.oauth.access_token("crm"); rela.output(e.kind)`,
			want: "not_configured",
		},
		{
			name: "needs consent", caps: granted, err: fmt.Errorf("x: %w", lua.ErrOAuthNeedsConsent),
			src:  `local _, e = rela.oauth.access_token("crm"); rela.output(e.kind)`,
			want: "needs_consent", wantCalls: "access crm",
		},
		{
			name: "undeclared connection", caps: granted, err: fmt.Errorf("x: %w", lua.ErrOAuthNotConfigured),
			src:  `local _, e = rela.oauth.invalidate("crm", "t"); rela.output(e.kind)`,
			want: "not_configured", wantCalls: "invalidate crm t",
		},
		{
			name: "other failure", caps: granted, err: errors.New("provider down"),
			src:  `local _, e = rela.oauth.access_token("crm"); rela.output(e.kind .. ":" .. e.message)`,
			want: "error:provider down", wantCalls: "access crm",
		},
		{
			name: "absent without a grant", caps: lua.Capabilities{HTTP: true},
			src:  `rela.output(type(rela.oauth))`,
			want: "nil",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newHistoryWorld(t)
			f := &fakeOAuth{err: tc.err}
			if !tc.noBroker {
				deps.OAuth = f
			}
			out, err := runOAuth(connectorCtx(), t, deps, tc.caps, tc.src)
			if err != nil {
				t.Fatalf("script: %v", err)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("output %q lacks %q", out, tc.want)
			}
			if got := strings.Join(f.calls, ";"); got != tc.wantCalls {
				t.Errorf("calls = %q, want %q", got, tc.wantCalls)
			}
		})
	}
}

// R13: a refresh is slow network I/O and must not run inside a store
// transaction.
func TestOAuth_RefusedInTx(t *testing.T) {
	deps := newHistoryWorld(t)
	f := &fakeOAuth{}
	deps.OAuth = f
	ctx := store.ContextInTx(principal.With(context.Background(), principal.Principal{User: "bot"}))
	_, err := runOAuth(ctx, t, deps, lua.Capabilities{Tokens: []string{"crm"}}, `rela.oauth.access_token("crm")`)
	if err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Fatalf("err = %v, want a transaction refusal", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("broker called inside a transaction: %v", f.calls)
	}
}

// A reader runtime never gets rela.oauth, whatever its grant: ReadDeps has
// no token source to give it.
func TestOAuth_AbsentOnReader(t *testing.T) {
	deps := newHistoryWorld(t)
	var out bytes.Buffer
	rt := lua.NewReader(deps.ReadDeps, &out, lua.WithCapabilities(lua.Capabilities{Tokens: []string{"crm"}}))
	defer rt.Close()
	if err := rt.RunString(`rela.output(type(rela.oauth))`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nil") {
		t.Fatalf("reader runtime has rela.oauth: %q", out.String())
	}
}

// The token store key never reaches a script, not even under AllSecrets.
func TestCapabilities_TokenKeyNeverExposed(t *testing.T) {
	for _, caps := range []lua.Capabilities{
		lua.TrustedCapabilities(),
		{Secrets: []string{"token_key", "api"}},
	} {
		if caps.AllowsSecret("token_key") {
			t.Errorf("%+v allows token_key", caps)
		}
	}
	if lua.DocsCapabilities().AllowsToken("crm") {
		t.Error("the docs build must hold no token connection")
	}
	if !lua.TrustedCapabilities().AllowsToken("crm") {
		t.Error("the operator shell holds every connection")
	}
}
