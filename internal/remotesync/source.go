package remotesync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
)

// Caps on what one fetch reads from the source.
const (
	maxSchemaBytes   = 8 << 20
	maxEntityBytes   = 4 << 20
	maxRelationBytes = 1 << 20
)

// Getter reads one route of the source's API: path is relative to
// /api/v1/ with its segments escaped. It returns an error wrapping
// [httpremote.ErrNotFound] for a 404 and [httpremote.ErrDenied] for a 401 or
// 403. *httpremote.Remote implements it.
type Getter interface {
	Get(ctx context.Context, path string, limit int64) ([]byte, error)
}

// Source is the remote as a run reads it: the reconcile endpoint and the read
// routes. *httpremote.Remote implements it.
type Source interface {
	reconclient.Remote
	Getter
}

// Pusher sends requests to the remote's API that need the ETag: the reads
// and writes of a remote with push. It returns the answer for every status
// a row can explain. It fails for a transport error, a redirect, a 401
// (wrapping [httpremote.ErrDenied]), a 429 or 503
// ([reconclient.RefusedError]) and a success body over the limit; see
// [httpremote.Remote.Send]. *httpremote.Remote implements it.
type Pusher interface {
	Send(ctx context.Context, method, path string, body []byte, ifMatch string, limit int64) (httpremote.Answer, error)
}

// errProtocol marks a source answer that is not what was asked for.
var errProtocol = errors.New("remotesync: invalid answer from the source")

// remoteEntity is the part of a fetched entity a pull reads.
type remoteEntity struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Content    *string        `json:"content"`
	// Redacted is present on every per-entity read of a source that
	// reports redaction; nil means the source does not say.
	Redacted *[]string `json:"_redacted"`
	// Updated is when the row last changed, in RFC 3339. The source serves
	// it only when nothing on the row is hidden from this instance.
	Updated string `json:"_updated"`
}

// remoteRelation is the part of a fetched relation a pull reads.
type remoteRelation struct {
	From     string         `json:"from"`
	Type     string         `json:"type"`
	To       string         `json:"to"`
	Meta     map[string]any `json:"meta"`
	Content  *string        `json:"content"`
	Redacted *[]string      `json:"_redacted"`
	Updated  string         `json:"_updated"`
}

// plurals reads the source schema and returns the route name of each
// source entity type in m.
func plurals(ctx context.Context, g Getter, m *Mapping) (map[string]string, error) {
	raw, err := g.Get(ctx, "_schema", maxSchemaBytes)
	if err != nil {
		return nil, err
	}
	var schema struct {
		Entities map[string]struct {
			Plural string `json:"plural"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("%w: the schema is not valid JSON", errProtocol)
	}
	out := make(map[string]string, len(m.Entities))
	for _, src := range sortedKeys(m.Entities) {
		t, ok := schema.Entities[src]
		switch {
		case !ok:
			return nil, fmt.Errorf("remotesync: the source has no entity type %q", src)
		case !validTypeName(t.Plural):
			return nil, fmt.Errorf("%w: the plural of %q is not a valid name", errProtocol, src)
		}
		out[src] = url.PathEscape(t.Plural)
	}
	return out, nil
}

// fetchEntity reads one source entity and checks it is the one asked for.
func fetchEntity(ctx context.Context, g Getter, plural, typ, id string) (remoteEntity, error) {
	raw, err := g.Get(ctx, entityPath(plural, id), maxEntityBytes)
	if err != nil {
		return remoteEntity{}, err
	}
	return checkEntity(raw, typ, id)
}

// entityPath is the read and update route of one entity; plural is escaped.
func entityPath(plural, id string) string { return plural + "/" + url.PathEscape(id) }

// fetchTagged reads one source entity with its ETag, through p. A 404 wraps
// [httpremote.ErrNotFound] and a 403 [httpremote.ErrDenied], as a plain
// fetch does.
func fetchTagged(ctx context.Context, p Pusher, plural, typ, id string) (remoteEntity, string, error) {
	ans, err := p.Send(ctx, http.MethodGet, entityPath(plural, id), nil, "", maxEntityBytes)
	switch {
	case err != nil:
		return remoteEntity{}, "", err
	case ans.Status == http.StatusNotFound:
		return remoteEntity{}, "", fmt.Errorf("%w: answered 404", httpremote.ErrNotFound)
	case ans.Status == http.StatusForbidden:
		return remoteEntity{}, "", fmt.Errorf("%w: answered 403", httpremote.ErrDenied)
	case ans.Status != http.StatusOK:
		return remoteEntity{}, "", fmt.Errorf("remotesync: the source answered %d: %s", ans.Status,
			reconclient.Sanitize(string(ans.Body)))
	}
	e, err := checkEntity(ans.Body, typ, id)
	return e, ans.ETag, err
}

// checkEntity decodes a fetched entity and checks it is the one asked for.
func checkEntity(raw []byte, typ, id string) (remoteEntity, error) {
	var e remoteEntity
	if err := decode(raw, &e); err != nil {
		return remoteEntity{}, err
	}
	if e.ID != id || e.Type != typ {
		return remoteEntity{}, fmt.Errorf("%w: asked for %s/%s, got another row", errProtocol, typ, id)
	}
	return e, nil
}

// fetchRelation reads one source relation and checks it is the one asked
// for.
func fetchRelation(ctx context.Context, g Getter, fromPlural, from, typ, to string) (remoteRelation, error) {
	path := fromPlural + "/" + url.PathEscape(from) + "/relations/" + url.PathEscape(typ) + "/" + url.PathEscape(to)
	raw, err := g.Get(ctx, path, maxRelationBytes)
	if err != nil {
		return remoteRelation{}, err
	}
	return checkRelation(raw, from, typ, to)
}

// checkRelation decodes a fetched relation and checks it is the one asked
// for.
func checkRelation(raw []byte, from, typ, to string) (remoteRelation, error) {
	var r remoteRelation
	if err := decode(raw, &r); err != nil {
		return remoteRelation{}, err
	}
	if r.From != from || r.Type != typ || r.To != to {
		return remoteRelation{}, fmt.Errorf("%w: asked for %s--%s--%s, got another row", errProtocol, from, typ, to)
	}
	return r, nil
}

// decode reads JSON with numbers kept as their text, so a large integer
// keeps its digits; [syncmerge.Canonical] parses the text.
func decode(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: not valid JSON", errProtocol)
	}
	return nil
}

// plain replaces every json.Number in v with its text.
func plain(v any) any {
	switch t := v.(type) {
	case json.Number:
		return t.String()
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = plain(x)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = plain(x)
		}
		return out
	}
	return v
}
