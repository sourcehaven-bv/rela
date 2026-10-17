package dataentry

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// liveShape is the kind of GET resource a live subscription tracks.
type liveShape int

const (
	liveEntity   liveShape = iota + 1 // /api/v1/{plural}/{id}
	liveList                          // /api/v1/{plural}
	liveView                          // /api/v1/_views/{type}/{id}
	livePresence                      // /api/v1/{plural}/{id}/_presence
	liveChannels                      // /api/v1/{plural}/{id}/_channels
	liveChannel                       // /api/v1/{plural}/{id}/_channels/{type}[/{name}]
)

// livePath is a subscription path parsed into checked parts. The replay URL
// is rebuilt from these parts by [livePath.String], never taken from the
// client, so a subscription can only ever reach the six read handlers.
type livePath struct {
	shape  liveShape
	plural string // entity, list and presence
	typ    string // view: the entry entity's type
	id     string // entity, view, presence and channel
	chtype string // channel: the channel type
	name   string // channel: the channel name, empty for cardinality one
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
	// livePresenceKeys: the presence route reads no parameter but the world
	// that picks the face.
	livePresenceKeys = keySet("world")
	// liveChannelsKeys and liveChannelKeys: the channel list filters on
	// state; both resolve the anchor in a world.
	liveChannelsKeys = keySet("state", "world")
	liveChannelKeys  = keySet("world")
	liveListKeys     = keySet(
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

	lp, allowed, err := liveShapeOf(strings.Split(rest, "/"), meta)
	if err != nil {
		return livePath{}, err
	}

	for k := range query {
		if !allowed[k] && (lp.shape != liveList || !liveFilterKey.MatchString(k)) {
			return livePath{}, fmt.Errorf("%w: query parameter %q is not accepted", errLivePath, k)
		}
	}
	lp.query = query
	return lp, nil
}

// liveShapeOf parses the path segments after /api/v1/ into a shape and the
// query keys it accepts.
func liveShapeOf(segs []string, meta *metamodel.Metamodel) (livePath, map[string]bool, error) {
	switch {
	case len(segs) == 3 && segs[0] == "_views":
		if !isSafePathSegment(segs[1]) || !meta.HasEntityType(segs[1]) {
			return livePath{}, nil, fmt.Errorf("%w: unknown view entity type", errLivePath)
		}
		if err := checkLiveID(segs[2]); err != nil {
			return livePath{}, nil, err
		}
		return livePath{shape: liveView, typ: segs[1], id: segs[2]}, liveViewKeys, nil
	case len(segs) >= 3 && segs[2] == channelsSegment:
		return liveChannelShape(segs, meta)
	case len(segs) == 3 && segs[2] == "_presence":
		if err := checkLivePlural(segs[0], meta); err != nil {
			return livePath{}, nil, err
		}
		if err := checkLiveID(segs[1]); err != nil {
			return livePath{}, nil, err
		}
		if err := checkPresenceFace(segs[0], segs[1], meta); err != nil {
			return livePath{}, nil, err
		}
		return livePath{shape: livePresence, plural: segs[0], id: segs[1]}, livePresenceKeys, nil
	case len(segs) == 1 || len(segs) == 2:
		if err := checkLivePlural(segs[0], meta); err != nil {
			return livePath{}, nil, err
		}
		if len(segs) == 1 {
			return livePath{shape: liveList, plural: segs[0]}, liveListKeys, nil
		}
		if err := checkLiveID(segs[1]); err != nil {
			return livePath{}, nil, err
		}
		return livePath{shape: liveEntity, plural: segs[0], id: segs[1]}, liveEntityKeys, nil
	default:
		return livePath{}, nil, fmt.Errorf("%w: not an entity, list, view, presence or channel", errLivePath)
	}
}

// namedChannelSegs is the segment count of a named channel head,
// {plural}/{id}/_channels/{type}/{name}, the longest live channel shape.
const namedChannelSegs = 5

// liveChannelShape parses {plural}/{id}/_channels[/{type}[/{name}]]. The
// channel head and the channel list are the only live channel shapes: a
// client that gets a frame for a head fetches messages?after_rev= itself,
// so a messages path is refused. Parsing is structural, as in the channel
// routes: a minted name is 13 base36 digits and is never `messages`.
func liveChannelShape(segs []string, meta *metamodel.Metamodel) (livePath, map[string]bool, error) {
	if err := checkLivePlural(segs[0], meta); err != nil {
		return livePath{}, nil, err
	}
	if err := checkLiveID(segs[1]); err != nil {
		return livePath{}, nil, err
	}
	lp := livePath{shape: liveChannels, plural: segs[0], id: segs[1]}
	if len(segs) == 3 {
		return lp, liveChannelsKeys, nil
	}
	if len(segs) > namedChannelSegs || segs[len(segs)-1] == messagesSegment {
		return livePath{}, nil, fmt.Errorf("%w: subscribe to the channel head, not to its messages", errLivePath)
	}
	lp.shape, lp.chtype = liveChannel, segs[3]
	if len(segs) == namedChannelSegs {
		lp.name = segs[4]
	}
	addr, err := entity.ParseAddress(lp.id)
	if err != nil {
		// coverage-ignore: checkLiveID accepted lp.id.
		return livePath{}, nil, fmt.Errorf("%w: invalid entity id", errLivePath)
	}
	ref, _ := addr.Named()
	key := channels.Key{Anchor: channels.Anchor{ID: addr.ID(), Face: ref.Face}, Type: lp.chtype, Name: lp.name}
	if key.Validate() != nil {
		return livePath{}, nil, fmt.Errorf("%w: invalid channel type or name", errLivePath)
	}
	return lp, liveChannelKeys, nil
}

// checkLivePlural accepts the plural of an entity type.
func checkLivePlural(seg string, meta *metamodel.Metamodel) error {
	if strings.HasPrefix(seg, "_") || entityTypeForPlural(meta, seg) == "" {
		return fmt.Errorf("%w: not an entity collection", errLivePath)
	}
	return nil
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

// checkPresenceFace refuses a bare id on a type that declares faces.
// Presence is per face, and the presence route answers such an address with
// `face_required`, so the subscription could never join. Which types have
// faces is in the metamodel, which is not a secret.
func checkPresenceFace(plural, seg string, meta *metamodel.Metamodel) error {
	addr, err := entity.ParseAddress(seg)
	if err != nil {
		// coverage-ignore: checkLiveID accepted seg.
		return fmt.Errorf("%w: invalid entity id", errLivePath)
	}
	if _, named := addr.Named(); named || len(meta.Entities[entityTypeForPlural(meta, plural)].Faces) == 0 {
		return nil
	}
	return fmt.Errorf("%w: this type has faces; a presence path names one (ID@face)", errLivePath)
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
	case livePresence:
		p = "/api/v1/" + url.PathEscape(lp.plural) + "/" + url.PathEscape(lp.id) + "/_presence"
	case liveChannels, liveChannel:
		p = "/api/v1/" + url.PathEscape(lp.plural) + "/" + url.PathEscape(lp.id) + "/" + channelsSegment
		if lp.chtype != "" {
			p += "/" + url.PathEscape(lp.chtype)
		}
		if lp.name != "" {
			p += "/" + url.PathEscape(lp.name)
		}
	}
	if q := lp.query.Encode(); q != "" {
		p += "?" + q
	}
	return p
}

// presenceID is the entity id a presence subscription names, without its
// face, or "" for every other shape. A replay may join presence only for the
// entity with this id (M4).
func (lp livePath) presenceID() string {
	if lp.shape != livePresence {
		return ""
	}
	addr, err := entity.ParseAddress(lp.id)
	if err != nil {
		// coverage-ignore: parseLivePath accepted lp.id through checkLiveID.
		return ""
	}
	return addr.ID()
}

// isChannel reports whether lp is a channel head or channel list. Such a
// subscription is replayed by a topic round over the channels that changed.
func (lp livePath) isChannel() bool { return lp.shape == liveChannels || lp.shape == liveChannel }
