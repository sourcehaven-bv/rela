package dataentry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// PeerKeys verifies the peer key a sync peer presents (DEC-81161C3). The
// wiring site supplies it; *peerkey.Source implements it.
type PeerKeys interface {
	// IsPeerKey reports whether credential is presented as a peer key. Only
	// the prefix decides, so a malformed key is refused by Verify instead of
	// passing to another identity source.
	IsPeerKey(credential string) bool
	// Verify returns the peer name the key authenticates, or an error for a
	// malformed, unknown or revoked key.
	Verify(credential string) (name string, err error)
}

// SetPeerKeys lets sync peers authenticate with a peer key. Call it before
// NewRouter. A package function because App is at its plimsoll method cap.
//
// Nil: accepted; no request is then treated as a peer, and a peer key in
// the Authorization header takes the normal identity chain like any other
// credential.
func SetPeerKeys(a *App, keys PeerKeys) { a.peerKeys = keys }

// peerKeysFor returns the peer keys the router checks. Without an acl.yaml
// nothing would limit what a peer reads, so every peer key is refused: a
// peer reads only what acl.yaml grants it, and with no policy it reads
// nothing rather than everything.
//
// Nil: returned when a has no peer keys.
func peerKeysFor(a *App) PeerKeys {
	if a.peerKeys == nil {
		return nil
	}
	if d, ok := a.acl.(*acl.Declarative); !ok || d == nil {
		return refusePeerKeys{a.peerKeys}
	}
	return a.peerKeys
}

// refusePeerKeys recognizes peer keys like its wrapped PeerKeys and refuses
// every one with [errPeerKeysNeedACL].
type refusePeerKeys struct{ PeerKeys }

func (refusePeerKeys) Verify(string) (string, error) { return "", errPeerKeysNeedACL }

// errPeerKeysNeedACL reports a peer key on a server without an acl.yaml.
var errPeerKeysNeedACL = errors.New("dataentry: peer keys need an acl.yaml that grants the peer its reads")

// peerAuthenticatedKey marks a request whose principal the peer key check
// stamped. It is unexported, so no code outside this package can set it;
// inside it, only [peerKeyEdge.serve] does, after the key verified.
// [requireVerifiedJWT] passes a marked request through.
type peerAuthenticatedKey struct{}

// peerAuthenticated reports whether the peer key check stamped ctx.
func peerAuthenticated(ctx context.Context) bool {
	v, _ := ctx.Value(peerAuthenticatedKey{}).(bool)
	return v
}

// peerKeyEdge is the peer half of the identity stage in [stampAuditPrincipal].
// It holds the log sampler for refused keys, so a scanner cannot flood the
// log.
type peerKeyEdge struct {
	keys    PeerKeys
	refused logSampler
}

// peerCredential returns the peer key on r, if any. Every Authorization value
// is read: when one of them is a peer key, the request is a peer request, and
// more than one value makes it ambiguous, which is refused.
func (e *peerKeyEdge) peerCredential(r *http.Request) (credential string, present, ambiguous bool) {
	values := r.Header.Values("Authorization")
	for _, v := range values {
		if c := stripBearer(v); e.keys.IsPeerKey(c) {
			credential, present = c, true
		}
	}
	return credential, present, present && len(values) > 1
}

// serve handles a request that presents a peer key. The key alone decides
// the identity: a bad key answers 401 and never falls through to the header
// or the JWT, so a peer key cannot downgrade either. A good key stamps
// `peer:<name>` and marks ctx; the rest of the chain then runs unchanged,
// including the ACL request the Opener resolves from that principal.
//
// The route list is the read-only rule. It is applied after the key is
// verified, so an unauthenticated caller learns nothing from a 403.
func (e *peerKeyEdge) serve(w http.ResponseWriter, r *http.Request, credential string, ambiguous bool,
	next http.Handler,
) {
	var name string
	err := errPeerKeyAmbiguous
	if !ambiguous {
		name, err = e.keys.Verify(credential)
	}
	if err != nil {
		e.logRefusal(r, err)
		// RFC 6750: the key rides the Authorization header as a bearer token.
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeV1Error(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required", "")
		return
	}
	if !peerRouteAllowed(r.Method, r.URL.Path) {
		writeV1Error(w, r, http.StatusForbidden, "peer_route_refused", "Peer keys only read",
			"A peer key may read entities and their relations, "+schemaPath+" and "+openAPISpecPath+
				", and call POST "+reconcilePath+". No other route, and no sub-resource whose name starts with '_'.")
		return
	}
	ctx := principal.With(r.Context(), principal.Peer(name, toolForPath(r.URL.Path)))
	ctx = context.WithValue(ctx, peerAuthenticatedKey{}, true)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// errPeerKeyMisplaced reports a peer key that an identity source (the
// principal header or $RELA_DATAENTRY_USER) gave as a user name.
var errPeerKeyMisplaced = errors.New("dataentry: a peer key in the identity header, not in Authorization")

// errPeerKeyAmbiguous reports a peer key next to another Authorization value.
var errPeerKeyAmbiguous = errors.New("dataentry: a peer key and another Authorization value")

// logRefusal logs a refused key, sampled. The key is never logged; the error
// says only whether it was malformed, unknown or ambiguous.
func (e *peerKeyEdge) logRefusal(r *http.Request, err error) {
	suppressed, ok := e.refused.sample()
	if !ok {
		return
	}
	slog.InfoContext(r.Context(), "peer key refused",
		"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr,
		"error", err, "suppressed_count", suppressed)
}

// schemaPath is the schema route a peer may read.
const schemaPath = "/api/v1/_schema"

// peerRouteAllowed is the read-only rule for peer keys: GET and HEAD on
// `/api/v1/<plural>[/<id>[/relations[/<type>[/<target>]]]]` and the other
// entity paths without an underscore segment, the schema and the OpenAPI
// spec, and POST on the reconcile endpoint. Nothing else.
//
// A route list rather than a method rule, because a GET is not always a
// read: the co-editing WebSocket upgrades from a GET and accepts updates.
// A segment that starts with `_` is refused at any depth. That keeps peers
// off the long-lived streams (`_events`, `_views`, `_collab`), so a removed
// key stops working within the key list's reload interval, and off the
// sub-resources sync does not need: `_export` runs external commands,
// `_presence` records the caller as a viewer and lists who else is,
// `_channels` holds chat, and `_attachments`, `_history` and `_actions` are
// left for a later step that needs them. This is not concealment; it limits
// what a leaked peer key can do, whatever role acl.yaml gives the peer.
func peerRouteAllowed(method, path string) bool {
	switch method {
	case http.MethodPost:
		return path == reconcilePath
	case http.MethodGet, http.MethodHead:
	default:
		return false
	}
	if path == schemaPath || path == openAPISpecPath {
		return true
	}
	rest, ok := strings.CutPrefix(path, "/api/v1/")
	if !ok || rest == "" {
		return false
	}
	for seg := range strings.SplitSeq(rest, "/") {
		if seg == "" || strings.HasPrefix(seg, "_") {
			return false
		}
	}
	return true
}
