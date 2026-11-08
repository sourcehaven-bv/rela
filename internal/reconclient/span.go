package reconclient

import (
	"fmt"
	"slices"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// span is a set of types and an address range [lo, hi), unbounded above
// when bounded is false. An entity span names the entity types whose rows
// it covers. A relation span names relation types; its entities are the
// endpoint scope, which is the same for every relation span of a run, so a
// split never moves a row out of every child.
type span struct {
	kind      reconcile.Kind
	entities  []string
	relations []string
	lo, hi    string
	bounded   bool
	// local holds the local elements of the span in order once localKnown.
	local      []reconcile.Element
	localKnown bool
	// notRun counts the deadline refusals of this span.
	notRun int
}

// types returns the span's types as the local reader takes them.
func (s *span) types() visibility.ElementTypes {
	return visibility.ElementTypes{Entities: s.entities, Relations: s.relations}
}

// storeRange returns [lo, hi) of the span as the local reader takes it.
func (s *span) storeRange(lo string) store.ElementRange {
	return store.ElementRange{
		Relations: s.kind == reconcile.KindRelation, Lo: lo, Hi: s.hi, Unbounded: !s.bounded,
	}
}

// names is the number of type names the span sends.
func (s *span) names() int { return len(s.entities) + len(s.relations) }

// multiType reports whether the span can be split by type.
func (s *span) multiType() bool {
	if s.kind == reconcile.KindRelation {
		return len(s.relations) > 1
	}
	return len(s.entities) > 1
}

// wire returns the span as one range of a request.
func (s *span) wire(op string, limit int, after *string) v1.ReconcileRange {
	rr := v1.ReconcileRange{Kind: "entity", Entities: s.entities, Op: op, Limit: limit}
	if s.kind == reconcile.KindRelation {
		rr.Kind, rr.Relations = "relation", s.relations
	}
	if after != nil {
		a := *after
		rr.After = &a
	} else {
		rr.Lo = s.lo
	}
	if s.bounded {
		hi := s.hi
		rr.Hi = &hi
	}
	return rr
}

// child returns a span over the same range with other types and nothing
// known locally.
func (s *span) child(entities, relations []string) *span {
	return &span{
		kind: s.kind, entities: entities, relations: relations, lo: s.lo, hi: s.hi, bounded: s.bounded,
	}
}

// byType splits a multi-type span into one span per type.
func (s *span) byType() []*span {
	var out []*span
	if s.kind == reconcile.KindRelation {
		for _, r := range s.relations {
			out = append(out, s.child(s.entities, []string{r}))
		}
		return out
	}
	for _, e := range s.entities {
		out = append(out, s.child([]string{e}, nil))
	}
	return out
}

// byAddress splits a span whose local elements are known into about n
// buckets at local element boundaries, and returns nil when the span has
// fewer than two local elements. Each bucket knows its local elements. An
// address too long to be a range bound is not used as a boundary.
func (s *span) byAddress(n int) []*span {
	els := s.local
	n = min(n, len(els))
	if n < 2 {
		return nil
	}
	cuts := []int{0}
	for i := 1; i < n; i++ {
		c := i * len(els) / n
		if c > cuts[len(cuts)-1] && len(els[c].Address) <= store.MaxRangeBound {
			cuts = append(cuts, c)
		}
	}
	if len(cuts) < 2 {
		return nil
	}
	out := make([]*span, 0, len(cuts))
	for i, c := range cuts {
		b := &span{
			kind: s.kind, entities: s.entities, relations: s.relations,
			lo: s.lo, hi: s.hi, bounded: s.bounded, localKnown: true,
		}
		if i > 0 {
			b.lo = els[c].Address
		}
		end := len(els)
		if i+1 < len(cuts) {
			end = cuts[i+1]
			b.hi, b.bounded = els[end].Address, true
		}
		b.local = els[c:end]
		out = append(out, b)
	}
	return out
}

// localSum folds the known local elements.
func (s *span) localSum() reconcile.Range {
	var r reconcile.Range
	for _, e := range s.local {
		r = r.Add(e)
	}
	return r
}

// checkAddress checks an address the remote listed for this span: its
// length, its characters, its type, and that it lies above floor (strictly
// when strict) and below hi.
func (s *span) checkAddress(a, floor string, strict bool) error {
	switch {
	case a == "":
		return fmt.Errorf("%w: empty address", ErrProtocol)
	case len(a) > store.MaxRangeBound:
		return fmt.Errorf("%w: address of %d bytes", ErrProtocol, len(a))
	case !Printable(a):
		return fmt.Errorf("%w: address %q holds a character that is not printable", ErrProtocol, Sanitize(a))
	case strict && a <= floor, !strict && a < floor:
		return fmt.Errorf("%w: address %q is out of order or below the range", ErrProtocol, a)
	case s.bounded && a >= s.hi:
		return fmt.Errorf("%w: address %q is above the range", ErrProtocol, a)
	}
	if !s.ownsAddress(a) {
		return fmt.Errorf("%w: address %q is not of a type the range names", ErrProtocol, a)
	}
	return nil
}

// ownsAddress reports whether a is an address of one of the span's types:
// "type/id[@face]" for an entity, "from[@face]--type--to" for a relation.
// Neither ids nor faces hold "--" (the relation key format depends on it),
// so a relation address has exactly three parts.
func (s *span) ownsAddress(a string) bool {
	if s.kind == reconcile.KindRelation {
		parts := strings.Split(a, "--")
		return len(parts) == 3 && parts[0] != "" && parts[2] != "" && slices.Contains(s.relations, parts[1])
	}
	typ, id, ok := strings.Cut(a, "/")
	return ok && id != "" && slices.Contains(s.entities, typ)
}

// chunk splits names into slices of at most n.
func chunk(names []string, n int) [][]string {
	var out [][]string
	for len(names) > n {
		out = append(out, names[:n:n])
		names = names[n:]
	}
	if len(names) > 0 {
		out = append(out, names)
	}
	return out
}
