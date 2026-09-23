package appbuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// traversalAnswers holds, per traversal (keyed by [predicate.TraversalSpec.Key]),
// the candidate ids that satisfy it. Built once per Filter call and dropped
// with it: the answers depend on the principal, so they must never outlive
// the request that computed them.
type traversalAnswers map[string]map[string]bool

// answerTraversals answers every `related(...)` in prog for the candidate
// ids, with one gated store query per distinct traversal.
//
// A traversal the principal may not read at all ([acl.ErrTraversalDenied])
// matches no candidate: a hidden entity is a nonexistent one. Every other
// gate refusal is returned as an error, because "no match" would widen a
// negated traversal.
func answerTraversals(
	ctx context.Context, meta *metamodel.Metamodel, prog *predicate.Program, entityType string, ids []string,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
	match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
) (traversalAnswers, error) {
	specs := prog.Traversals()
	if len(specs) == 0 {
		return traversalAnswers{}, nil
	}
	if gate == nil || match == nil {
		return nil, errors.New("appbuild: query scope traversal: no gate or store to answer it")
	}
	answers := make(traversalAnswers, len(specs))
	for _, spec := range specs {
		key := spec.Key()
		if _, done := answers[key]; done {
			continue
		}
		hop, err := traversalHop(meta, entityType, spec)
		if err != nil {
			return nil, err
		}
		pred, err := gate(ctx, entityType, hop)
		if errors.Is(err, acl.ErrTraversalDenied) {
			answers[key] = map[string]bool{}
			continue
		}
		if err != nil {
			return nil, err
		}
		got, err := match(ctx, acl.TraversalQuery(entityType, hop, pred), ids)
		if err != nil {
			return nil, err
		}
		answers[key] = got
	}
	return answers, nil
}

// traversalHop lowers a compiled traversal into the ungated hop chain the
// ACL gate authorizes. Resolution goes through [predicatefns.ResolveTraversal],
// the same walk load-time validation and index derivation use, so the three
// cannot disagree about where a chain lands.
//
// The author's property filters constrain the FINAL hop only; intermediate
// hops constrain type and relation.
func traversalHop(
	meta *metamodel.Metamodel, entityType string, spec predicate.TraversalSpec,
) (acl.TraversalHop, error) {
	hops, err := predicatefns.ResolveTraversal(meta, entityType, spec)
	if err != nil {
		return acl.TraversalHop{}, err
	}
	if len(hops) == 0 {
		// coverage-ignore: invariant: the engine refuses an empty relation path at compile
		return acl.TraversalHop{}, errors.New("related: empty relation path")
	}
	props := make([]store.PropPredicate, 0, len(spec.Props))
	for _, name := range spec.PropNames() {
		str, ok := spec.Props[name].(predicate.String)
		if !ok {
			// Load-time validation refuses this; guard rather than lower a
			// comparison the store would read differently.
			return acl.TraversalHop{}, fmt.Errorf("related: property %q must be compared against a string", name)
		}
		props = append(props, store.PropPredicate{
			Property: name, Op: store.PropEqual, Value: str.String(), Scalar: true,
		})
	}

	var next *acl.TraversalHop
	for i := len(hops) - 1; i >= 0; i-- {
		h := &acl.TraversalHop{
			RelationTypes: []string{hops[i].Relation},
			Incoming:      hops[i].Incoming,
			EntityType:    hops[i].Target,
			Next:          next,
		}
		if i == len(hops)-1 {
			h.Props = props
		}
		next = h
	}
	return *next, nil
}

// traversalFunc answers a traversal for ONE row from precomputed answers.
//
// It refuses rather than guesses: a subject other than the row itself, or a
// traversal nobody answered, is an error. Returning false would present an
// unanswered question as a legitimate "no match".
func (a traversalAnswers) traversalFunc(rowID string) predicate.TraversalFunc {
	return func(subject predicate.Value, spec predicate.TraversalSpec) (bool, error) {
		rec, ok := subject.(predicate.Record)
		if !ok {
			return false, errors.New("traversal subject is not a record")
		}
		if id, _ := rec.Get("id"); id != predicate.NewString(rowID) {
			return false, errors.New("traversal subject is not the row being evaluated")
		}
		ans, ok := a[spec.Key()]
		if !ok {
			return false, fmt.Errorf("traversal %v was not answered", spec.Path)
		}
		return ans[rowID], nil
	}
}
