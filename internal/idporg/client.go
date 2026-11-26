// Package idporg reads an organization's status from the identity
// provider's operator API (pratique), so tenant mode can follow an org's
// suspension (TKT-TNPRV8).
//
// The IdP's webhook events are thin: `organization.updated` names the org
// and nothing else, so a receiver asks the operator API for the current
// state. The API authenticates each request with an HMAC over the method,
// path, date and body digest, keyed with a secret both sides share. Only
// the signature travels; the secret never does.
package idporg

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/httpscheme"
)

// Status is an organization's status at the IdP.
type Status string

// The statuses the operator API reports.
const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// ErrNotFound reports an org the IdP answered it does not know.
var ErrNotFound = errors.New("idporg: organization not found")

// ErrInvalidOrgID reports an org id the client refuses to put into a
// request path, so the IdP was not asked. It is not [ErrNotFound]: nothing
// says the org is gone.
var ErrInvalidOrgID = errors.New("idporg: invalid organization id")

// ErrUnavailable marks every failure to get an answer from the IdP: it
// could not be reached, refused the request, or answered something that is
// not an org. The caller should retry later.
var ErrUnavailable = errors.New("idporg: identity provider unavailable")

// DefaultTimeout bounds one request to the operator API.
const DefaultTimeout = 5 * time.Second

// maxBody caps the response body read. An org record is a few hundred
// bytes.
const maxBody = 64 << 10

// The operator API's authentication headers.
const (
	headerDate      = "X-Pratique-Date"
	headerSignature = "X-Pratique-Signature"
)

// socketHost is the host name a request over a unix socket carries. The
// socket path, not the name, decides where it goes.
const socketHost = "idp-operator"

// Client reads org status from the operator API.
type Client struct {
	base   *url.URL
	secret []byte
	http   *http.Client
	now    func() time.Time
}

// New builds a client for the operator API at base, which is
// `https://host[:port]`, `unix:///path/to/socket`, or
// `http://host[:port]` for a loopback host, signing with secret.
// A request is bounded by [DefaultTimeout] and follows no redirect.
//
// Nil: rejected — an empty base or secret is an error, so a misconfigured
// server refuses to start.
func New(base string, secret []byte) (*Client, error) {
	if len(secret) == 0 {
		return nil, errors.New("idporg: the operator secret is required")
	}
	u, transport, err := parseBase(base)
	if err != nil {
		return nil, err
	}
	return &Client{
		base:   u,
		secret: secret,
		http: &http.Client{
			Transport: transport,
			Timeout:   DefaultTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now,
	}, nil
}

// parseBase reads the base address and builds the transport that reaches
// it. It refuses user info, a path, a query and a fragment: none has a use
// here, and user info would put a credential on the command line.
func parseBase(base string) (*url.URL, *http.Transport, error) {
	if base == "" {
		return nil, nil, errors.New("idporg: the operator API address is required")
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, nil, fmt.Errorf("idporg: operator API address: %w", err)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, nil, errors.New("idporg: the operator API address takes no user info, query or fragment")
	}
	// A fresh transport rather than a clone of the default one: the
	// operator API is never reached through a proxy.
	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   DefaultTimeout,
		ExpectContinueTimeout: time.Second,
	}
	switch u.Scheme {
	case httpscheme.HTTP, httpscheme.HTTPS:
		if u.Host == "" || (u.Path != "" && u.Path != "/") {
			return nil, nil, errors.New("idporg: the operator API address must be scheme://host[:port]")
		}
		// The answer is not signed, so whoever can change it in transit
		// can suspend or resume an org. Plain http is safe only when the
		// traffic never leaves the machine.
		if u.Scheme == "http" && !isLoopback(u.Hostname()) {
			return nil, nil, fmt.Errorf("idporg: operator API host %q needs https; "+
				"plain http is allowed only for localhost or a loopback IP", u.Hostname())
		}
		return &url.URL{Scheme: u.Scheme, Host: u.Host}, transport, nil
	case "unix":
		if u.Host != "" || u.Path == "" {
			return nil, nil, errors.New("idporg: a unix operator API address must be unix:///path/to/socket")
		}
		socket := u.Path
		var d net.Dialer
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return d.DialContext(ctx, "unix", socket)
		}
		return &url.URL{Scheme: "http", Host: socketHost}, transport, nil
	}
	return nil, nil, fmt.Errorf("idporg: operator API scheme %q is not http, https or unix", u.Scheme)
}

// isLoopback reports whether host is localhost or a loopback IP literal. A
// name other than localhost is not resolved: what it resolves to can
// change after startup.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// org is the part of the operator API's org record the client reads.
type org struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// OrgStatus returns orgID's status at the IdP. It returns [ErrNotFound] for
// an org the IdP does not know, [ErrInvalidOrgID] for an id it cannot ask
// about, and an error wrapping [ErrUnavailable] for every other failure.
func (c *Client) OrgStatus(ctx context.Context, orgID string) (Status, error) {
	// The signature covers the decoded path. An id that needs escaping
	// would be signed in one form and checked in another, and is not an
	// IdP org id anyway.
	if orgID == "" || url.PathEscape(orgID) != orgID {
		return "", fmt.Errorf("%w: org id %q cannot name an IdP organization", ErrInvalidOrgID, orgID)
	}
	path := "/api/v1/orgs/" + orgID
	u := *c.base
	u.Path = path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	date := strconv.FormatInt(c.now().Unix(), 10)
	req.Header.Set(headerDate, date)
	req.Header.Set(headerSignature, sign(c.secret, http.MethodGet, path, date, nil))
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return "", fmt.Errorf("%w: read the answer: %w", ErrUnavailable, err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return "", fmt.Errorf("%w: %q", ErrNotFound, orgID)
	case resp.StatusCode != http.StatusOK:
		// The body is not echoed: the IdP's error text is not ours to log.
		return "", fmt.Errorf("%w: the operator API answered %d", ErrUnavailable, resp.StatusCode)
	case len(body) > maxBody:
		return "", fmt.Errorf("%w: the answer is larger than %d bytes", ErrUnavailable, maxBody)
	}
	var o org
	if err := json.Unmarshal(body, &o); err != nil {
		return "", fmt.Errorf("%w: the answer is not an org record: %w", ErrUnavailable, err)
	}
	if o.ID != orgID {
		return "", fmt.Errorf("%w: the answer names org %q, not %q", ErrUnavailable, o.ID, orgID)
	}
	// An empty or missing status is not read as active: an answer that
	// does not say the org is active must not resume it, so it is
	// unavailable and the caller retries. pratique's operator API always
	// sends the status and reports an org without one as active itself.
	switch Status(o.Status) {
	case StatusActive:
		return StatusActive, nil
	case StatusSuspended:
		return StatusSuspended, nil
	}
	return "", fmt.Errorf("%w: org %q has status %q, which is not active or suspended",
		ErrUnavailable, orgID, o.Status)
}

// sign returns the operator API signature of one request: the base64
// HMAC-SHA256, keyed with secret, of METHOD, PATH, DATE and the hex SHA-256
// of the body, joined by newlines. path is the decoded request path and
// date is in unix seconds.
func sign(secret []byte, method, path, date string, body []byte) string {
	sum := sha256.Sum256(body)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(method + "\n" + path + "\n" + date + "\n" + hex.EncodeToString(sum[:])))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
