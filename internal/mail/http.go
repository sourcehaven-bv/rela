package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SimpleMailService APIv2 constants.
//
// The base URL is not configurable. An operator-supplied endpoint would make
// this transport a general "POST my mail somewhere" primitive with a
// provider-shaped body — which is precisely what `transport: script` is for,
// and doing it here would put a redirectable credential-bearing request behind
// a config key. A different provider means a different script, not a different
// URL under the same field names.
const (
	simpleMailBaseURL = "https://api.simplemailservice.eu/v2"

	// simpleMailMaxErrorBytes caps how much of a failure response is read into
	// an error message. The body is attacker-influenced (a compromised or
	// simply misbehaving upstream), and an unbounded read into a log line is
	// a memory and log-flooding problem for no diagnostic gain.
	simpleMailMaxErrorBytes = 4 << 10
)

// HTTPSenderSecretKey is the key read from .rela/secrets.yaml for the API
// token. Named like [SecretKey] for SMTP so an operator finds both in the same
// place.
const HTTPSenderSecretKey = "mail_api_token"

// HTTPSender delivers through the SimpleMailService APIv2 HTTP API.
//
// # Why exactly one provider is compiled in
//
// Provider send APIs agree on nothing: Postmark uses a custom header and
// capitalized field names, Resend uses bearer auth and lowercase ones, Mailgun
// is not JSON at all — it is multipart/form-data with HTTP Basic. A JSON
// field-mapping DSL that covered the three JSON ones would still exclude
// Mailgun by construction, so the general answer is [ScriptSender] plus the
// http/crypto primitives in internal/lua, and the FEAT-CN5L0X precedent
// applies: the generic primitive lives in Go, the provider specifics live in
// an example Lua script.
//
// This one transport exists because a first-class, zero-Lua path for the
// provider rela is deployed against is worth having, and because a concrete
// second wire transport keeps the [Sender] seam honest — the same reason
// [MemorySender] exists.
type HTTPSender struct {
	cfg Config

	// client is the HTTP client. Bounded explicitly rather than
	// http.DefaultClient, which has NO timeout: a hung provider would
	// otherwise pin the outbox's single worker goroutine forever and stall
	// every message behind it, with Outbox.Stop unable to do anything about it.
	client *http.Client

	// baseURL is the API root. Overridable ONLY from Go (see
	// WithHTTPBaseURL), for tests against an httptest.Server.
	baseURL string
}

// HTTPOption adjusts an HTTPSender.
type HTTPOption func(*HTTPSender)

// WithHTTPBaseURL points the sender at a different API root.
//
// Go-only, with no YAML counterpart, exactly as [WithRootCAs] is for SMTP:
// this is for tests against an httptest.Server. Exposing it in config would
// let a project file redirect authenticated, credential-bearing requests to an
// arbitrary host — a credential-exfiltration primitive dressed as a
// convenience.
func WithHTTPBaseURL(u string) HTTPOption {
	return func(s *HTTPSender) { s.baseURL = strings.TrimSuffix(u, "/") }
}

// WithHTTPClient replaces the HTTP client. For tests.
func WithHTTPClient(c *http.Client) HTTPOption {
	return func(s *HTTPSender) {
		if c != nil {
			s.client = c
		}
	}
}

// NewHTTPSender returns a sender for cfg.
//
// Nil: a nil config is rejected. The config is re-validated here rather than
// trusted from load, so a programmatically constructed Config cannot bypass
// the checks the YAML path enforces.
func NewHTTPSender(cfg *Config, opts ...HTTPOption) (*HTTPSender, error) {
	if cfg == nil {
		return nil, errors.New("mail: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Transport != TransportHTTP {
		return nil, fmt.Errorf("mail: NewHTTPSender given transport %q", cfg.Transport)
	}
	s := &HTTPSender{
		cfg:     *cfg,
		baseURL: simpleMailBaseURL,
		client:  &http.Client{Timeout: time.Duration(cfg.Timeout()) * time.Second},
	}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// simpleMailAddress is the {email, name} shape APIv2 uses for both the sender
// and each recipient.
type simpleMailAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// simpleMailAttachment is one attachment. Inline images ride here with
// base64=true; APIv2 has no separate inline-image field, so the Content-ID
// linkage is carried by file_name matching the cid: reference in the HTML.
type simpleMailAttachment struct {
	Data        string `json:"data"`
	Base64      bool   `json:"base64"`
	ContentType string `json:"content_type"`
	FileName    string `json:"file_name"`
}

// simpleMailRequest is the APIv2 message body. Field names are the provider's,
// verified against its documentation; they are NOT rela's vocabulary and must
// not be renamed for internal tidiness.
type simpleMailRequest struct {
	From          simpleMailAddress      `json:"from"`
	Recipients    []simpleMailAddress    `json:"recipients"`
	Subject       string                 `json:"subject"`
	HTMLContent   string                 `json:"html_content,omitempty"`
	TextContent   string                 `json:"text_content,omitempty"`
	Attachments   []simpleMailAttachment `json:"attachments,omitempty"`
	Substitutions map[string]string      `json:"substitutions,omitempty"`
}

// Send delivers m through the APIv2 messages endpoint.
//
// Validation runs FIRST and independently of the transport, so this transport
// refuses exactly the messages SMTP and memory refuse — the property the
// mailtest conformance suite exists to hold. In particular the CR/LF header
// check still applies: JSON encoding would happily carry a newline in a
// subject, and the provider then puts it in a header at the far end, so the
// injection defense cannot be delegated to the encoding.
func (s *HTTPSender) Send(ctx context.Context, m Message) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	token := s.cfg.resolveAPIToken()
	if token == "" {
		return fmt.Errorf("mail: no API token; set %q in .rela/secrets.yaml or name an "+
			"environment variable in password_env", HTTPSenderSecretKey)
	}

	body, err := json.Marshal(s.buildRequest(m))
	if err != nil {
		return fmt.Errorf("mail: encoding request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/accounts/%s/messages", s.baseURL, s.cfg.AccountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mail: building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		// The URL can appear in a *url.Error, and the token never does — it is
		// a header, not userinfo. Redact anyway: this error reaches a log, and
		// the cost of the guarantee holding by construction rather than by
		// this argument remaining true is one function call.
		return errors.New(redact(fmt.Sprintf("mail: sending: %v", err), token))
	}
	defer func() { _ = resp.Body.Close() }()

	return s.checkResponse(resp, token)
}

// checkResponse turns a non-2xx into an error carrying enough of the body to
// diagnose, with the credential redacted.
//
// The body is included because provider errors are the only thing that
// distinguishes "your token is revoked" from "that recipient is suppressed",
// and an operator staring at a bare 400 has nothing to act on. It is bounded
// and redacted for the reasons on simpleMailMaxErrorBytes and redact.
func (s *HTTPSender) checkResponse(resp *http.Response, token string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Drain before close so the connection returns to the pool rather than
		// being torn down — the outbox sends repeatedly to one host, so this
		// is the difference between reusing a TLS session and renegotiating.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, simpleMailMaxErrorBytes))
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, simpleMailMaxErrorBytes))
	return errors.New(redact(
		fmt.Sprintf("mail: provider returned %s: %s", resp.Status, strings.TrimSpace(string(snippet))),
		token))
}

// buildRequest maps a Message onto the APIv2 body.
//
// It does NOT mutate m: the outbox hands the same message to a retry, so a
// transport that consumed or rewrote it would make attempt two send something
// different from attempt one.
func (s *HTTPSender) buildRequest(m Message) simpleMailRequest {
	req := simpleMailRequest{
		From:        simpleMailAddress{Email: s.cfg.From, Name: s.cfg.FromName},
		Subject:     m.Subject,
		HTMLContent: string(m.HTML),
		TextContent: string(m.Text),
	}
	req.Recipients = make([]simpleMailAddress, 0, len(m.To))
	for _, a := range m.To {
		req.Recipients = append(req.Recipients, simpleMailAddress(a))
	}
	for _, img := range m.InlineImages {
		req.Attachments = append(req.Attachments, simpleMailAttachment{
			Data:        base64.StdEncoding.EncodeToString(img.Data),
			Base64:      true,
			ContentType: img.ContentType,
			FileName:    img.CID,
		})
	}
	// substitutions is deliberately left empty. Bodies arrive already rendered
	// (internal/mailrender), so handing the provider a second templating pass
	// would give it a chance to reinterpret content rela has already sanitized
	// — and would make two transports disagree about what a message says.
	return req
}
