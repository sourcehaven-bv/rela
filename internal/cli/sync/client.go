package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client talks to a remote rela-server's /api/sync/ API. It is a thin wire
// adapter: it serializes records, sends conditional requests, and maps HTTP
// status to typed outcomes. All higher-level policy (dirty detection, topo
// ordering, conflict halting) lives in the push/pull commands.
//
// The bearer token (if any) is held only in memory and attached as an
// Authorization header; it is NEVER placed in a URL, an error message, or a log
// line.
type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

// NewClient builds a sync client for the proxy-fronted base URL. token may be
// empty (loopback/dev with no proxy); when set it is sent as a bearer on every
// request. The base URL must be absolute.
func NewClient(base, token string, httpClient *http.Client) (*Client, error) {
	if base == "" {
		return nil, errors.New("sync: --remote base URL is required")
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("sync: invalid --remote URL: %w", err)
	}
	if !u.IsAbs() {
		return nil, fmt.Errorf("sync: --remote URL must be absolute, got %q", base)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{base: u, token: token, http: httpClient}, nil
}

// --- wire DTOs (must match the server's in internal/dataentry/sync.go) ---

type manifestResponse struct {
	Changes []ManifestChange `json:"changes"`
	Cursor  string           `json:"cursor"`
}

// ManifestChange is one entry in the server's change feed since a cursor.
// Kind is "e" (entity) or "r" (relation); ID is the entity id or the
// "from/type/to" relation key; Deleted marks a tombstone.
type ManifestChange struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Typ     string `json:"typ,omitempty"`
	Deleted bool   `json:"deleted"`
}

// EntityBody is the JSON push/fetch payload for an entity.
type EntityBody struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
}

// RelationBody is the JSON push/fetch payload for a relation.
type RelationBody struct {
	From       string         `json:"from"`
	Type       string         `json:"type"`
	To         string         `json:"to"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
}

// Manifest is the decoded change feed plus the next cursor to persist.
type Manifest struct {
	Changes []ManifestChange
	Cursor  string
}

// Manifest fetches the change feed since cursor. An empty cursor requests the
// full manifest (first sync). The returned cursor is opaque and stored verbatim.
func (c *Client) Manifest(ctx context.Context, cursor string) (*Manifest, error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	req, err := c.newRequest(ctx, http.MethodGet, []string{"api", "sync", "manifest"}, q, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp, "fetch manifest")
	}
	var mr manifestResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &Manifest{Changes: mr.Changes, Cursor: mr.Cursor}, nil
}

// FetchedEntity / FetchedRelation pair a fetched body with its server hash (the
// ETag), which the caller records in the index after a successful local apply.
type FetchedEntity struct {
	Body EntityBody
	Hash string
}
type FetchedRelation struct {
	Body RelationBody
	Hash string
}

// GetEntity fetches an entity's full content and its current server hash (ETag).
func (c *Client) GetEntity(ctx context.Context, id string) (*FetchedEntity, error) {
	resp, err := c.get(ctx, entitySegments(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp, "fetch entity "+id)
	}
	var b EntityBody
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, fmt.Errorf("decode entity %s: %w", id, err)
	}
	return &FetchedEntity{Body: b, Hash: resp.Header.Get("ETag")}, nil
}

// GetRelation fetches a relation's full content and its current server hash.
func (c *Client) GetRelation(ctx context.Context, from, relType, to string) (*FetchedRelation, error) {
	resp, err := c.get(ctx, relationSegments(from, relType, to))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp, fmt.Sprintf("fetch relation %s/%s/%s", from, relType, to))
	}
	var b RelationBody
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, fmt.Errorf("decode relation: %w", err)
	}
	return &FetchedRelation{Body: b, Hash: resp.Header.Get("ETag")}, nil
}

// PushResult is the typed outcome of a conditional PUT/DELETE. Exactly one of
// the boolean states is true; on Applied, Hash carries the new server hash.
//
// A Conflict carries no server hash: --force is a SEPARATE command invocation
// from the conflicting push, so any in-process hash would be stale by the time
// the operator runs it. ForcePush therefore re-reads the current remote hash at
// force time (see force.go).
type PushResult struct {
	Applied  bool
	Conflict bool
	Invalid  bool   // 422: content rejected by validation (NOT a conflict)
	Hash     string // new hash on Applied
	Detail   string // human-readable detail (validation message, etc.)
}

// PutEntity pushes an entity conditionally. ifMatch is the index hash (the base
// the client edited); empty means "expect this to not yet exist" (first create).
func (c *Client) PutEntity(ctx context.Context, body EntityBody, ifMatch string) (*PushResult, error) {
	return c.put(ctx, entitySegments(body.ID), body, ifMatch)
}

// PutRelation pushes a relation conditionally.
func (c *Client) PutRelation(ctx context.Context, body RelationBody, ifMatch string) (*PushResult, error) {
	return c.put(ctx, relationSegments(body.From, body.Type, body.To), body, ifMatch)
}

// DeleteEntity / DeleteRelation conditionally delete a record. ifMatch must be
// the record's current hash (the server rejects a blind delete with 412).
func (c *Client) DeleteEntity(ctx context.Context, id, ifMatch string) (*PushResult, error) {
	return c.delete(ctx, entitySegments(id), ifMatch)
}
func (c *Client) DeleteRelation(ctx context.Context, from, relType, to, ifMatch string) (*PushResult, error) {
	return c.delete(ctx, relationSegments(from, relType, to), ifMatch)
}

// --- internal request plumbing ---

func (c *Client) put(ctx context.Context, segments []string, body any, ifMatch string) (*PushResult, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPut, segments, nil, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	return c.pushResult(req)
}

func (c *Client) delete(ctx context.Context, segments []string, ifMatch string) (*PushResult, error) {
	req, err := c.newRequest(ctx, http.MethodDelete, segments, nil, nil)
	if err != nil {
		return nil, err
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	return c.pushResult(req)
}

func (c *Client) get(ctx context.Context, segments []string) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, segments, nil, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// pushResult maps the PUT/DELETE response status to a typed PushResult. 200 ->
// applied (+ new hash from the body or ETag); 412 -> conflict (+ server hash
// from ETag); 422 -> invalid; anything else -> an error (403/404/5xx surfaced
// via statusError so auth failures are distinct from conflicts).
func (c *Client) pushResult(req *http.Request) (*PushResult, error) {
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		hash := resp.Header.Get("ETag")
		if hash == "" {
			// PUT returns {"hash": ...}; DELETE returns {"deleted": ...}. Prefer
			// the ETag, fall back to the body's hash field for PUT.
			var bodyHash struct {
				Hash string `json:"hash"`
			}
			if data, rerr := io.ReadAll(resp.Body); rerr == nil {
				_ = json.Unmarshal(data, &bodyHash)
				hash = bodyHash.Hash
			}
		}
		return &PushResult{Applied: true, Hash: hash}, nil
	case http.StatusPreconditionFailed:
		return &PushResult{Conflict: true}, nil
	case http.StatusUnprocessableEntity:
		return &PushResult{Invalid: true, Detail: c.errorDetail(resp)}, nil
	default:
		return nil, c.statusError(resp, "push "+req.URL.Path)
	}
}

// newRequest builds a request against the base URL with the bearer token
// attached. The token is set as a header only — never echoed into the URL or
// returned in an error.
//
// segments are RAW (unescaped) path elements joined onto the base URL's
// existing path via url.URL.JoinPath, which percent-escapes each one exactly
// once. Joining (not replacing) preserves a base path prefix, so a base like
// https://host/rela/ keeps its prefix: the result is
// https://host/rela/api/sync/entities/<id>, not https://host/api/sync/... .
func (c *Client) newRequest(
	ctx context.Context, method string, segments []string, q url.Values, body io.Reader,
) (*http.Request, error) {
	full := c.base.JoinPath(segments...)
	if q != nil {
		full.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, full.String(), body)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

// entitySegments / relationSegments return the raw (unescaped) path elements for
// a record's sync URL. newRequest escapes them via JoinPath.
func entitySegments(id string) []string { return []string{"api", "sync", "entities", id} }
func relationSegments(from, relType, to string) []string {
	return []string{"api", "sync", "relations", from, relType, to}
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		// Never include req.URL with credentials — newRequest keeps the token in
		// the header, so the URL here is credential-free, but be explicit about
		// what we surface.
		return nil, fmt.Errorf("sync request to %s %s failed: %w", req.Method, req.URL.Path, err)
	}
	return resp, nil
}

// ErrNotFound signals a 404 from a sync GET — the record is absent on the
// server. Callers use it to distinguish "remote tombstone / first create" from a
// transport or auth failure (errors.Is).
var ErrNotFound = errors.New("sync: record not found on server")

// isNotFound reports whether err is (or wraps) ErrNotFound.
func isNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// statusError builds an error for an unexpected status, including the server's
// error code/message when present. A 401/403 is given an auth-specific hint so
// the operator can tell a proxy auth failure apart from a 412 conflict; a 404 is
// wrapped as ErrNotFound so callers can branch on absence.
func (c *Client) statusError(resp *http.Response, op string) error {
	detail := c.errorDetail(resp)
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%s: authentication failed (HTTP %d) — check RELA_SYNC_TOKEN / proxy config: %s",
			op, resp.StatusCode, detail)
	case http.StatusNotImplemented:
		return fmt.Errorf("%s: the server does not support sync (HTTP 501) — sync requires the postgres backend: %s",
			op, detail)
	default:
		return fmt.Errorf("%s: unexpected HTTP %d: %s", op, resp.StatusCode, detail)
	}
}

// maxErrorBody caps how much of an error response body we read for the detail
// message — enough for a JSON error envelope, bounded against a hostile body.
const maxErrorBody = 4096

// errorDetail extracts the server's {error, reason, detail} message when present,
// falling back to a short raw-body excerpt. Never returns request credentials.
func (c *Client) errorDetail(resp *http.Response) string {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil || len(data) == 0 {
		return resp.Status
	}
	var e struct {
		Error  string `json:"error"`
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal(data, &e) == nil && (e.Error != "" || e.Reason != "" || e.Detail != "") {
		parts := make([]string, 0, 3)
		for _, s := range []string{e.Error, e.Reason, e.Detail} {
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ": ")
	}
	return strings.TrimSpace(string(data))
}
