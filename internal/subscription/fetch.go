package subscription

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

const (
	fetchTimeout = 30 * time.Second
	maxRedirects = 5
	// maxValidatorLen bounds a stored ETag or Last-Modified. Real ones are
	// tens of bytes; a longer one is stored nowhere and sent back never.
	maxValidatorLen = 1024
	userAgent       = "rela-subscriptions/1"
	acceptHeader    = "application/rss+xml, application/atom+xml, application/feed+json, " +
		"application/json;q=0.9, application/xml;q=0.8, text/xml;q=0.8, */*;q=0.5"
)

// maxBody is the response cap: the same limit feed.Parse enforces, so the two
// cannot drift.
var maxBody = int64(safexml.DefaultLimits.MaxBytes)

// Fetcher does the conditional GET of one poll.
type Fetcher struct {
	// Transport carries the requests. Nil: accepted, meaning
	// http.DefaultTransport; tests inject their own.
	Transport http.RoundTripper
}

// request is one conditional GET.
type request struct {
	url          *url.URL
	header       http.Header // auth only
	etag         string
	lastModified string
}

// response is the outcome of a fetch.
type response struct {
	notModified  bool
	body         []byte
	finalURL     *url.URL
	etag         string
	lastModified string
}

// fetch performs the request. Errors name the origin only: a feed URL may
// carry a token in its query, and errors end up in logs and run state.
func (f *Fetcher) fetch(ctx context.Context, req request) (response, error) {
	origin := originOf(req.url)
	client := &http.Client{
		Transport:     f.Transport,
		Timeout:       fetchTimeout,
		CheckRedirect: checkRedirect,
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.url.String(), http.NoBody)
	if err != nil {
		return response{}, fmt.Errorf("fetch %s: %w", origin, err)
	}
	maps.Copy(httpReq.Header, req.header)
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("Accept", acceptHeader)
	if req.etag != "" {
		httpReq.Header.Set("If-None-Match", req.etag)
	}
	if req.lastModified != "" {
		httpReq.Header.Set("If-Modified-Since", req.lastModified)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return response{}, fmt.Errorf("fetch %s: %w", origin, unwrapURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return response{notModified: true}, nil
	case resp.StatusCode != http.StatusOK:
		return response{}, fmt.Errorf("fetch %s: status %d", origin, resp.StatusCode)
	}

	// Read past the cap by one byte to tell "exactly the cap" from "over it".
	// The body is already decompressed here, so the cap bounds memory, not
	// wire bytes.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return response{}, fmt.Errorf("fetch %s: read: %w", origin, unwrapURLError(err))
	}
	if int64(len(body)) > maxBody {
		return response{}, fmt.Errorf("fetch %s: response exceeds %d bytes", origin, maxBody)
	}
	return response{
		body:         body,
		finalURL:     resp.Request.URL,
		etag:         validator(resp.Header.Get("ETag")),
		lastModified: validator(resp.Header.Get("Last-Modified")),
	}, nil
}

// checkRedirect follows a redirect only on the configured origin, allowing an
// http to https upgrade of the same host.
//
// A redirect target is chosen by the feed's publisher, not the operator. Off
// the origin it could point the server at an internal address, and Go's own
// redirect rules would forward the auth header to any subdomain and keep a
// custom one everywhere. A feed that moved is a config change.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	from := via[0].URL
	if !sameOrigin(from, req.URL) {
		return fmt.Errorf("redirect to another origin (%s); update the subscription url", locationOf(req.URL))
	}
	return nil
}

// sameOrigin reports whether to is on from's origin, or is its https upgrade.
func sameOrigin(from, to *url.URL) bool {
	if !hostsEqual(from.Hostname(), to.Hostname()) {
		return false
	}
	if from.Scheme == to.Scheme {
		return portOf(from) == portOf(to)
	}
	// An upgrade only between default ports: http://h:8080 to https://h:8443
	// is a different service.
	return from.Scheme == "http" && to.Scheme == "https" && from.Port() == "" && to.Port() == ""
}

func hostsEqual(a, b string) bool {
	return a != "" && strings.EqualFold(a, b)
}

func portOf(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// originOf is scheme://host[:port], with no path or query.
func originOf(u *url.URL) string {
	return u.Scheme + "://" + u.Host
}

// locationOf is the origin and path of u, without the query.
func locationOf(u *url.URL) string {
	return originOf(u) + u.EscapedPath()
}

// unwrapURLError drops the *url.Error wrapper, whose message repeats the full
// URL including its query.
func unwrapURLError(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err
	}
	return err
}

// validator returns v when it is safe to store and send back: short, and free
// of control characters, which Go would refuse in a request header and so
// fail every later poll.
func validator(v string) string {
	if len(v) > maxValidatorLen {
		return ""
	}
	for i := range len(v) {
		if c := v[i]; (c < 0x20 && c != '\t') || c == 0x7f {
			return ""
		}
	}
	return v
}
