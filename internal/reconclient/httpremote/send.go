package httpremote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/reconclient"
)

// Answer is a response to [Remote.Send] that reached the remote's API.
type Answer struct {
	// Status is the HTTP status.
	Status int
	// Body is the response body on a success. On any other status it is the
	// problem detail, with control characters removed.
	Body []byte
	// ETag is the ETag header as the remote sent it, or empty.
	ETag string
}

// maxETag bounds the ETag a caller keeps. rela's ETags are a quoted hex
// digest; anything much longer is not one.
const maxETag = 256

// Send sends one request to a route of the remote's API: the reads of a
// sync remote that need the ETag, and its writes. path is relative to
// /api/v1/ with its segments escaped, body is JSON or nil, and ifMatch,
// when not empty, is sent as If-Match.
//
// It returns the answer for every status a row can explain, so the caller
// decides what a 403, 404, 409, 412 or 422 means for its row. It fails for
// a transport error, a redirect, a 401 (wrapping [ErrDenied]: the remote
// does not accept the credential, which no row can fix), a 429 or 503
// ([reconclient.RefusedError]), and a success body over limit.
func (r *Remote) Send(
	ctx context.Context, method, path string, body []byte, ifMatch string, limit int64,
) (Answer, error) {
	if !validPath(path) {
		return Answer{}, errors.New("httpremote: invalid path")
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete:
	default:
		return Answer{}, fmt.Errorf("httpremote: method %s is not sent", method)
	}
	if strings.ContainsAny(ifMatch, "\r\n") || len(ifMatch) > maxETag {
		return Answer{}, errors.New("httpremote: invalid If-Match value")
	}
	var reader io.Reader = http.NoBody
	if body != nil {
		reader = bytes.NewReader(body)
	}
	hreq, err := http.NewRequestWithContext(ctx, method, r.api+path, reader)
	if err != nil {
		return Answer{}, fmt.Errorf("httpremote: %s: %w", r.origin, err)
	}
	hreq.Header.Set("Accept", "application/json")
	hreq.Header.Set("User-Agent", userAgent)
	if body != nil {
		hreq.Header.Set("Content-Type", "application/json")
	}
	if ifMatch != "" {
		hreq.Header.Set("If-Match", ifMatch)
	}
	if r.auth != nil {
		r.auth.Authenticate(hreq.Header)
	}
	resp, err := r.client.Do(hreq)
	if err != nil {
		return Answer{}, fmt.Errorf("httpremote: %s: %w", r.origin, unwrapURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusServiceUnavailable,
		resp.StatusCode >= 300 && resp.StatusCode < 400:
		return Answer{}, r.refusal(resp)
	}
	out := Answer{Status: resp.StatusCode}
	if tag := resp.Header.Get("ETag"); len(tag) <= maxETag {
		out.ETag = tag
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p := readProblem(resp.Body)
		detail := reconclient.Sanitize(p.detail)
		if detail == "" {
			detail = reconclient.Sanitize(p.title)
		}
		out.Body = []byte(detail)
		return out, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return Answer{}, fmt.Errorf("httpremote: %s: read: %w", r.origin, unwrapURLError(err))
	}
	if int64(len(raw)) > limit {
		return Answer{}, fmt.Errorf("%w: %s: the answer exceeds %d bytes", reconclient.ErrProtocol, r.origin, limit)
	}
	out.Body = raw
	return out, nil
}
