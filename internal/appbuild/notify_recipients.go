package appbuild

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// maxNotifyFanout bounds the entities one notify path may reach. A path
// that fans out further is cut, with an error, rather than notifying a
// whole graph.
const maxNotifyFanout = 1000

// notifyRecipients answers autocascade.RecipientFinder from the raw store:
// one relation query per hop over the whole frontier, then one header query
// for the entities reached.
type notifyRecipients struct{ st store.Store }

func (n notifyRecipients) Recipients(
	ctx context.Context, from entity.Ref, hops []automation.NotifyHop, property string,
) ([]string, error) {
	ids := []string{from.ID}
	for _, h := range hops {
		q := store.RelationQuery{EntityIDs: ids, Type: h.Relation, Direction: store.DirectionOutgoing}
		if h.Incoming {
			q.Direction = store.DirectionIncoming
		}
		var next []string
		seen := map[string]bool{}
		for rel, err := range n.st.ListRelations(ctx, q) {
			if err != nil {
				return nil, fmt.Errorf("notify: following %q: %w", h.Relation, err)
			}
			id := rel.To
			if h.Incoming {
				id = rel.From
			}
			if !seen[id] {
				seen[id] = true
				next = append(next, id)
			}
			if len(next) > maxNotifyFanout {
				return nil, fmt.Errorf("notify: %q reaches more than %d entities", h.Relation, maxNotifyFanout)
			}
		}
		if len(next) == 0 {
			return nil, nil
		}
		ids = next
	}
	var owners []string
	seen := map[string]bool{}
	q := store.EntityQuery{IDs: ids, Type: holderType(hops), Faces: store.AllFaces()}
	headers := store.ListEntityHeaders(ctx, n.st, q)
	for h, err := range headers {
		if err != nil {
			return nil, fmt.Errorf("notify: reading recipients: %w", err)
		}
		for _, o := range automation.PropertyOwners(h.Properties[property]) {
			if !seen[o] {
				seen[o] = true
				owners = append(owners, o)
			}
		}
	}
	return owners, nil
}

// holderType is the type the last hop lands on: a relation that may reach
// several types reaches recipients only on the one the path resolved to.
func holderType(hops []automation.NotifyHop) string {
	if len(hops) == 0 {
		return ""
	}
	return hops[len(hops)-1].Target
}
