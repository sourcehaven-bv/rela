package docscapture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// APIClient implements docs.APIClient: it stands up the data-entry server over
// a seeded temp project and issues real HTTP requests against it.
//
// It lives here rather than in internal/docs for the same reason the Capturer
// does — it pulls in dataentry + appbuild, which the core doc language must not
// carry. Unlike the Capturer it needs NO browser and no built frontend, so a
// manual using only api{} assertions runs anywhere the Go tests do.
//
// The temp project is stood up lazily on the first request and reused, with the
// manual's growing seed applied incrementally (see project.syncSeed): a manual
// creates entities as it goes, so a later api{} must see entities created after
// the server started.
//
// It does not own that project — the DOCUMENT does, via SharedProject, which
// the screenshot{}/page{} capturer reaches too. That sharing is what makes a
// write issued here visible to a later figure.
//
// Nil: never returned by New; the zero value is not usable — use NewAPIClient.
type APIClient struct {
	shared *SharedProject

	mu     sync.Mutex
	proj   *project
	closed bool
}

// NewAPIClient returns a client serving the document's shared temp project.
func NewAPIClient(shared *SharedProject) *APIClient {
	return &APIClient{shared: shared}
}

// Do issues one request, standing the server up on first use.
//
// An HTTP error status is a normal return, not an error: asserting a 403 or a
// 404 is the point of the verb. Only a transport-level failure errors.
func (c *APIClient) Do(ctx context.Context, req docs.APIRequest) (docs.APIResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Without this a request after Close silently stands up a WHOLE NEW temp
	// project, so the caller gets an answer from a server it believes it tore
	// down — and the fresh project leaks.
	if c.closed {
		return docs.APIResponse{}, errors.New("api{}: client is closed")
	}

	if c.shared == nil {
		return docs.APIResponse{}, errors.New("api{}: no shared project wired")
	}
	// No SPA needed: api{} talks to the data-entry router directly, so a manual
	// with only api{} assertions runs with no built frontend.
	p, err := c.shared.acquire(ctx, req.ProjectDir, req.Seed, false)
	if err != nil {
		return docs.APIResponse{}, fmt.Errorf("api{}: %w", err)
	}
	c.proj = p

	if err := c.proj.requireKnownRole(req.As); err != nil {
		return docs.APIResponse{}, err
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	httpReq, rerr := http.NewRequestWithContext(ctx, method, c.proj.server.URL+req.Path, body)
	if rerr != nil {
		return docs.APIResponse{}, fmt.Errorf("api{path=%q}: %w", req.Path, rerr)
	}
	if req.Body != "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	// The role header is what the temp server's principal resolver maps to a
	// principal assigned that role in acl.yaml — the same mechanism screenshot{}
	// uses for `as=`.
	if req.As != "" {
		httpReq.Header.Set(roleHeader, req.As)
	}

	resp, derr := c.proj.server.Client().Do(httpReq)
	if derr != nil {
		return docs.APIResponse{}, fmt.Errorf("api{path=%q}: %w", req.Path, derr)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, berr := io.ReadAll(resp.Body)
	if berr != nil {
		return docs.APIResponse{}, fmt.Errorf("api{path=%q}: reading body: %w", req.Path, berr)
	}
	return docs.APIResponse{Status: resp.StatusCode, Body: string(raw), Header: resp.Header}, nil
}

// Close marks the client unusable. It does NOT tear down the temp project:
// that belongs to the document's SharedProject, which the capturer may still
// be using and which the wiring site closes when the document finishes.
func (c *APIClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.proj = nil
	return nil
}
