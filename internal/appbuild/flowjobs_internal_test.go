package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func TestWebhookOutcome(t *testing.T) {
	for code, want := range map[int]string{
		200: "ok", 204: "ok", 301: "permanent", 400: "permanent", 404: "permanent",
		429: "retry", 500: "retry", 503: "retry",
	} {
		err := webhookOutcome(code)
		got := "retry"
		switch {
		case err == nil:
			got = "ok"
		case errors.Is(err, flowengine.ErrPermanent):
			got = "permanent"
		}
		require.Equal(t, want, got, "status %d", code)
	}
}

func TestWebhookSignature(t *testing.T) {
	// HMAC-SHA256("key", "1.{}"), computed independently.
	require.Equal(t, "1ba6b8171186efc613e8bcc0cbdab2748f24984d7c5a84faa2637afa0e40d224",
		webhookSignature("key", "1", []byte("{}")))
}

func TestAttemptPayload_RoundTrips(t *testing.T) {
	ref := flowengine.AttemptRef{Run: "RUN-1", Branch: "it", Generation: 7, Seq: 3}
	p, err := attemptPayload(ref)
	require.NoError(t, err)
	got, err := attemptRef(p)
	require.NoError(t, err)
	require.Equal(t, ref, got)

	_, err = attemptRef(map[string]any{"seq": "three"})
	require.Error(t, err)
}

func TestScriptOutcome(t *testing.T) {
	require.NoError(t, scriptOutcome(nil))
	plain := errors.New("boom")
	require.NotErrorIs(t, scriptOutcome(plain), flowengine.ErrPermanent)
	require.NotErrorIs(t, scriptOutcome(&lua.ScriptError{LuaMessage: "x"}), flowengine.ErrPermanent)
	require.ErrorIs(t, scriptOutcome(&lua.ScriptError{LuaMessage: "x", Permanent: true}), flowengine.ErrPermanent)
}

func TestFlowActionUsable(t *testing.T) {
	ok := dataentryconfig.Action{Script: "notify.lua", Permission: "p"}
	require.NoError(t, flowActionUsable("a", ok, ""))
	tests := map[string]dataentryconfig.Action{
		"no script":    {},
		"set":          {Script: "x.lua", Set: map[string]string{"status": "done"}},
		"request":      {Script: "x.lua", Request: &dataentryconfig.ActionRequest{}},
		"available_on": {Script: "x.lua", AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"person"}}},
		"bypass":       {Script: "x.lua", AllowACLBypass: metamodel.ACLBypassWrite},
		"elevated":     {Script: "x.lua", ElevatedUpdates: map[string][]string{"person": {"stage"}}},
	}
	for name, a := range tests {
		t.Run(name, func(t *testing.T) {
			require.Error(t, flowActionUsable("a", a, ""))
		})
	}
}

// mapLoader is a config.Loader over a map; a missing name is absent.
type mapLoader map[string]string

func (m mapLoader) Load(_ context.Context, name string) ([]byte, error) {
	v, ok := m[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(v), nil
}

func (m mapLoader) List(context.Context, string) ([]string, error) { return nil, nil }

func TestLoadActions(t *testing.T) {
	ctx := context.Background()
	got, err := loadActions(ctx, mapLoader{})
	require.NoError(t, err)
	require.Empty(t, got, "an absent data-entry.yaml declares no action")

	got, err = loadActions(ctx, mapLoader{dataentryconfig.ConfigFile: "actions:\n  notify: { script: n.lua }\n"})
	require.NoError(t, err)
	require.Equal(t, "n.lua", got["notify"].Script)

	_, err = loadActions(ctx, mapLoader{dataentryconfig.ConfigFile: "actions: ["})
	require.Error(t, err)
}

// secretsHost serves one set of global secrets.
type secretsHost map[string]string

func (h secretsHost) File(string) ([]byte, error)               { return nil, fs.ErrNotExist }
func (h secretsHost) Secrets(string) (map[string]string, error) { return h, nil }
func (h secretsHost) Path() string                              { return "" }

func webhookCall(url, secret string) flowengine.JobCall {
	return flowengine.JobCall{
		Flow: "f", Run: "RUN-1", State: "send", Subject: entity.Ref{ID: "PER-1"}, RunAs: "bot", Attempt: 1,
		Key: "RUN-1/send.1/1", StepKey: "RUN-1/send.1",
		Job: flowcfg.Job{Target: flowcfg.JobWebhook, URL: url, Secret: secret},
	}
}

func webhookRunner(host lua.HostConfig) *flowJobRunner {
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	return &flowJobRunner{host: host, client: client}
}

func TestPost_DoesNotFollowARedirect(t *testing.T) {
	followed := false
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/elsewhere", func(http.ResponseWriter, *http.Request) { followed = true })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := webhookRunner(nil).post(context.Background(), webhookCall(srv.URL+"/hook", ""))
	require.ErrorIs(t, err, flowengine.ErrPermanent)
	require.False(t, followed, "the signed body must not reach a URL flows.yaml does not name")
}

func TestPost_NetworkErrorRetries(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	err := webhookRunner(nil).post(context.Background(), webhookCall(url+"/hook?token=s3cret", ""))
	require.Error(t, err)
	require.NotErrorIs(t, err, flowengine.ErrPermanent)
	require.NotContains(t, err.Error(), "s3cret", "the error names the host, not the query")
}

func TestPost_MissingSecretIsPermanent(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	for name, host := range map[string]lua.HostConfig{"no host": nil, "unset": secretsHost{"other": "x"}} {
		t.Run(name, func(t *testing.T) {
			err := webhookRunner(host).post(context.Background(), webhookCall(srv.URL, "hook_key"))
			require.ErrorIs(t, err, flowengine.ErrPermanent)
		})
	}
	require.False(t, called, "nothing is sent unsigned")
}

func TestRunJob_RefusesRelaIdentity(t *testing.T) {
	r := &flowJobRunner{as: func(ctx context.Context, _ string) (context.Context, error) {
		t.Fatal("no identity is bound")
		return ctx, nil
	}}
	for _, as := range []string{"", principal.UserFlow} {
		err := r.RunJob(context.Background(), flowengine.JobCall{RunAs: as})
		require.ErrorIs(t, err, flowengine.ErrPermanent, "run_as %q", as)
	}
}

// An llm step hands the permission check to the runner: a refusal there
// fails the step for good, and a provider failure is retried.
func TestRunAction_LLMStep(t *testing.T) {
	cfg := mapLoader{dataentryconfig.ConfigFile: `actions:
  triage:
    permission: ticket.triage
    available_on: { entity_types: [ticket] }
    llm: { prompt: p, apply: auto, outputs: { set: [priority] } }
`}
	call := flowengine.JobCall{RunAs: "bot", Subject: entity.Ref{ID: "TKT-1"},
		Job: flowcfg.Job{Target: flowcfg.JobAction, Action: "triage"}}
	for _, tc := range []struct {
		name      string
		runner    *fakeLLMRunner
		permanent bool
	}{
		{"refused", &fakeLLMRunner{err: errors.New("run_as lacks ticket.triage")}, true},
		{"provider down", &fakeLLMRunner{retry: true, err: errors.New("503")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &llmActionHolder{}
			h.r.Store(&llmActionRunnerBox{tc.runner})
			err := (&flowJobRunner{cfg: cfg, llm: h}).runAction(context.Background(), call)
			require.Error(t, err)
			require.Equal(t, tc.permanent, errors.Is(err, flowengine.ErrPermanent))
			require.Equal(t, 1, tc.runner.runs)
		})
	}
}

// scriptSecretsHost serves global secrets and, for one script, its own.
type scriptSecretsHost struct {
	global map[string]string
	script string
	own    map[string]string
}

func (h scriptSecretsHost) File(string) ([]byte, error) { return nil, fs.ErrNotExist }
func (h scriptSecretsHost) Path() string                { return "" }
func (h scriptSecretsHost) Secrets(path string) (map[string]string, error) {
	if path == h.script {
		return h.own, nil
	}
	return h.global, nil
}

// TestRedact pins BUG-53GP5F5: the error a job returns names no secret
// value the job's script or the project holds, and keeps its chain.
func TestRedact(t *testing.T) {
	host := scriptSecretsHost{
		global: map[string]string{"api": "glob4l-key", "short": "ab"},
		script: "flows/x.lua",
		own:    map[string]string{"api": "scr1pt-key", "long": "scr1pt-key-and-more"},
	}
	r := &flowJobRunner{host: host}
	err := flowengine.Permanent(errors.New("401 for glob4l-key, scr1pt-key-and-more and scr1pt-key; ab"))
	got := r.redact("flows/x.lua", err)
	require.Equal(t, "flowengine: permanent job failure: 401 for [secret], [secret] and [secret]; ab", got.Error(),
		"a short value is kept: it would match ordinary words")
	require.ErrorIs(t, got, flowengine.ErrPermanent)

	require.Contains(t, r.redact("", err).Error(), "scr1pt-key", "without its script, a script's own secret is unknown")
	require.NoError(t, r.redact("flows/x.lua", nil))
	plain := errors.New("no secret here")
	require.Same(t, plain, r.redact("flows/x.lua", plain), "an error without a secret passes unchanged")
	require.Same(t, plain, (&flowJobRunner{}).redact("", plain))

	encoded := &flowJobRunner{host: scriptSecretsHost{global: map[string]string{"tok": "a+b/c=d \"e\""}, script: "x"}}
	got = encoded.redact("", fmt.Errorf("GET /x?k=%s: %q, path %s", url.QueryEscape("a+b/c=d \"e\""),
		"a+b/c=d \"e\"", url.PathEscape("a+b/c=d \"e\"")))
	require.NotContains(t, got.Error(), "a+b", "an escaped or quoted value is a secret too")
	require.NotContains(t, got.Error(), "a%2Bb")

	withheld := &flowJobRunner{host: withheldHost{err: lua.ErrHostConfigWithheld}}
	require.Same(t, plain, withheld.redact("", plain), "withheld secrets are secrets the script could not read")
	broken := &flowJobRunner{host: failingSecretsHost{}}
	got = broken.redact("", errors.Join(flowengine.ErrPermanent, errors.New("glob4l-key")))
	require.Equal(t, errSecretsUnread, got.Error(), "unreadable secrets withhold the error")
	require.ErrorIs(t, got, flowengine.ErrPermanent)
}

// failingSecretsHost cannot read its secrets.
type failingSecretsHost struct{ scriptSecretsHost }

func (failingSecretsHost) Secrets(string) (map[string]string, error) {
	return nil, errors.New("secrets.yaml: permission denied")
}
