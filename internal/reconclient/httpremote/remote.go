// Package httpremote implements reconclient.Remote over HTTP: it posts a
// request to a rela server's /api/v1/_reconcile and maps the refusals to the
// error types the Differ acts on.
//
// The remote is untrusted. The client follows no redirect, keeps no
// cookies, caps every body it reads, and names only the remote's origin in
// its errors. A credential goes only to the configured origin, and over
// plain HTTP only to a loopback host unless the caller allows it.
package httpremote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
)

// reconcilePath is the endpoint under the base URL.
const reconcilePath = "/api/v1/_reconcile"

// Limits of what one exchange reads.
const (
	// maxAnswer caps a 200 body: 2000 listed elements with addresses of up
	// to 4096 bytes fit with room to spare.
	maxAnswer = 16 << 20
	// maxProblem caps an error body.
	maxProblem = 64 << 10
	// defaultTimeout bounds one exchange. The server stops its own work
	// after 15 seconds.
	defaultTimeout = time.Minute
	userAgent      = "rela-reconcile/1"
	// maxRetryAfter caps the wait a Retry-After header asks for.
	maxRetryAfter = 24 * time.Hour
)

// Config configures a [Remote].
type Config struct {
	// BaseURL is the remote rela's address, such as https://rela.example
	// or https://example.org/rela. It holds no credentials, query or
	// fragment.
	BaseURL string
	// Auth adds the credential.
	//
	// Nil: accepted. The requests carry no credential.
	Auth Authenticator
	// Transport carries the requests.
	//
	// Nil: accepted, meaning a clone of http.DefaultTransport; tests
	// inject their own.
	Transport http.RoundTripper
	// Timeout bounds one exchange; zero means one minute.
	Timeout time.Duration
	// AllowInsecureHTTP permits plain HTTP to a host that is not loopback.
	// Every request carries local addresses as range bounds, and every
	// answer could be forged on the way, so plain HTTP is refused with or
	// without a credential.
	AllowInsecureHTTP bool
}

// Remote posts reconcile requests to one rela server.
type Remote struct {
	endpoint string
	origin   string
	auth     Authenticator
	client   *http.Client
}

// New returns a Remote for c, or an error naming what is wrong with it.
func New(c Config) (*Remote, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, errors.New("httpremote: the remote URL does not parse")
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, errors.New("httpremote: the remote URL must start with https:// or http://")
	case u.Host == "":
		return nil, errors.New("httpremote: the remote URL has no host")
	case u.User != nil:
		return nil, errors.New("httpremote: the remote URL must not hold credentials; use a token")
	case u.RawQuery != "" || u.Fragment != "" || u.ForceQuery:
		return nil, errors.New("httpremote: the remote URL must not have a query or fragment")
	}
	plain := u.Scheme == "http"
	if plain && !loopback(u.Hostname()) && !c.AllowInsecureHTTP {
		return nil, fmt.Errorf("httpremote: refusing plain http to %s; use https", u.Host)
	}
	transport := c.Transport
	if transport == nil {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok { // coverage-ignore: invariant: the standard library's default is a *http.Transport
			return nil, errors.New("httpremote: the default transport is not an *http.Transport")
		}
		t := base.Clone()
		if plain {
			// A proxy from the environment would read the credential and
			// the addresses in clear text.
			t.Proxy = nil
		}
		transport = t
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	origin := u.Scheme + "://" + u.Host
	return &Remote{
		endpoint: origin + strings.TrimRight(u.EscapedPath(), "/") + reconcilePath,
		origin:   origin,
		auth:     c.Auth,
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
			// A redirect would take the credential elsewhere; the
			// configured address is the only one used.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// Origin returns the remote's scheme and host, the part of its address
// errors and output name.
func (r *Remote) Origin() string { return r.origin }

// loopback reports whether host is localhost or a loopback address.
func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Reconcile sends req and returns the decoded answer. The answer's content
// is not checked here; the Differ validates it.
func (r *Remote) Reconcile(ctx context.Context, req v1.ReconcileRequest) (v1.ReconcileResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return v1.ReconcileResponse{}, fmt.Errorf("httpremote: encode request: %w", err)
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return v1.ReconcileResponse{}, fmt.Errorf("httpremote: %s: %w", r.origin, err)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	hreq.Header.Set("User-Agent", userAgent)
	if r.auth != nil {
		r.auth.Authenticate(hreq.Header)
	}
	resp, err := r.client.Do(hreq)
	if err != nil {
		return v1.ReconcileResponse{}, fmt.Errorf("httpremote: %s: %w", r.origin, unwrapURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return r.decodeAnswer(resp.Body)
	}
	return v1.ReconcileResponse{}, r.refusal(resp)
}

func (r *Remote) decodeAnswer(body io.Reader) (v1.ReconcileResponse, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxAnswer+1))
	if err != nil {
		return v1.ReconcileResponse{}, fmt.Errorf("httpremote: %s: read: %w", r.origin, unwrapURLError(err))
	}
	if len(raw) > maxAnswer {
		return v1.ReconcileResponse{}, fmt.Errorf("%w: %s: the answer exceeds %d bytes",
			reconclient.ErrProtocol, r.origin, maxAnswer)
	}
	var out v1.ReconcileResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return v1.ReconcileResponse{}, fmt.Errorf("%w: %s: the answer is not valid JSON", reconclient.ErrProtocol, r.origin)
	}
	return out, nil
}

// refusal maps a status other than 200 to an error.
func (r *Remote) refusal(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		// Drain a little so the connection can carry the retry.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProblem))
		return &reconclient.RefusedError{
			Status: resp.StatusCode, RetryAfter: retryAfter(resp.Header.Get("Retry-After"), time.Now()),
		}
	}
	p := readProblem(resp.Body)
	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return fmt.Errorf("httpremote: %s: answered %d (a redirect); point --remote at the final address",
			r.origin, resp.StatusCode)
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("httpremote: %s: authentication required (401); supply a token", r.origin)
	case resp.StatusCode == http.StatusNotImplemented:
		return fmt.Errorf("httpremote: %s: the remote does not serve reconciliation (501)", r.origin)
	case p.Range != nil:
		return &reconclient.RangeError{
			Index:     *p.Range,
			MetaGated: resp.StatusCode == http.StatusUnprocessableEntity && strings.HasSuffix(p.Type, "/relation_meta_gated"),
			Status:    resp.StatusCode,
			Detail:    reconclient.Sanitize(p.Detail),
		}
	case resp.StatusCode == http.StatusUnprocessableEntity && strings.HasSuffix(p.Type, "/relation_meta_gated"):
		return &reconclient.RangeError{Index: -1, MetaGated: true, Status: resp.StatusCode,
			Detail: reconclient.Sanitize(p.Detail)}
	}
	msg := strconv.Itoa(resp.StatusCode)
	if t := reconclient.Sanitize(p.Title); t != "" {
		msg += " " + t
	}
	if d := reconclient.Sanitize(p.Detail); d != "" {
		msg += ": " + d
	}
	return fmt.Errorf("httpremote: %s: answered %s", r.origin, msg)
}

// readProblem reads an RFC 9457 problem body, or returns the zero value
// when the body is not one.
func readProblem(body io.Reader) v1.Error {
	raw, err := io.ReadAll(io.LimitReader(body, maxProblem))
	if err != nil {
		return v1.Error{}
	}
	var p v1.Error
	if json.Unmarshal(raw, &p) != nil {
		return v1.Error{}
	}
	return p
}

// retryAfter parses a Retry-After value, seconds or an HTTP date, relative
// to now. A missing or invalid value is zero; the caller applies its floor
// and cap.
func retryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(min(secs, int(maxRetryAfter/time.Second))) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return min(t.Sub(now), maxRetryAfter)
	}
	return 0
}

// unwrapURLError drops the *url.Error wrapper, whose text repeats the full
// URL.
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}
