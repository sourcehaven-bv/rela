package mail_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/mail/mailtest"
)

// scriptProject writes a project with a .rela dir, a send script and an
// optional secrets.yaml, and returns the .rela directory.
func scriptProject(tb testing.TB, scriptRelPath, scriptBody, secretsYAML string) string {
	tb.Helper()

	root := tb.TempDir()
	relaDir := filepath.Join(root, ".rela")
	require.NoError(tb, os.MkdirAll(relaDir, 0o750))

	full := filepath.Join(root, scriptRelPath)
	require.NoError(tb, os.MkdirAll(filepath.Dir(full), 0o750))
	require.NoError(tb, os.WriteFile(full, []byte(scriptBody), 0o600))

	if secretsYAML != "" {
		require.NoError(tb, os.WriteFile(
			filepath.Join(relaDir, "secrets.yaml"), []byte(secretsYAML), 0o600))
	}
	return relaDir
}

// scriptConfig returns a valid script-transport config.
func scriptConfig(relaDir, scriptRelPath string, caps mail.ScriptCapabilities) *mail.Config {
	cfg := &mail.Config{
		Transport:    mail.TransportScript,
		Script:       scriptRelPath,
		From:         "from@example.com",
		FromName:     "From Name",
		Capabilities: caps,
	}
	return cfg.WithRelaDir(relaDir)
}

// newJSONStub records requests a send script makes and answers 200.
//
// Always 200: the non-2xx path is exercised by the tests that need it with
// purpose-built handlers, because what they assert is the SCRIPT's reaction to
// a specific status and body, not a status this helper happened to be handed.
func newJSONStub(tb testing.TB) (srv *httptest.Server,
	requests func() []*http.Request, bodies func() []string) {
	tb.Helper()

	var mu sync.Mutex
	var reqs []*http.Request
	var gotBodies []string

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)

		mu.Lock()
		//nolint:contextcheck // r.Clone needs a ctx and the request's own is
		// cancelled the moment the handler returns; the clone is inspected
		// after that, so a detached background ctx is the correct choice.
		reqs = append(reqs, r.Clone(context.Background()))
		gotBodies = append(gotBodies, buf.String())
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	tb.Cleanup(srv.Close)

	requests = func() []*http.Request {
		mu.Lock()
		defer mu.Unlock()
		out := make([]*http.Request, len(reqs))
		copy(out, reqs)
		return out
	}
	bodies = func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(gotBodies))
		copy(out, gotBodies)
		return out
	}
	return srv, requests, bodies
}

// TestScriptSender_Delivers covers AC2: a script ships a rendered message to
// an httptest.Server.
func TestScriptSender_Delivers(t *testing.T) {
	t.Parallel()

	srv, requests, bodies := newJSONStub(t)

	script := `
local resp, err = http.request({
  url = rela.secrets.endpoint,
  method = "POST",
  headers = { ["Authorization"] = "Bearer " .. rela.secrets.api_key },
  body = rela.json.encode({
    to = message.to[1].email,
    subject = message.subject,
    html = message.html,
    text = message.text,
    from = message.from.email,
  }),
})
if err then error("send failed: " .. err.message) end
if resp.status_code ~= 200 then error("bad status " .. resp.status_code) end
`
	relaDir := scriptProject(t, "mail/send.lua", script,
		"api_key: k-123\nendpoint: "+srv.URL+"/send\n")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/send.lua",
		mail.ScriptCapabilities{HTTP: true, Secrets: []string{"api_key", "endpoint"}}))
	require.NoError(t, err)

	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	reqs := requests()
	require.Len(t, reqs, 1)
	require.Equal(t, "/send", reqs[0].URL.Path)
	require.Equal(t, "Bearer k-123", reqs[0].Header.Get("Authorization"))
	require.Contains(t, bodies()[0], `"subject":"Subject"`)
	require.Contains(t, bodies()[0], `"from":"from@example.com"`)
}

// TestScriptSender_NoGraphAccess covers AC4 — the security property this
// ticket's whole design rests on, asserted rather than claimed in prose.
//
// The runtime is built with a zero ReadDeps and is a READER, so every graph
// binding either raises or is absent. The script below probes each one and
// reports what it found; the test fails if any of them worked.
func TestScriptSender_NoGraphAccess(t *testing.T) {
	t.Parallel()

	srv, _, bodies := newJSONStub(t)

	// pcall each binding so the script survives a raise and can report. A
	// binding that RETURNED data instead of raising is the failure this is
	// looking for, and it must be distinguishable from a raise.
	script := `
local findings = {}

local function probe(name, fn)
  if fn == nil then
    findings[name] = "absent"
    return
  end
  local ok, res = pcall(fn)
  if ok then
    findings[name] = "RETURNED"
  else
    findings[name] = "raised"
  end
end

probe("get_entity",      rela.get_entity      and function() return rela.get_entity("x") end)
probe("list_entities",   rela.list_entities   and function() return rela.list_entities("person") end)
probe("search",          rela.search          and function() return rela.search("x") end)
probe("get_relations",   rela.get_relations   and function() return rela.get_relations("x") end)
probe("trace_from",      rela.trace_from      and function() return rela.trace_from("x") end)

-- Mutation bindings must be ABSENT, not merely guarded: a reader runtime
-- never registers them at all.
findings.create_entity = rela.create_entity == nil and "absent" or "PRESENT"
findings.update_entity = rela.update_entity == nil and "absent" or "PRESENT"
findings.delete_entity = rela.delete_entity == nil and "absent" or "PRESENT"
findings.write_file    = rela.write_file    == nil and "absent" or "PRESENT"
findings.bypass_acl    = rela.bypass_acl    == nil and "absent" or "PRESENT"

-- An undeclared secret must be nil, not empty-string: a typo in a key name
-- has to surface at the use site rather than authenticate with "".
findings.undeclared_secret = rela.secrets.database_dsn == nil and "nil" or "PRESENT"
findings.declared_secret   = rela.secrets.endpoint

http.request({
  url = rela.secrets.endpoint,
  method = "POST",
  body = rela.json.encode(findings),
})
`
	relaDir := scriptProject(t, "mail/probe.lua", script,
		"endpoint: "+srv.URL+"/probe\ndatabase_dsn: postgres://secret\n")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/probe.lua",
		mail.ScriptCapabilities{HTTP: true, Secrets: []string{"endpoint"}}))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	require.Len(t, bodies(), 1)
	report := bodies()[0]

	// No read binding may return data.
	require.NotContains(t, report, "RETURNED",
		"a graph read binding returned data to a send script: %s", report)
	// No write binding may exist.
	require.NotContains(t, report, "PRESENT",
		"a mutation binding or undeclared secret was reachable: %s", report)
	// And the declared secret DID arrive — otherwise every assertion above
	// would pass on a runtime that simply had nothing wired.
	require.Contains(t, report, srv.URL+"/probe")
}

// TestScriptSender_UndeclaredSecretIsAbsent isolates the secrets half of AC4
// so a regression names the right thing.
func TestScriptSender_UndeclaredSecretIsAbsent(t *testing.T) {
	t.Parallel()

	srv, _, bodies := newJSONStub(t)

	script := `
http.request({
  url = "` + srv.URL + `/x",
  method = "POST",
  body = rela.json.encode({
    granted = rela.secrets.mailgun_key or "MISSING",
    withheld = rela.secrets.database_dsn or "MISSING",
  }),
})
`
	relaDir := scriptProject(t, "mail/s.lua", script,
		"mailgun_key: key-granted\ndatabase_dsn: postgres://withheld\n")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/s.lua",
		mail.ScriptCapabilities{HTTP: true, Secrets: []string{"mailgun_key"}}))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	require.Len(t, bodies(), 1)
	require.Contains(t, bodies()[0], "key-granted")
	require.Contains(t, bodies()[0], `"withheld":"MISSING"`)
	require.NotContains(t, bodies()[0], "postgres://withheld")
}

// TestScriptSender_NoHTTPCapabilityIsLoud covers the negative half of AC8 for
// the transport: without `http: true` the global is absent and the send FAILS
// rather than silently succeeding without sending anything.
func TestScriptSender_NoHTTPCapabilityIsLoud(t *testing.T) {
	t.Parallel()

	relaDir := scriptProject(t, "mail/s.lua",
		`http.request({url = "https://example.com", method = "POST"})`, "")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/s.lua",
		mail.ScriptCapabilities{}))
	require.NoError(t, err)

	sendErr := sender.Send(context.Background(), validTestMessage())
	require.Error(t, sendErr, "a script denied http must not report success")
	require.ErrorContains(t, sendErr, "mail/s.lua")
}

// TestScriptCapabilities_ToLua pins the config→runtime translation for a send
// script, including the one capability an operator does not control.
//
// `mail` is hard-wired TRUE and there is deliberately no YAML key for it: this
// runtime IS the implementation of mail.send, so gating it on the capability
// system it implements would be circular (TKT-JVHSOZ). Everything else stays
// as narrow as the operator wrote it — `ai` and `write_file` are hard-wired
// off, and `http` is carried across only when asked for.
//
// Table-driven over both grant shapes because the interesting property is that
// three of the five fields do NOT vary with the input; a test on one row could
// not tell "hard-wired" from "happened to match".
func TestScriptCapabilities_ToLua(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   mail.ScriptCapabilities
		want lua.Capabilities
	}{
		{
			name: "empty grant still authorizes mail",
			in:   mail.ScriptCapabilities{},
			want: lua.Capabilities{Mail: true},
		},
		{
			name: "operator grant carries http and secrets, nothing more",
			in:   mail.ScriptCapabilities{HTTP: true, Secrets: []string{"mailgun_key"}},
			want: lua.Capabilities{HTTP: true, Mail: true, Secrets: []string{"mailgun_key"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, mail.ExportScriptCapsToLua(tc.in))
		})
	}
}

// TestScriptSender_FailureIsRetryableAndDoesNotDuplicate covers AC9: a script
// failure surfaces as an error the outbox retries, and each attempt sends the
// same message once.
func TestScriptSender_FailureIsRetryableAndDoesNotDuplicate(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var attempts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		mu.Lock()
		attempts = append(attempts, buf.String())
		n := len(attempts)
		mu.Unlock()

		// Fail the first attempt, accept the second.
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	script := `
local resp, err = http.request({
  url = "` + srv.URL + `/send",
  method = "POST",
  body = rela.json.encode({subject = message.subject}),
})
if err then error(err.message) end
if resp.status_code >= 300 then error("HTTP " .. resp.status_code) end
`
	relaDir := scriptProject(t, "mail/s.lua", script, "")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/s.lua",
		mail.ScriptCapabilities{HTTP: true}))
	require.NoError(t, err)

	msg := validTestMessage()

	// Attempt one fails and is reported as an error — which is what makes the
	// outbox retry it rather than mark it delivered.
	first := sender.Send(context.Background(), msg)
	require.Error(t, first)
	require.ErrorContains(t, first, "mail/s.lua")

	// Attempt two, with the SAME message value, succeeds. That the message is
	// unchanged is the property the outbox's retry depends on.
	require.NoError(t, sender.Send(context.Background(), msg))

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, attempts, 2, "one request per attempt, no duplicates")
	require.Equal(t, attempts[0], attempts[1], "a retry must send the same bytes")
}

// TestScriptSender_ValidatesBeforeRunning pins that the header-injection check
// runs in the transport, not in the script. A script cannot opt out of it by
// being written differently, and a rejected message never reaches the network.
func TestScriptSender_ValidatesBeforeRunning(t *testing.T) {
	t.Parallel()

	srv, requests, _ := newJSONStub(t)
	relaDir := scriptProject(t, "mail/s.lua",
		`http.request({url = "`+srv.URL+`/x", method = "POST"})`, "")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/s.lua",
		mail.ScriptCapabilities{HTTP: true}))
	require.NoError(t, err)

	bad := validTestMessage()
	bad.Subject = "Hi\r\nBcc: evil@example.com"
	require.Error(t, sender.Send(context.Background(), bad))
	require.Empty(t, requests(), "a rejected message must not reach the script")
}

// TestScriptSender_PerScriptSecretOverride pins the secrets-scope decision:
// the scope key is the CONFIGURED SCRIPT PATH, so an operator's ordinary
// `overrides:` block works with no mail-specific convention.
func TestScriptSender_PerScriptSecretOverride(t *testing.T) {
	t.Parallel()

	srv, _, bodies := newJSONStub(t)

	script := `
http.request({
  url = "` + srv.URL + `/x",
  method = "POST",
  body = rela.secrets.api_key,
})
`
	relaDir := scriptProject(t, "mail/send.lua", script,
		"api_key: global-key\noverrides:\n  mail/send.lua:\n    api_key: per-script-key\n")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/send.lua",
		mail.ScriptCapabilities{HTTP: true, Secrets: []string{"api_key"}}))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	require.Equal(t, []string{"per-script-key"}, bodies())
}

// TestScriptSender_Principal pins the audit identity half of the same
// decision: the runtime runs as the fixed system:mail principal, not as
// whoever happened to enqueue the message.
func TestScriptSender_Principal(t *testing.T) {
	t.Parallel()

	srv, _, bodies := newJSONStub(t)

	script := `
http.request({
  url = "` + srv.URL + `/x",
  method = "POST",
  body = rela.json.encode({
    user = rela.principal.user,
    tool = rela.principal.tool,
    rendered_for = message.rendered_for,
  }),
})
`
	relaDir := scriptProject(t, "mail/s.lua", script, "")
	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/s.lua",
		mail.ScriptCapabilities{HTTP: true}))
	require.NoError(t, err)

	msg := validTestMessage()
	msg.RenderedFor = "user:alice"
	require.NoError(t, sender.Send(context.Background(), msg))

	require.Len(t, bodies(), 1)
	require.Contains(t, bodies()[0], `"user":"`+mail.SendScriptPrincipal+`"`)
	require.Contains(t, bodies()[0], `"tool":"`+mail.SendScriptTool+`"`)
	// RenderedFor still names the identity whose visibility bounded the
	// CONTENT — a different question from who is doing the delivering, and the
	// reason the delivery identity did not need to inherit it.
	require.Contains(t, bodies()[0], `"rendered_for":"user:alice"`)
}

// TestScriptConfig_Validation table-tests the script-specific config rules.
// A send script runs with outbound HTTP and a credential, so a path escaping
// the project is refused at load rather than discovered later.
func TestScriptConfig_Validation(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		script  string
		wantErr string
	}{
		"missing":       {script: "", wantErr: "script is required"},
		"blank":         {script: "   ", wantErr: "script is required"},
		"absolute":      {script: "/etc/evil.lua", wantErr: "project-relative"},
		"traversal":     {script: "../../evil.lua", wantErr: "project-relative"},
		"not lua":       {script: "mail/send.sh", wantErr: ".lua file"},
		"ok":            {script: "mail/send.lua", wantErr: ""},
		"ok nested":     {script: "a/b/c/send.lua", wantErr: ""},
		"ok bare":       {script: "send.lua", wantErr: ""},
		"inner dotdot":  {script: "mail/../mail/send.lua", wantErr: ""},
		"traversal esc": {script: "mail/../../send.lua", wantErr: "project-relative"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := &mail.Config{
				Transport: mail.TransportScript,
				Script:    tc.script,
				From:      "a@example.com",
			}
			err := cfg.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestScriptSender_RejectsWrongTransport pins the constructor guard.
func TestScriptSender_RejectsWrongTransport(t *testing.T) {
	t.Parallel()

	_, err := mail.NewScriptSender(&mail.Config{
		Transport: mail.TransportMemory,
		From:      "a@example.com",
	})
	require.ErrorContains(t, err, "NewScriptSender given transport")
}

// TestConformance_Script covers AC3 for the script transport, against a stub —
// the SAME suite, unchanged, that memory, SMTP and http pass.
func TestConformance_Script(t *testing.T) {
	mailtest.RunAll(t, func(tb testing.TB) (mail.Sender, mailtest.Sent) {
		tb.Helper()

		srv, _, bodies := newJSONStub(tb)

		// A minimal but honest send script: it forwards the whole message as
		// JSON, which is what lets the suite read back what was transmitted.
		script := `
local to = {}
for _, r in ipairs(message.to) do
  table.insert(to, {email = r.email, name = r.name})
end
local resp, err = http.request({
  url = "` + srv.URL + `/send",
  method = "POST",
  body = rela.json.encode({
    to = to,
    subject = message.subject,
    html = message.html,
    text = message.text,
  }),
})
if err then error(err.message) end
if resp.status_code >= 300 then error("HTTP " .. resp.status_code) end
`
		relaDir := scriptProject(tb, "mail/send.lua", script, "")
		sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/send.lua",
			mail.ScriptCapabilities{HTTP: true}))
		require.NoError(tb, err)

		sent := func() []mail.Message {
			var out []mail.Message
			for _, body := range bodies() {
				out = append(out, messageFromScriptBody(tb, body))
			}
			return out
		}
		return sender, sent
	})
}

// messageFromScriptBody reconstructs a Message from what the send script
// posted to the stub.
func messageFromScriptBody(tb testing.TB, body string) mail.Message {
	tb.Helper()

	var payload struct {
		To      []struct{ Email, Name string } `json:"to"`
		Subject string                         `json:"subject"`
		HTML    string                         `json:"html"`
		Text    string                         `json:"text"`
	}
	require.NoError(tb, json.Unmarshal([]byte(body), &payload))

	m := mail.Message{Subject: payload.Subject}
	for _, a := range payload.To {
		m.To = append(m.To, mail.Address{Email: a.Email, Name: a.Name})
	}
	if payload.HTML != "" {
		m.HTML = []byte(payload.HTML)
	}
	if payload.Text != "" {
		m.Text = []byte(payload.Text)
	}
	return m
}

// --- shipped example scripts ----------------------------------------------

// TestExampleMailgunScript covers AC5: the SHIPPED examples/mail/mailgun.lua
// posts multipart/form-data with Basic auth, against a stub asserting
// Mailgun's exact field names.
//
// It runs the file from examples/ rather than a copy inlined here. A copy
// would keep passing after the shipped script rotted, which is the one failure
// this test exists to catch.
func TestExampleMailgunScript(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var (
		gotPath   string
		gotUser   string
		gotPass   string
		gotOK     bool
		gotFields map[string][]string
		gotCT     string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		ct := r.Header.Get("Content-Type")

		fields := map[string][]string{}
		mediaType, params, err := mime.ParseMediaType(ct)
		if err == nil && strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, perr := mr.NextPart()
				if perr != nil {
					break
				}
				var buf bytes.Buffer
				_, _ = buf.ReadFrom(part)
				fields[part.FormName()] = append(fields[part.FormName()], buf.String())
			}
		}

		mu.Lock()
		gotPath, gotUser, gotPass, gotOK, gotFields, gotCT = r.URL.Path, user, pass, ok, fields, ct
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"<x@mg>"}`))
	}))
	t.Cleanup(srv.Close)

	relaDir := exampleProject(t, "mailgun.lua",
		"mailgun_key: key-abc123\n"+
			"mailgun_domain: mg.example.com\n"+
			"mailgun_base_url: "+srv.URL+"/v3\n")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/mailgun.lua",
		mail.ScriptCapabilities{
			HTTP:    true,
			Secrets: []string{"mailgun_key", "mailgun_domain", "mailgun_base_url"},
		}))
	require.NoError(t, err)

	require.NoError(t, sender.Send(context.Background(), mail.Message{
		To: []mail.Address{
			{Email: "a@example.com", Name: "Ay"},
			{Email: "b@example.com"},
		},
		Subject:     "Hello",
		HTML:        []byte("<p>hi</p>"),
		Text:        []byte("hi"),
		RenderedFor: "user:test",
	}))

	mu.Lock()
	defer mu.Unlock()

	require.Equal(t, "/v3/mg.example.com/messages", gotPath)

	// HTTP Basic with api:KEY — Mailgun's scheme, and the reason basic_auth
	// exists as a primitive rather than as a hand-built header.
	require.True(t, gotOK, "request must carry HTTP Basic auth")
	require.Equal(t, "api", gotUser)
	require.Equal(t, "key-abc123", gotPass)

	require.True(t, strings.HasPrefix(gotCT, "multipart/form-data;"),
		"Mailgun takes multipart/form-data, got %q", gotCT)

	// Mailgun's exact field names.
	require.Equal(t, []string{"From Name <from@example.com>"}, gotFields["from"])
	require.Equal(t, []string{"Hello"}, gotFields["subject"])
	require.Equal(t, []string{"<p>hi</p>"}, gotFields["html"])
	require.Equal(t, []string{"hi"}, gotFields["text"])

	// Repeated `to` — the shape no map-keyed form encoding could express, and
	// the reason http's `form` accepts the positional pair list.
	require.Equal(t, []string{"Ay <a@example.com>", "b@example.com"}, gotFields["to"])
}

// TestExamplePostmarkScript exercises the shipped Postmark example against a
// stub asserting its custom auth header and capitalized field names.
func TestExamplePostmarkScript(t *testing.T) {
	t.Parallel()

	srv, requests, bodies := newJSONStub(t)
	relaDir := exampleProject(t, "postmark.lua",
		"postmark_token: pm-token\npostmark_base_url: "+srv.URL+"\n")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/postmark.lua",
		mail.ScriptCapabilities{HTTP: true, Secrets: []string{"postmark_token", "postmark_base_url"}}))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	reqs := requests()
	require.Len(t, reqs, 1)
	require.Equal(t, "/email", reqs[0].URL.Path)
	require.Equal(t, "pm-token", reqs[0].Header.Get("X-Postmark-Server-Token"))

	// Decoded rather than substring-matched: rela.json.encode HTML-escapes
	// angle brackets, so asserting on the raw bytes would pin an encoder
	// detail instead of the provider field names this test is about.
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(bodies()[0]), &body))
	require.Equal(t, "From Name <from@example.com>", body["From"])
	require.Equal(t, "to@example.com", body["To"])
	require.Equal(t, "Subject", body["Subject"])
	require.Equal(t, "<p>hello</p>", body["HtmlBody"])
	require.Equal(t, "hello", body["TextBody"])
}

// TestExampleResendScript exercises the shipped Resend example.
func TestExampleResendScript(t *testing.T) {
	t.Parallel()

	srv, requests, bodies := newJSONStub(t)
	relaDir := exampleProject(t, "resend.lua",
		"resend_key: re-key\nresend_base_url: "+srv.URL+"\n")

	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/resend.lua",
		mail.ScriptCapabilities{HTTP: true, Secrets: []string{"resend_key", "resend_base_url"}}))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	reqs := requests()
	require.Len(t, reqs, 1)
	require.Equal(t, "/emails", reqs[0].URL.Path)
	require.Equal(t, "Bearer re-key", reqs[0].Header.Get("Authorization"))

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(bodies()[0]), &body))
	require.Equal(t, "From Name <from@example.com>", body["from"])
	require.Equal(t, []any{"to@example.com"}, body["to"])
	require.Equal(t, "<p>hello</p>", body["html"])
	require.Equal(t, "hello", body["text"])
}

// TestExampleScriptFailsOnBadStatus pins that the shipped examples do not
// swallow a non-2xx. Returning normally would tell rela the mail was
// delivered when the provider rejected it — a silent mail loss.
func TestExampleScriptFailsOnBadStatus(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"mailgun.lua", "postmark.lua", "resend.lua"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"nope"}`))
			}))
			t.Cleanup(srv.Close)

			relaDir := exampleProject(t, name,
				"mailgun_key: k\nmailgun_domain: d\nmailgun_base_url: "+srv.URL+"/v3\n"+
					"postmark_token: t\npostmark_base_url: "+srv.URL+"\n"+
					"resend_key: r\nresend_base_url: "+srv.URL+"\n")

			sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/"+name,
				mail.ScriptCapabilities{HTTP: true, Secrets: []string{
					"mailgun_key", "mailgun_domain", "mailgun_base_url",
					"postmark_token", "postmark_base_url",
					"resend_key", "resend_base_url",
				}}))
			require.NoError(t, err)

			require.Error(t, sender.Send(context.Background(), validTestMessage()),
				"a 400 from the provider must not be reported as delivered")
		})
	}
}

// exampleProject copies a SHIPPED examples/mail script into a temp project.
//
// Copying the real file, rather than inlining a lookalike, is what makes these
// tests catch rot in what we actually ship.
func exampleProject(tb testing.TB, name, secretsYAML string) string {
	tb.Helper()

	src := filepath.Join("..", "..", "examples", "mail", name)
	body, err := os.ReadFile(src)
	require.NoError(tb, err, "shipped example %s must exist", src)

	return scriptProject(tb, filepath.Join("mail", name), string(body), secretsYAML)
}

// TestSenderFor covers the transport switch, which every wiring site now
// shares. It was previously duplicated in internal/appbuild, and a two-copy
// switch over a closed set is how a transport gets wired in one place and not
// the other.
func TestSenderFor(t *testing.T) {
	t.Parallel()

	t.Run("memory", func(t *testing.T) {
		t.Parallel()
		s, err := mail.SenderFor(&mail.Config{Transport: mail.TransportMemory, From: "f@e.com"})
		require.NoError(t, err)
		require.IsType(t, &mail.MemorySender{}, s)
	})

	t.Run("smtp", func(t *testing.T) {
		t.Parallel()
		s, err := mail.SenderFor(&mail.Config{
			Transport: mail.TransportSMTP, Host: "smtp.example.com", From: "f@e.com",
		})
		require.NoError(t, err)
		require.IsType(t, &mail.SMTPSender{}, s)
	})

	t.Run("http", func(t *testing.T) {
		t.Parallel()
		s, err := mail.SenderFor(&mail.Config{
			Transport: mail.TransportHTTP, AccountID: "acct", From: "f@e.com",
		})
		require.NoError(t, err)
		require.IsType(t, &mail.HTTPSender{}, s)
	})

	t.Run("script", func(t *testing.T) {
		t.Parallel()
		s, err := mail.SenderFor(&mail.Config{
			Transport: mail.TransportScript, Script: "mail/send.lua", From: "f@e.com",
		})
		require.NoError(t, err)
		require.IsType(t, &mail.ScriptSender{}, s)
	})

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()
		_, err := mail.SenderFor(&mail.Config{Transport: "pigeon", From: "f@e.com"})
		require.Error(t, err)
	})
}

// TestLoadLuaSender covers the mail.send wiring path: absent config is a
// normal absence, a broken config is an error, and a valid one yields a
// working adapter.
func TestLoadLuaSender(t *testing.T) {
	t.Parallel()

	t.Run("not configured is not an error", func(t *testing.T) {
		t.Parallel()
		sender, err := mail.LoadLuaSender(t.TempDir())
		require.NoError(t, err)
		require.Nil(t, sender, "absent mail.yaml must yield a nil sender, not an error")
	})

	t.Run("broken config is an error", func(t *testing.T) {
		t.Parallel()
		relaDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(relaDir, "mail.yaml"),
			[]byte("transport: pigeon\nfrom: f@e.com\n"), 0o600))

		_, err := mail.LoadLuaSender(relaDir)
		require.ErrorContains(t, err, "pigeon")
	})

	t.Run("valid config yields a sender", func(t *testing.T) {
		t.Parallel()
		relaDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(relaDir, "mail.yaml"),
			[]byte("transport: memory\nfrom: f@e.com\n"), 0o600))

		sender, err := mail.LoadLuaSender(relaDir)
		require.NoError(t, err)
		require.NotNil(t, sender)
	})
}

// TestLuaSender_StampsSystemPrincipal pins that a script-originated send
// records the system principal in RenderedFor rather than leaving it empty.
//
// Empty would be indistinguishable from a declarative render that forgot to
// set it. Naming the system principal says truthfully "no user's visibility
// bounded this content" — a script assembled its own body.
func TestLuaSender_StampsSystemPrincipal(t *testing.T) {
	t.Parallel()

	rec := mail.NewMemorySender(0)
	adapter, err := mail.NewLuaSender(rec, &mail.Config{
		Transport: mail.TransportMemory, From: "f@e.com",
	})
	require.NoError(t, err)

	require.NoError(t, adapter.SendMail(context.Background(), lua.MailMessage{
		To:      []string{"a@example.com"},
		Subject: "S",
		Text:    "t",
	}))

	got := rec.Messages()
	require.Len(t, got, 1)
	require.Equal(t, mail.SendScriptPrincipal, got[0].RenderedFor)
	require.Equal(t, "a@example.com", got[0].To[0].Email)
}

// TestLuaSender_RejectsNil pins the constructor guard. A LuaSender over
// nothing would report success for mail that was never sent.
func TestLuaSender_RejectsNil(t *testing.T) {
	t.Parallel()

	_, err := mail.NewLuaSender(nil, &mail.Config{Transport: mail.TransportMemory, From: "f@e.com"})
	require.ErrorContains(t, err, "nil sender")

	_, err = mail.NewLuaSender(mail.NewMemorySender(0), nil)
	require.ErrorContains(t, err, "nil config")
}

// TestLuaSender_ValidationStillApplies pins that a script-originated send goes
// through the same Message.Validate as every other caller — mail.send is not a
// way around the header-injection check.
func TestLuaSender_ValidationStillApplies(t *testing.T) {
	t.Parallel()

	rec := mail.NewMemorySender(0)
	adapter, err := mail.NewLuaSender(rec, &mail.Config{
		Transport: mail.TransportMemory, From: "f@e.com",
	})
	require.NoError(t, err)

	require.Error(t, adapter.SendMail(context.Background(), lua.MailMessage{
		To:      []string{"a@example.com"},
		Subject: "Hi\r\nBcc: evil@example.com",
		Text:    "t",
	}))
	require.Empty(t, rec.Messages())
}

// TestScriptSender_InnerRuntimeHasNoMailSender pins that a send script cannot
// recurse: the runtime a ScriptSender builds has no mail sender wired, so a
// script calling mail.send gets not_configured rather than an unbounded chain
// of runtimes.
func TestScriptSender_InnerRuntimeHasNoMailSender(t *testing.T) {
	t.Parallel()

	srv, _, bodies := newJSONStub(t)

	script := `
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
http.request({
  url = "` + srv.URL + `/x",
  method = "POST",
  body = (ok == nil and err.kind or "SENT"),
})
`
	relaDir := scriptProject(t, "mail/s.lua", script, "")
	sender, err := mail.NewScriptSender(scriptConfig(relaDir, "mail/s.lua",
		mail.ScriptCapabilities{HTTP: true}))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	require.Equal(t, []string{"not_configured"}, bodies())
}
