package writerule

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// Relation rules (TKT-EBARYBI).
//
// A relation rule may call `related(entity, 'rel')` or `related(entity, 'rel',
// {type = 'x'})`: one hop, along a relation or its inverse name, with no
// constraint on the far entity but its type. The answer then depends only on
// which edges of that relation the entity has. A write that does not change
// the entity or those edges cannot change it, so the write path knows exactly
// which writes to check: an update of the entity, and a create or delete of an
// edge of the relation at either end. A constraint on a far property would
// make every write of the far entity a candidate, which is why [NewSet]
// refuses it.

// Hop is one relation a relation rule reads, seen from the rule's entity:
// Incoming is set when the rule walks the relation backwards, by its inverse
// name.
type Hop struct {
	Relation string
	Incoming bool
}

// Edges answers a relation rule's `related(...)` for one entity at one
// moment: whether the entity has an edge along hop to an existing entity of
// type target. Defined here, at the consumer; the write path supplies an
// implementation over the store transaction it writes in.
type Edges interface {
	Related(ctx context.Context, hop Hop, target string) (bool, error)
}

// hopTarget is one resolved traversal of a relation rule.
type hopTarget struct {
	hop    Hop
	target string
}

// compileHops resolves the traversals of a rule's programs on typ, and
// refuses those [Set.CheckChange] cannot answer exactly. A rule without a
// traversal returns nil: it is a property rule.
func compileHops(meta *metamodel.Metamodel, typ string, progs []*predicate.Program) (map[string]hopTarget, error) {
	var hops map[string]hopTarget
	for _, prog := range progs {
		for _, spec := range prog.Traversals() {
			ht, err := compileHop(meta, typ, spec)
			if err != nil {
				return nil, fmt.Errorf("%s(...): %w", predicate.FuncRelated, err)
			}
			if hops == nil {
				hops = map[string]hopTarget{}
			}
			hops[spec.Key()] = ht
		}
	}
	if hops != nil {
		if def, ok := meta.GetEntityDef(typ); ok && len(def.Faces) > 0 {
			return nil, fmt.Errorf("%s(...) is not available on a type with `faces:`: "+
				"a traversal is answered from the entity's edges, not from one of its faces",
				predicate.FuncRelated)
		}
	}
	return hops, nil
}

func compileHop(meta *metamodel.Metamodel, typ string, spec predicate.TraversalSpec) (hopTarget, error) {
	switch {
	case len(spec.Path) != 1:
		return hopTarget{}, errors.New("a write-enforced rule follows one relation, not a path")
	case len(spec.Props) > 0 || spec.ID != nil || len(spec.Refs) > 0:
		return hopTarget{}, errors.New("a write-enforced rule may constrain only the `type` " +
			"of the related entity: a write to that entity would change the answer without being checked")
	}
	hops, err := predicatefns.ResolveTraversal(meta, typ, spec)
	if err != nil {
		return hopTarget{}, err
	}
	if metamodel.IsContentScoped(meta, hops[0].Relation) {
		return hopTarget{}, fmt.Errorf("relation %q is content-scoped; a condition can follow "+
			"identity-scoped relations only", hops[0].Relation)
	}
	return hopTarget{hop: Hop{Relation: hops[0].Relation, Incoming: hops[0].Incoming}, target: hops[0].Target}, nil
}

// ReadsRelation reports whether any relation rule reads relType, in either
// direction. A relation write that returns false needs no check.
func (s *Set) ReadsRelation(relType string) bool {
	return s != nil && s.relations[relType]
}

// HasRelationRules reports whether a relation rule applies to typ.
func (s *Set) HasRelationRules(typ string) bool {
	if s == nil {
		return false
	}
	for _, r := range s.byType[typ] {
		if r.isRelationRule() {
			return true
		}
	}
	return false
}

// ReadsHop reports whether a relation rule of typ reads hop.
func (s *Set) ReadsHop(typ string, hop Hop) bool {
	if s == nil {
		return false
	}
	for _, r := range s.byType[typ] {
		for _, h := range r.hops {
			if h.hop == hop {
				return true
			}
		}
	}
	return false
}

// CheckChange returns one error for each relation rule of after's type that
// before satisfied with the edges in was and after breaks with the edges in
// now, in declaration order. before and after are the same entity before and
// after the write; was and now answer its edges at those two moments.
//
// Only a write that turns a satisfied rule into a broken one is refused. An
// entity that already breaks a rule stays writable, and so does a sequence of
// writes that passes through a broken state: an entity created in the rule's
// state and then linked edge by edge, or a status set and its required edge
// added by two requests. The caller therefore does not run CheckChange for a
// create, which has no before.
//
// A rule that fails to evaluate breaks, as in [Set.Check]. An error from was
// or now is returned instead, because a store fault is not a verdict.
//
// Nil: before, after, was and now rejected — an error, because each is part
// of the verdict.
func (s *Set) CheckChange(
	ctx context.Context, before, after *entity.Entity, was, now Edges,
) ([]*metamodel.ValidationError, error) {
	if s == nil {
		return nil, nil
	}
	if before == nil || after == nil || was == nil || now == nil {
		return nil, errors.New("writerule.CheckChange: before, after, was and now are required")
	}
	var out []*metamodel.ValidationError
	for _, r := range s.byType[after.Type] {
		if !r.isRelationRule() {
			continue
		}
		afterHolds, err := s.holdsWith(ctx, r, after, now)
		if err != nil {
			return nil, err
		}
		if afterHolds {
			continue
		}
		beforeHolds, err := s.holdsWith(ctx, r, before, was)
		if err != nil {
			return nil, err
		}
		if beforeHolds {
			out = append(out, violation(r))
		}
	}
	return out, nil
}

// holdsWith is Set.holds for a relation rule, its traversals answered by
// edges. A rule whose face scope leaves e out holds.
func (s *Set) holdsWith(ctx context.Context, r compiledRule, e *entity.Entity, edges Edges) (bool, error) {
	if !inFaceScope(r, e) {
		return true, nil
	}
	var fault error
	traversal := func(_ predicate.Value, spec predicate.TraversalSpec) (bool, error) {
		ht, ok := r.hops[spec.Key()]
		if !ok {
			// coverage-ignore: invariant: compileHops resolved every traversal of the rule's programs
			return false, fmt.Errorf("%s(...): traversal %v was not compiled", predicate.FuncRelated, spec.Path)
		}
		got, err := edges.Related(ctx, ht.hop, ht.target)
		if err != nil && fault == nil {
			fault = err
		}
		return got, err
	}
	ok := s.holds(ctx, r, e, traversal)
	if fault != nil {
		return false, fault
	}
	return ok, nil
}
