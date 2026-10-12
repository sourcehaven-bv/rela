package dataentry

import (
	"context"
	"net/http"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/presence"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Presence over live views (TKT-MHXH87). A plain GET of
// /api/v1/{plural}/{id}/_presence lists who else has the entity open and
// never joins. Only a live subscription to that path joins: each replay that
// answers 200 records the face it resolved in a probe, and the live hub
// joins the subscriber to it (see [liveHub.applyPresenceLocked]).
//
// Presence is keyed by the resolved face, so two readers of different faces
// of one entity do not see each other.

// presenceViewerCap is how many viewers a presence response lists.
const presenceViewerCap = 20

// presenceKey is the tracker key of e: its resolved face.
func presenceKey(e *entityPkg.Entity) string { return e.Ref().String() }

// presenceIdentity is the tracker identity of p: the fields
// [principal.Principal.SameIdentity] compares.
func presenceIdentity(p principal.Principal) presence.Identity {
	return presence.Identity{User: p.User, Tool: p.Tool, RawUser: p.RawUser, OrgID: p.OrgID()}
}

// handlePresence serves GET /api/v1/{plural}/{id}/_presence. Every miss is
// the entity GET's uniform 404: it reads the entity through the same call.
//
// The caller is left out on user, raw user and organization, whatever the
// tool, so your own desktop app never shows up as someone else.
func handlePresence(
	w http.ResponseWriter, r *http.Request, vr visibleReader, tr *presence.Tracker, typeName, addr string,
) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	e, found := readAddressedOr404(w, r, vr, typeName, addr)
	if !found {
		return
	}
	recordPresence(r.Context(), e)
	self := presenceIdentity(principal.From(r.Context()))
	resp := v1.PresenceResponse{Viewers: []v1.PresenceViewer{}}
	for _, v := range tr.Viewers(presenceKey(e)) {
		if v.User == self.User && v.RawUser == self.RawUser && v.OrgID == self.OrgID {
			continue
		}
		if len(resp.Viewers) == presenceViewerCap {
			resp.More++
			continue
		}
		resp.Viewers = append(resp.Viewers, v1.PresenceViewer{
			User: v.User, Tool: v.Tool, Since: v.Since.UTC().Truncate(time.Second).Format(time.RFC3339),
		})
	}
	writeV1JSON(w, http.StatusOK, resp)
}

// presenceProbe is where a replay of the presence route reports the face it
// resolved and who resolved it. [replayHash] allocates one per call; the
// hub honors it only for a presence subscription whose replay answered 200
// and whose path names the same entity id (M4).
type presenceProbe struct {
	set bool
	id  string // the entity id, compared with the subscription path's
	key string // presenceKey of the entity
	who presence.Identity
}

type presenceProbeKey struct{}

func withPresenceProbe(ctx context.Context) (context.Context, *presenceProbe) {
	p := &presenceProbe{}
	return context.WithValue(ctx, presenceProbeKey{}, p), p
}

// recordPresence fills the request's probe, when it has one, from e. Pass
// only the entity [readAddressedOr404] returned.
func recordPresence(ctx context.Context, e *entityPkg.Entity) {
	p, _ := ctx.Value(presenceProbeKey{}).(*presenceProbe)
	if p == nil {
		return
	}
	*p = presenceProbe{
		set: true, id: e.ID, key: presenceKey(e), who: presenceIdentity(principal.From(ctx)),
	}
}
