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
// Nil: never returned by New; the zero value is not usable — use NewAPIClient.
type APIClient struct {
	projectDir string

	mu   sync.Mutex
	proj *project
}

// NewAPIClient returns a client serving the given project.
func NewAPIClient(projectDir string) *APIClient {
	return &APIClient{projectDir: projectDir}
}

// Do issues one request, standing the server up on first use.
//
// An HTTP error status is a normal return, not an error: asserting a 403 or a
// 404 is the point of the verb. Only a transport-level failure errors.
func (c *APIClient) Do(ctx context.Context, req docs.APIRequest) (docs.APIResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.proj == nil {
		dir := req.ProjectDir
		if dir == "" {
			dir = c.projectDir
		}
		if dir == "" {
			return docs.APIResponse{}, errors.New("api{}: no project directory to serve " +
				"(build with --project)")
		}
		p, err := standUp(ctx, dir, req.Seed, false)
		if err != nil {
			return docs.APIResponse{}, err
		}
		c.proj = p
	} else if err := c.proj.syncSeed(ctx, req.Seed); err != nil {
		return docs.APIResponse{}, fmt.Errorf("api{}: seeding: %w", err)
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, c.proj.server.URL+req.Path, body)
	if err != nil {
		return docs.APIResponse{}, fmt.Errorf("api{path=%q}: %w", req.Path, err)
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

	resp, err := c.proj.server.Client().Do(httpReq)
	if err != nil {
		return docs.APIResponse{}, fmt.Errorf("api{path=%q}: %w", req.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return docs.APIResponse{}, fmt.Errorf("api{path=%q}: reading body: %w", req.Path, err)
	}
	return docs.APIResponse{Status: resp.StatusCode, Body: string(raw)}, nil
}

// Close tears down the temp project and server if one was stood up.
func (c *APIClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proj != nil {
		c.proj.close()
		c.proj = nil
	}
	return nil
}
