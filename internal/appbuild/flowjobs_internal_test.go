package appbuild

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
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
	require.NoError(t, flowActionUsable("a", ok))
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
			require.Error(t, flowActionUsable("a", a))
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
