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

	"github.com/Sourcehaven-BV/rela/internal/reconwire"

	"github.com/Sourcehaven-BV/rela/internal/reconclient"
)

// reconcilePath is the endpoint under the base URL.
const reconcilePath = "/api/v1/_reconcile"

// apiPrefix starts every read route [Remote.Get] takes.
const apiPrefix = "/api/v1/"

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
	// api is the base of the read routes, ending in "/api/v1/".
	api    string
	origin string
	auth   Authenticator
	client *http.Client
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
	base := origin + strings.TrimRight(u.EscapedPath(), "/")
	return &Remote{
		endpoint: base + reconcilePath,
		api:      base + apiPrefix,
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
func (r *Remote) Reconcile(ctx context.Context, req reconwire.Request) (reconwire.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return reconwire.Response{}, fmt.Errorf("httpremote: encode request: %w", err)
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return reconwire.Response{}, fmt.Errorf("httpremote: %s: %w", r.origin, err)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	hreq.Header.Set("User-Agent", userAgent)
	if r.auth != nil {
		r.auth.Authenticate(hreq.Header)
	}
	resp, err := r.client.Do(hreq)
	if err != nil {
		return reconwire.Response{}, fmt.Errorf("httpremote: %s: %w", r.origin, unwrapURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return r.decodeAnswer(resp.Body)
	}
	return reconwire.Response{}, r.refusal(resp)
}

func (r *Remote) decodeAnswer(body io.Reader) (reconwire.Response, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxAnswer+1))
	if err != nil {
		return reconwire.Response{}, fmt.Errorf("httpremote: %s: read: %w", r.origin, unwrapURLError(err))
	}
	if len(raw) > maxAnswer {
		return reconwire.Response{}, fmt.Errorf("%w: %s: the answer exceeds %d bytes",
			reconclient.ErrProtocol, r.origin, maxAnswer)
	}
	var out reconwire.Response
	if err := json.Unmarshal(raw, &out); err != nil {
		return reconwire.Response{}, fmt.Errorf("%w: %s: the answer is not valid JSON", reconclient.ErrProtocol, r.origin)
	}
	return out, nil
}

// ErrNotFound reports that [Remote.Get] was answered 404.
var ErrNotFound = errors.New("httpremote: not found")

// ErrDenied reports that a request was answered 401 or 403: the credential
// is missing, wrong, or no longer grants the read.
var ErrDenied = errors.New("httpremote: access denied")

// Get reads one route of the remote's API, path relative to /api/v1/ with
// its segments already escaped, and returns the body of a 200 answer. A
// body over limit bytes is an error, as is any other status: 404 wraps
// [ErrNotFound], 401 and 403 wrap [ErrDenied].
func (r *Remote) Get(ctx context.Context, path string, limit int64) ([]byte, error) {
	if !validPath(path) {
		return nil, errors.New("httpremote: invalid path")
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, r.api+path, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("httpremote: %s: %w", r.origin, err)
	}
	hreq.Header.Set("Accept", "application/json")
	hreq.Header.Set("User-Agent", userAgent)
	if r.auth != nil {
		r.auth.Authenticate(hreq.Header)
	}
	resp, err := r.client.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("httpremote: %s: %w", r.origin, unwrapURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProblem))
		return nil, fmt.Errorf("%w: %s", ErrNotFound, r.origin)
	case http.StatusUnauthorized, http.StatusForbidden:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProblem))
		return nil, fmt.Errorf("%w: %s answered %d", ErrDenied, r.origin, resp.StatusCode)
	default:
		return nil, r.refusal(resp)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("httpremote: %s: read: %w", r.origin, unwrapURLError(err))
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%w: %s: the answer exceeds %d bytes", reconclient.ErrProtocol, r.origin, limit)
	}
	return raw, nil
}

// validPath reports whether path is a relative route of escaped segments:
// no empty, "." or ".." segment (also when escaped), no escaped slash, and
// no query or fragment.
func validPath(path string) bool {
	if path == "" || strings.ContainsAny(path, "?#\\") {
		return false
	}
	for seg := range strings.SplitSeq(path, "/") {
		plain, err := url.PathUnescape(seg)
		if err != nil || plain == "" || plain == "." || plain == ".." || strings.ContainsAny(plain, "/\\") {
			return false
		}
	}
	return true
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
		return fmt.Errorf("%w: %s: authentication required (401); supply a token", ErrDenied, r.origin)
	case resp.StatusCode == http.StatusForbidden && p.index < 0:
		return fmt.Errorf("%w: %s: answered 403", ErrDenied, r.origin)
	case resp.StatusCode == http.StatusNotImplemented:
		return fmt.Errorf("httpremote: %s: the remote does not serve reconciliation (501)", r.origin)
	case p.index >= 0:
		return &reconclient.RangeError{
			Index:     p.index,
			MetaGated: resp.StatusCode == http.StatusUnprocessableEntity && strings.HasSuffix(p.kind, "/relation_meta_gated"),
			Status:    resp.StatusCode,
			Detail:    reconclient.Sanitize(p.detail),
		}
	case resp.StatusCode == http.StatusUnprocessableEntity && strings.HasSuffix(p.kind, "/relation_meta_gated"):
		return &reconclient.RangeError{Index: -1, MetaGated: true, Status: resp.StatusCode,
			Detail: reconclient.Sanitize(p.detail)}
	}
	msg := strconv.Itoa(resp.StatusCode)
	if t := reconclient.Sanitize(p.title); t != "" {
		msg += " " + t
	}
	if d := reconclient.Sanitize(p.detail); d != "" {
		msg += ": " + d
	}
	return fmt.Errorf("httpremote: %s: answered %s", r.origin, msg)
}

// refusal is what the client uses of an RFC 9457 problem body.
type refusal struct {
	// kind is the problem's type URI.
	kind, title, detail string
	// index is the range the problem names, or -1.
	index int
}

// rangeIndex returns the index of the range a JSON pointer names,
// /ranges/<index>, or -1 when it names none.
func rangeIndex(pointer string) int {
	digits, ok := strings.CutPrefix(pointer, "/ranges/")
	if !ok || digits == "" || len(digits) > maxIndexDigits || (len(digits) > 1 && digits[0] == '0') {
		return -1
	}
	i, err := strconv.Atoi(digits)
	if err != nil || i < 0 {
		return -1
	}
	return i
}

// maxIndexDigits bounds the index of a range pointer. A request holds far
// fewer ranges; the bound keeps Atoi off long input.
const maxIndexDigits = 4

// readProblem reads an RFC 9457 problem body. A body that is not one names
// no range and carries no text. A refusal of one range names it with the
// JSON pointer of its first error.
func readProblem(body io.Reader) refusal {
	none := refusal{index: -1}
	raw, err := io.ReadAll(io.LimitReader(body, maxProblem))
	if err != nil {
		return none
	}
	var p struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Errors []struct {
			Source struct {
				Pointer string `json:"pointer"`
			} `json:"source"`
		} `json:"errors"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return none
	}
	out := refusal{kind: p.Type, title: p.Title, detail: p.Detail, index: -1}
	if len(p.Errors) > 0 {
		out.index = rangeIndex(p.Errors[0].Source.Pointer)
	}
	return out
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
