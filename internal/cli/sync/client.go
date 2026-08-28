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

// EntityBody is the JSON fetch payload for an entity, decoded from the
// authorized /api/v1 GET (TKT-8P1TM7). Properties carries the VISIBLE values
// only; Redacted names the properties the primary withheld by field-level ACL
// (the `_redacted` wire field, DEC-T0XIWQ). The two together let the replica
// distinguish a hidden field (named in Redacted → leave the local copy alone)
// from a genuinely deleted one (in neither → unset locally) when it splices the
// fetch onto its raw local record. Redacted is a pointer so "absent" (a shape
// with no field affordances) is distinct from "present and empty" (evaluated,
// nothing hidden).
type EntityBody struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
	Redacted   *[]string      `json:"_redacted,omitempty"`
}

// RelationBody is the JSON fetch payload for a relation. Like EntityBody,
// Properties carries the VISIBLE meta values and Redacted names the withheld
// ones (TKT-8P1TM7) so the replica can splice relation meta without erasing
// hidden values it isn't entitled to.
type RelationBody struct {
	From       string         `json:"from"`
	Type       string         `json:"type"`
	To         string         `json:"to"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
	Redacted   *[]string      `json:"_redacted,omitempty"`
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

// GetEntity fetches an entity through the authorized /api/v1 read path
// (TKT-8P1TM7). The response body is the redacted v1 entity — VISIBLE property
// values plus the `_redacted` names — and the Hash is the primary's opaque
// ETag (its conflict token, echoed back as If-Match on the next write, never
// re-derived locally). plural is the type's /api/v1 route segment, resolved from
// the primary's schema by the engine.
func (c *Client) GetEntity(ctx context.Context, plural, id string) (*FetchedEntity, error) {
	resp, err := c.get(ctx, entitySegments(plural, id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp, "fetch entity "+id)
	}
	var v v1EntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode entity %s: %w", id, err)
	}
	return &FetchedEntity{Body: v.toEntityBody(), Hash: resp.Header.Get("ETag")}, nil
}

// v1EntityResponse decodes the fields of an /api/v1 entity response the sync
// client uses. It mirrors the server's apiwire/v1 Entity shape but is a local,
// minimal decode (the CLI must not import the server wire package).
type v1EntityResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Content    string         `json:"content,omitempty"`
	Redacted   *[]string      `json:"_redacted,omitempty"`
}

func (v v1EntityResponse) toEntityBody() EntityBody {
	// Identical field set/order, so a direct conversion is exact.
	return EntityBody(v)
}

// GetRelation fetches a relation through the /api/v1 single-relation read that
// carries the relation body + a relation-level ETag (RR-SYNCR1). fromPlural is
// the source entity type's route plural. The response is redacted relation meta
// + `_redacted` names, and Hash is the primary's opaque relation ETag.
func (c *Client) GetRelation(ctx context.Context, fromPlural, from, relType, to string) (*FetchedRelation, error) {
	resp, err := c.get(ctx, relationSegments(fromPlural, from, relType, to))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp, fmt.Sprintf("fetch relation %s/%s/%s", from, relType, to))
	}
	var v v1RelationResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode relation: %w", err)
	}
	return &FetchedRelation{Body: v.toRelationBody(from, relType, to), Hash: resp.Header.Get("ETag")}, nil
}

// v1RelationResponse decodes the /api/v1 single-relation read (RR-SYNCR1). The
// from/type/to are known from the request path, so the body carries meta +
// content + `_redacted` only.
type v1RelationResponse struct {
	Properties map[string]any `json:"meta"`
	Content    string         `json:"content,omitempty"`
	Redacted   *[]string      `json:"_redacted,omitempty"`
}

func (v v1RelationResponse) toRelationBody(from, relType, to string) RelationBody {
	return RelationBody{
		From: from, Type: relType, To: to,
		Properties: v.Properties, Content: v.Content, Redacted: v.Redacted,
	}
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
	// CreatedConcurrently distinguishes a 409 (a create-intent push lost a
	// race to a concurrent first-create of the same id on the multi-writer
	// backend) from the ordinary 412 conflict (the client declared a base that
	// no longer matches). Both are conflicts that HALT only the one record and
	// let the run continue; the flag only sharpens the operator-facing message.
	CreatedConcurrently bool
	Invalid             bool   // 422: content rejected by validation (NOT a conflict)
	Hash                string // new hash (primary's opaque ETag) on Applied
	// CreatedID is the primary-minted id returned by a create (POST /api/v1/
	// {plural}). The replica adopts it and renames its temp-id local doc
	// (TKT-8P1TM7). Empty for updates/deletes.
	CreatedID string
	Detail    string // human-readable detail (validation message, etc.)
}

// v1PatchEntity is the partial-update body the replica PUSHes to
// PATCH /api/v1/{plural}/{id} (TKT-8P1TM7). It names only the VISIBLE properties
// the replica holds — the primary merges them onto its raw record, so any field
// the replica does not name (including the primary's hidden fields) is
// preserved. PropertiesUnset carries genuine local deletions of VISIBLE fields.
// This is the push-side symmetry of the pull splice: never a whole-record
// replace, so a redacted replica cannot erase the primary's hidden data.
type v1PatchEntity struct {
	Properties      map[string]any `json:"properties,omitempty"`
	PropertiesUnset []string       `json:"properties_unset,omitempty"`
	Content         *string        `json:"content,omitempty"`
}

// v1CreateEntity is the create body POSTed to /api/v1/{plural} WITHOUT an id —
// the primary mints the real id and returns it (the replica adopts it and
// renames locally, TKT-8P1TM7). Content is a plain string on create.
type v1CreateEntity struct {
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
}

// PatchEntity pushes a partial entity update through PATCH /api/v1/{plural}/{id}
// under If-Match (the primary's opaque ETag baseline). plural is the type route.
func (c *Client) PatchEntity(
	ctx context.Context, plural, id string, body v1PatchEntity, ifMatch string,
) (*PushResult, error) {
	return c.patch(ctx, entitySegments(plural, id), body, ifMatch)
}

// CreateEntity pushes a create through POST /api/v1/{plural} (no id) and returns
// the primary-minted id in the PushResult (CreatedID) for the replica to adopt.
func (c *Client) CreateEntity(ctx context.Context, plural string, body v1CreateEntity) (*PushResult, error) {
	return c.create(ctx, []string{"api", "v1", plural}, body)
}

// DeleteEntity deletes an entity through DELETE /api/v1/{plural}/{id}.
func (c *Client) DeleteEntity(ctx context.Context, plural, id, ifMatch string) (*PushResult, error) {
	return c.delete(ctx, entitySegments(plural, id), ifMatch)
}

// v1RelationWrite is the create/update body for the dedicated single-relation
// endpoints (POST/PATCH /api/v1/{fromPlural}/{from}/relations/{relType}/{to}).
// The v1 relation write body names the target id + meta; the replica sends the
// VISIBLE meta only (symmetry with the pull splice).
type v1RelationWrite struct {
	ID   string         `json:"id"`             // the TO endpoint id
	Meta map[string]any `json:"meta,omitempty"` // relation properties
}

// PutRelation upserts a relation through the /api/v1 relation write endpoint.
// The server create/update is idempotent on the (from,type,to) triple, so a
// single PATCH-style call covers both create and update from the replica's
// point of view. fromPlural is the source entity type's route plural.
func (c *Client) PutRelation(
	ctx context.Context, fromPlural string, body RelationBody, ifMatch string,
) (*PushResult, error) {
	return c.patch(ctx, relationSegments(fromPlural, body.From, body.Type, body.To),
		v1RelationWrite{ID: body.To, Meta: body.Properties}, ifMatch)
}

// DeleteRelation deletes a relation through the /api/v1 relation endpoint.
func (c *Client) DeleteRelation(
	ctx context.Context, fromPlural, from, relType, to, ifMatch string,
) (*PushResult, error) {
	return c.delete(ctx, relationSegments(fromPlural, from, relType, to), ifMatch)
}

// --- internal request plumbing ---

func (c *Client) patch(ctx context.Context, segments []string, body any, ifMatch string) (*PushResult, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPatch, segments, nil, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	return c.pushResult(req)
}

// create POSTs a create body (no id) and maps the 201 response — the primary
// mints the id and returns it in the body, which the replica adopts. A create
// carries no If-Match (there is no prior version); a concurrent first-create of
// the same natural key surfaces as the primary's own conflict status.
func (c *Client) create(ctx context.Context, segments []string, body any) (*PushResult, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, segments, nil, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		var created struct {
			ID string `json:"id"`
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		_ = json.Unmarshal(data, &created)
		return &PushResult{Applied: true, Hash: resp.Header.Get("ETag"), CreatedID: created.ID}, nil
	case http.StatusConflict:
		return &PushResult{Conflict: true, CreatedConcurrently: true}, nil
	case http.StatusUnprocessableEntity:
		return &PushResult{Invalid: true, Detail: c.errorDetail(resp)}, nil
	default:
		return nil, c.statusError(resp, "create "+req.URL.Path)
	}
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
// from ETag); 409 -> conflict (a create raced a concurrent first-create,
// CreatedConcurrently set); 422 -> invalid; anything else -> an error (403/404/
// 5xx surfaced via statusError so auth failures are distinct from conflicts).
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
	case http.StatusConflict:
		// 409: a create-intent push raced a concurrent first-create of the same
		// id (postgres multi-writer). Like a 412, this is a per-record conflict
		// that HALTS this record and lets the run continue — it is NOT a
		// transport/auth error, so it must NOT fall through to statusError (which
		// would abort the whole push run). The flag lets the caller emit a
		// create-specific message.
		return &PushResult{Conflict: true, CreatedConcurrently: true}, nil
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
// https://host/rela/api/v1/tickets/<id>, not https://host/api/v1/... .
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
// a record's /api/v1 URL. newRequest escapes them via JoinPath.
//
// Entities route by type plural: /api/v1/{plural}/{id}. Relations use the
// dedicated sync-oriented single-relation read/write route that carries the
// relation body + a relation-level ETag (RR-SYNCR1): the source entity's plural,
// then the relation triple.
func entitySegments(plural, id string) []string { return []string{"api", "v1", plural, id} }
func relationSegments(fromPlural, from, relType, to string) []string {
	return []string{"api", "v1", fromPlural, from, "relations", relType, to}
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	// The destination is operator configuration, not attacker input: the base
	// URL is the `rela sync push/pull --remote` flag (env RELA_REMOTE), supplied
	// by whoever invokes the CLI, and NewClient requires it to be absolute.
	// Choosing which rela-server to sync against is the entire point of the
	// command, so a host allowlist would defeat it. Remote-controlled data never
	// reaches the destination: the path is built from JoinPath over segments this
	// package derives from local record ids, so a hostile server's manifest can
	// influence the path (escaped exactly once by JoinPath) but never the
	// scheme/host. The trust boundary is the operator's shell, the same one that
	// already grants full local filesystem access.
	//nolint:gosec // G704: destination comes from the operator's --remote/RELA_REMOTE, not from request input.
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
