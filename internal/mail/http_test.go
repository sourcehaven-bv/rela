package mail_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/mail/mailtest"
)

// capturedRequest is what the APIv2 stub recorded.
type capturedRequest struct {
	Method string
	Path   string
	Auth   string
	Body   map[string]any
}

// newAPIStub returns an httptest.Server standing in for SimpleMailService
// APIv2, plus an accessor for what it received.
//
// It asserts nothing itself: the tests below assert, so a failure names the
// property that broke rather than "the stub was unhappy".
func newAPIStub(t *testing.T, status int, respBody string) (srv *httptest.Server, received func() []capturedRequest) {
	t.Helper()

	var mu sync.Mutex
	var got []capturedRequest

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		got = append(got, capturedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Auth:   r.Header.Get("Authorization"),
			Body:   body,
		})
		mu.Unlock()

		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)

	received = func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()
		out := make([]capturedRequest, len(got))
		copy(out, got)
		return out
	}
	return srv, received
}

// writeSecrets writes a .rela/secrets.yaml and returns the .rela directory.
func writeSecrets(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	relaDir := filepath.Join(dir, ".rela")
	require.NoError(t, os.MkdirAll(relaDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(relaDir, "secrets.yaml"), []byte(body), 0o600))
	return relaDir
}

// httpTestConfig returns a valid http-transport config pointed at relaDir.
func httpTestConfig(relaDir string) *mail.Config {
	cfg := &mail.Config{
		Transport: mail.TransportHTTP,
		AccountID: "acct-123",
		From:      "from@example.com",
		FromName:  "From Name",
	}
	return cfg.WithRelaDir(relaDir)
}

// TestHTTPSender_APIv2Contract covers AC1: the exact path, the bearer header,
// and the documented body field names.
//
// Asserted against the PROVIDER's field names, deliberately spelled out here
// rather than derived from the Go struct tags. A test that read the tags would
// pass just as happily after someone renamed a field for internal tidiness,
// which is the exact change that breaks delivery in production.
func TestHTTPSender_APIv2Contract(t *testing.T) {
	t.Parallel()

	srv, received := newAPIStub(t, http.StatusAccepted, `{"id":"msg-1"}`)
	relaDir := writeSecrets(t, "mail_api_token: tok-secret\n")

	sender, err := mail.NewHTTPSender(httpTestConfig(relaDir), mail.WithHTTPBaseURL(srv.URL+"/v2"))
	require.NoError(t, err)

	msg := mail.Message{
		To: []mail.Address{
			{Email: "a@example.com", Name: "Ay"},
			{Email: "b@example.com"},
		},
		Subject:     "Hello",
		HTML:        []byte("<p>hi</p>"),
		Text:        []byte("hi"),
		RenderedFor: "user:test",
	}
	require.NoError(t, sender.Send(context.Background(), msg))

	got := received()
	require.Len(t, got, 1)
	req := got[0]

	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "/v2/accounts/acct-123/messages", req.Path)
	require.Equal(t, "Bearer tok-secret", req.Auth)

	// from{email,name}
	from, ok := req.Body["from"].(map[string]any)
	require.True(t, ok, "body must carry a `from` object")
	require.Equal(t, "from@example.com", from["email"])
	require.Equal(t, "From Name", from["name"])

	// recipients[]{email,name}
	recipients, ok := req.Body["recipients"].([]any)
	require.True(t, ok, "body must carry a `recipients` array")
	require.Len(t, recipients, 2)
	first, ok := recipients[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "a@example.com", first["email"])
	require.Equal(t, "Ay", first["name"])

	require.Equal(t, "Hello", req.Body["subject"])
	require.Equal(t, "<p>hi</p>", req.Body["html_content"])
	require.Equal(t, "hi", req.Body["text_content"])
}

// TestHTTPSender_InlineImagesAsAttachments pins the attachments[] shape,
// including base64=true — the field the provider uses to decide whether to
// decode.
func TestHTTPSender_InlineImagesAsAttachments(t *testing.T) {
	t.Parallel()

	srv, received := newAPIStub(t, http.StatusOK, `{}`)
	relaDir := writeSecrets(t, "mail_api_token: tok\n")

	sender, err := mail.NewHTTPSender(httpTestConfig(relaDir), mail.WithHTTPBaseURL(srv.URL+"/v2"))
	require.NoError(t, err)

	require.NoError(t, sender.Send(context.Background(), mail.Message{
		To:      []mail.Address{{Email: "a@example.com"}},
		Subject: "S",
		HTML:    []byte(`<img src="cid:logo">`),
		Text:    []byte("t"),
		InlineImages: []mail.InlineImage{
			{CID: "logo", ContentType: "image/png", Data: []byte{0x89, 0x50, 0x4e, 0x47}},
		},
	}))

	got := received()
	require.Len(t, got, 1)
	atts, ok := got[0].Body["attachments"].([]any)
	require.True(t, ok, "body must carry an `attachments` array")
	require.Len(t, atts, 1)
	att, ok := atts[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, att["base64"])
	require.Equal(t, "image/png", att["content_type"])
	require.Equal(t, "logo", att["file_name"])
	require.Equal(t, "iVBORw==", att["data"])
}

// TestHTTPSender_NoSubstitutions pins that rela never asks the provider to
// template. Bodies arrive rendered and sanitized; handing the provider a
// second pass would let it reinterpret content rela already made safe.
func TestHTTPSender_NoSubstitutions(t *testing.T) {
	t.Parallel()

	srv, received := newAPIStub(t, http.StatusOK, `{}`)
	relaDir := writeSecrets(t, "mail_api_token: tok\n")

	sender, err := mail.NewHTTPSender(httpTestConfig(relaDir), mail.WithHTTPBaseURL(srv.URL+"/v2"))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	got := received()
	require.Len(t, got, 1)
	require.NotContains(t, got[0].Body, "substitutions")
}

// TestHTTPSender_ErrorRedactsToken covers AC10 for the error path: a provider
// that echoes the credential back must not put it in rela's error, which goes
// straight to a log.
func TestHTTPSender_ErrorRedactsToken(t *testing.T) {
	t.Parallel()

	const token = "tok-super-secret"
	srv, _ := newAPIStub(t, http.StatusUnauthorized,
		`{"error":"invalid token `+token+`"}`)
	relaDir := writeSecrets(t, "mail_api_token: "+token+"\n")

	sender, err := mail.NewHTTPSender(httpTestConfig(relaDir), mail.WithHTTPBaseURL(srv.URL+"/v2"))
	require.NoError(t, err)

	sendErr := sender.Send(context.Background(), validTestMessage())
	require.Error(t, sendErr)
	require.NotContains(t, sendErr.Error(), token)
	require.Contains(t, sendErr.Error(), "<REDACTED>")
	// The status still has to survive redaction, or the operator learns
	// nothing from the error at all.
	require.Contains(t, sendErr.Error(), "401")
}

// TestHTTPSender_MissingToken names the two places an operator can put the
// credential rather than failing with a bare 401 from the provider.
func TestHTTPSender_MissingToken(t *testing.T) {
	t.Parallel()

	srv, received := newAPIStub(t, http.StatusOK, `{}`)
	sender, err := mail.NewHTTPSender(
		httpTestConfig(t.TempDir()), mail.WithHTTPBaseURL(srv.URL+"/v2"))
	require.NoError(t, err)

	sendErr := sender.Send(context.Background(), validTestMessage())
	require.ErrorContains(t, sendErr, "mail_api_token")
	require.ErrorContains(t, sendErr, "password_env")
	require.Empty(t, received(), "no request may be made without a credential")
}

// TestHTTPSender_TokenFromEnv covers the second credential source.
func TestHTTPSender_TokenFromEnv(t *testing.T) {
	srv, received := newAPIStub(t, http.StatusOK, `{}`)

	t.Setenv("RELA_TEST_MAIL_TOKEN", "env-token")
	cfg := httpTestConfig(t.TempDir())
	cfg.PasswordVar = "RELA_TEST_MAIL_TOKEN"

	sender, err := mail.NewHTTPSender(cfg, mail.WithHTTPBaseURL(srv.URL+"/v2"))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), validTestMessage()))

	got := received()
	require.Len(t, got, 1)
	require.Equal(t, "Bearer env-token", got[0].Auth)
}

// TestHTTPSender_RejectsWrongTransport pins that the constructor refuses a
// config for a different transport rather than sending SMTP config over HTTP.
func TestHTTPSender_RejectsWrongTransport(t *testing.T) {
	t.Parallel()

	_, err := mail.NewHTTPSender(&mail.Config{
		Transport: mail.TransportMemory,
		From:      "a@example.com",
	})
	require.ErrorContains(t, err, "NewHTTPSender given transport")
}

// TestHTTPConfig_Validation table-tests the http-specific config rules. The
// account ID becomes a URL path segment, so a value containing a separator
// could retarget the authenticated request.
func TestHTTPConfig_Validation(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		accountID string
		wantErr   string
	}{
		"missing":   {accountID: "", wantErr: "account_id is required"},
		"blank":     {accountID: "   ", wantErr: "account_id is required"},
		"slash":     {accountID: "a/b", wantErr: "plain identifier"},
		"traversal": {accountID: "..", wantErr: "plain identifier"},
		"query":     {accountID: "a?b", wantErr: "plain identifier"},
		"fragment":  {accountID: "a#b", wantErr: "plain identifier"},
		"ok":        {accountID: "acct-1", wantErr: ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := &mail.Config{
				Transport: mail.TransportHTTP,
				AccountID: tc.accountID,
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

// TestConformance_HTTP covers AC3 for the http transport, against the stub —
// the SAME suite, unchanged, that memory and SMTP pass.
func TestConformance_HTTP(t *testing.T) {
	mailtest.RunAll(t, func(tb testing.TB) (mail.Sender, mailtest.Sent) {
		tb.Helper()
		st, ok := tb.(*testing.T)
		require.True(tb, ok)

		srv, received := newAPIStub(st, http.StatusOK, `{}`)
		relaDir := writeSecrets(st, "mail_api_token: tok\n")

		sender, err := mail.NewHTTPSender(httpTestConfig(relaDir), mail.WithHTTPBaseURL(srv.URL+"/v2"))
		require.NoError(tb, err)

		// Reconstruct Messages from what crossed the wire. Reading the request
		// bodies back, rather than recording in the sender, is what makes this
		// a test of transmission rather than of bookkeeping.
		sent := func() []mail.Message {
			var out []mail.Message
			for _, req := range received() {
				out = append(out, messageFromAPIRequest(req))
			}
			return out
		}
		return sender, sent
	})
}

// messageFromAPIRequest reconstructs a Message from a captured APIv2 body.
func messageFromAPIRequest(req capturedRequest) mail.Message {
	m := mail.Message{}
	if recipients, ok := req.Body["recipients"].([]any); ok {
		for _, r := range recipients {
			rm, rok := r.(map[string]any)
			if !rok {
				continue
			}
			email, _ := rm["email"].(string)
			name, _ := rm["name"].(string)
			m.To = append(m.To, mail.Address{Email: email, Name: name})
		}
	}
	m.Subject, _ = req.Body["subject"].(string)
	if s, ok := req.Body["html_content"].(string); ok {
		m.HTML = []byte(s)
	}
	if s, ok := req.Body["text_content"].(string); ok {
		m.Text = []byte(s)
	}
	return m
}

// validTestMessage is the baseline message for the tests in this file.
func validTestMessage() mail.Message {
	return mail.Message{
		To:          []mail.Address{{Email: "to@example.com"}},
		Subject:     "Subject",
		HTML:        []byte("<p>hello</p>"),
		Text:        []byte("hello"),
		RenderedFor: "user:test",
	}
}

// TestHTTPSender_DoesNotLeakTokenInConfigDump covers the /api/v1/_config half
// of AC10 at its source: the credential is never a Config field, so no dump of
// a Config can carry it however that dump is produced.
func TestHTTPSender_DoesNotLeakTokenInConfigDump(t *testing.T) {
	t.Parallel()

	relaDir := writeSecrets(t, "mail_api_token: tok-secret\n")
	cfg := httpTestConfig(relaDir)

	dumped, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NotContains(t, string(dumped), "tok-secret")

	// And the resolution path does find it — otherwise the assertion above
	// would pass on a config that simply never worked.
	require.Equal(t, "tok-secret", mail.ExportResolveAPIToken(cfg))
	require.NotContains(t, strings.ToLower(string(dumped)), "token")
}
