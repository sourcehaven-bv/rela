package dataentry

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// liveShape is the kind of GET resource a live subscription tracks.
type liveShape int

const (
	liveEntity liveShape = iota + 1 // /api/v1/{plural}/{id}
	liveList                        // /api/v1/{plural}
	liveView                        // /api/v1/_views/{type}/{id}
)

// livePath is a subscription path parsed into checked parts. The replay URL
// is rebuilt from these parts by [livePath.String], never taken from the
// client, so a subscription can only ever reach the three read handlers.
type livePath struct {
	shape  liveShape
	plural string // entity and list
	typ    string // view: the entry entity's type
	id     string // entity and view
	query  url.Values
}

// Query keys each shape accepts. They mirror what the SPA sends: getEntity
// and fetchView in frontend/src/api, and ListParams in
// frontend/src/types/entity.ts. A key outside the set is refused rather than
// passed on, so a subscription cannot ask a read handler for anything the
// SPA does not.
//
// The list's free-text q is deliberately absent. It queries the unscoped
// searcher with a hit cap and gates the hits afterwards, so a hidden write can
// push a visible row out of the result; a live frame would turn that into a
// timed signal. The MCP watch tool refuses search_entities for the same reason.
var (
	liveEntityKeys = keySet("include", "fields", "world")
	liveViewKeys   = keySet("world")
	liveListKeys   = keySet(
		"page", "per_page", "cursor", "sort", "fields", "include", "comment_counts",
		"world", "list_id", "query_scope", "scope_page", "scope_tab", "anchor",
		"relation", "direction",
	)
	// liveFilterKey matches the list filter parameters, filter[prop] and
	// filter[prop][op]. The list handler validates the property and operator.
	liveFilterKey = regexp.MustCompile(`^filter\[[A-Za-z0-9_.-]+\](\[[a-z_]+\])?$`)
)

func keySet(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

var errLivePath = errors.New("unsupported subscription path")

// parseLivePath parses raw, a relative GET URL the client subscribes to.
// meta supplies the plural names that may appear. Every rejection wraps
// errLivePath and says why; the reasons name only the client's own input.
func parseLivePath(raw string, meta *metamodel.Metamodel) (livePath, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return livePath{}, fmt.Errorf("%w: %w", errLivePath, err)
	}
	if u.Scheme != "" || u.Host != "" || u.User != nil || u.Opaque != "" || u.Fragment != "" {
		return livePath{}, fmt.Errorf("%w: must be a relative path", errLivePath)
	}
	// RawPath is set only when the path holds an escape that matters, such
	// as %2F. Refusing it keeps the segments below exactly what was sent.
	if u.RawPath != "" {
		return livePath{}, fmt.Errorf("%w: escaped characters in path", errLivePath)
	}
	if u.Path != path.Clean(u.Path) {
		return livePath{}, fmt.Errorf("%w: path is not clean", errLivePath)
	}
	rest, ok := strings.CutPrefix(u.Path, "/api/v1/")
	if !ok {
		return livePath{}, fmt.Errorf("%w: must start with /api/v1/", errLivePath)
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return livePath{}, fmt.Errorf("%w: %w", errLivePath, err)
	}

	segs := strings.Split(rest, "/")
	var lp livePath
	var allowed map[string]bool
	switch {
	case len(segs) == 3 && segs[0] == "_views":
		if !isSafePathSegment(segs[1]) || !meta.HasEntityType(segs[1]) {
			return livePath{}, fmt.Errorf("%w: unknown view entity type", errLivePath)
		}
		if err := checkLiveID(segs[2]); err != nil {
			return livePath{}, err
		}
		lp = livePath{shape: liveView, typ: segs[1], id: segs[2]}
		allowed = liveViewKeys
	case len(segs) == 1 || len(segs) == 2:
		if strings.HasPrefix(segs[0], "_") || entityTypeForPlural(meta, segs[0]) == "" {
			return livePath{}, fmt.Errorf("%w: not an entity collection", errLivePath)
		}
		lp = livePath{shape: liveList, plural: segs[0]}
		allowed = liveListKeys
		if len(segs) == 2 {
			if err := checkLiveID(segs[1]); err != nil {
				return livePath{}, err
			}
			lp = livePath{shape: liveEntity, plural: segs[0], id: segs[1]}
			allowed = liveEntityKeys
		}
	default:
		return livePath{}, fmt.Errorf("%w: not an entity, list or view", errLivePath)
	}

	for k := range query {
		if !allowed[k] && (lp.shape != liveList || !liveFilterKey.MatchString(k)) {
			return livePath{}, fmt.Errorf("%w: query parameter %q is not accepted", errLivePath, k)
		}
	}
	lp.query = query
	return lp, nil
}

// checkLiveID accepts an entity address ("ID" or "ID@face") and refuses a
// segment that names a sub-resource, such as _export or _history.
func checkLiveID(seg string) error {
	if strings.HasPrefix(seg, "_") {
		return fmt.Errorf("%w: not an entity id", errLivePath)
	}
	if _, err := entity.ParseAddress(seg); err != nil {
		return fmt.Errorf("%w: invalid entity id", errLivePath)
	}
	return nil
}

// String is the replay URL, rebuilt from the parsed parts.
func (lp livePath) String() string {
	var p string
	switch lp.shape {
	case liveEntity:
		p = "/api/v1/" + url.PathEscape(lp.plural) + "/" + url.PathEscape(lp.id)
	case liveList:
		p = "/api/v1/" + url.PathEscape(lp.plural)
	case liveView:
		p = "/api/v1/_views/" + url.PathEscape(lp.typ) + "/" + url.PathEscape(lp.id)
	}
	if q := lp.query.Encode(); q != "" {
		p += "?" + q
	}
	return p
}
