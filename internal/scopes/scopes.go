// Package scopes compiles the metamodel's `query_scopes:` declarations into
// evaluable predicate programs — the named, reusable membership rules a
// data-entry list or kanban selects by name (TKT-EVR2TU).
//
// # Why a separate package
//
// The same boundary internal/worlds documents, for the same reason.
// Compiling needs [internal/predicatefns], which already imports
// [internal/metamodel], so compiling inside metamodel would be an import
// cycle — and arch-lint keeps metamodel a near-leaf regardless. So the
// metamodel holds the raw source and checks the NAME; this package turns
// source into a [predicate.Program] and is called during application
// assembly, making a bad expression a startup failure rather than a
// lurking runtime one.
//
// # Scopes are UX, not access control
//
// A scope narrows what a screen shows. It is never the only thing standing
// between a principal and a row: the ACL read gate runs independently, and
// first. This is why the package may not import internal/acl (enforced by
// arch-lint) — an operator who writes `mijn: "is_current_user(entity.owner)"`
// has written a display filter, and the policy must say so separately for it
// to be a restriction.
//
// # Identity
//
// Every scope compiles through [predicatefns.Evaluator.CompileWithCurrentUser],
// so `current_user` and its sugar are available. That does not make every
// scope identity-dependent: [predicatefns.RequiresCurrentUser] discriminates
// per program, and one that never mentions the identity evaluates exactly as
// a principal-free program would. See [Compiled.RequiresIdentity] for the
// case an operator needs warning about.
package scopes

import (
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// Key identifies one compiled scope: the entity type it was compiled
// against, and the name it was declared under.
//
// The type is part of the identity because a scope expression is compiled
// per type — two types may both declare `actief` over different properties,
// and they are different programs.
type Key struct {
	EntityType string
	Name       string
}

// Compiled is the result of compiling a metamodel's query scopes.
//
// The ZERO VALUE is usable and means "no declared scopes": [Compiled.Lookup]
// still answers [metamodel.AllQueryScopeName], [Compiled.Default] reports no
// default, and [Compiled.Names] is empty. That is also what Compile returns
// for a nil metamodel or one with no `query_scopes:`.
type Compiled struct {
	byKey map[Key]*predicate.Program
}

// Lookup returns the compiled program for a scope on an entity type.
//
// [metamodel.AllQueryScopeName] always resolves, to a nil program meaning
// "no predicate" — it is implicit, so a view may always withdraw the default
// even for a type that declares nothing. Any other unknown name returns
// ok=false, and callers MUST fail closed rather than substituting nil: a
// silent fallback turns a typo into "show everything", which for a type
// whose default hides archived rows is the opposite of what was asked.
//
// Nil: a nil *Compiled resolves only the implicit `all`.
func (c *Compiled) Lookup(entityType, name string) (prog *predicate.Program, ok bool) {
	if name == metamodel.AllQueryScopeName {
		return nil, true
	}
	if c == nil {
		return nil, false
	}
	prog, ok = c.byKey[Key{EntityType: entityType, Name: name}]
	return prog, ok
}

// Default returns the compiled default scope for an entity type, if the type
// declares one. A surface that names no scope uses this; ok=false means the
// type has no default and such a surface is unfiltered.
//
// Nil: a nil *Compiled has no defaults.
func (c *Compiled) Default(entityType string) (prog *predicate.Program, ok bool) {
	if c == nil {
		return nil, false
	}
	prog, ok = c.byKey[Key{EntityType: entityType, Name: metamodel.DefaultQueryScopeName}]
	return prog, ok
}

// Names returns the scope names declared for an entity type, sorted. It does
// NOT include the implicit [metamodel.AllQueryScopeName].
//
// Nil: a nil *Compiled has no names.
func (c *Compiled) Names(entityType string) []string {
	if c == nil {
		return nil
	}
	var out []string
	for key := range c.byKey {
		if key.EntityType == entityType {
			out = append(out, key.Name)
		}
	}
	sort.Strings(out)
	return out
}

// Each calls fn for every compiled scope, in a stable (type, name) order.
//
// Exists so a load-time report can inspect what every scope READS — the
// `visible:` overlap warning needs the programs themselves, not just their
// names.
//
// Ordered so that a caller emitting one line per scope gets a stable order for
// free. Today's only caller sorts its own output anyway (its lines interleave
// several scopes' findings), so the ordering here is belt-and-braces rather
// than load-bearing — but iterating a map is the kind of thing that produces a
// boot log nobody can diff, and the cost is one sort of a handful of keys.
//
// The program is handed out rather than copied because [predicate.Program] is
// immutable once compiled; callers must treat it as read-only.
//
// Nil: a nil *Compiled calls fn zero times.
func (c *Compiled) Each(fn func(key Key, prog *predicate.Program)) {
	if c == nil {
		return
	}
	keys := make([]Key, 0, len(c.byKey))
	for key := range c.byKey {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].EntityType != keys[j].EntityType {
			return keys[i].EntityType < keys[j].EntityType
		}
		return keys[i].Name < keys[j].Name
	})
	for _, key := range keys {
		fn(key, c.byKey[key])
	}
}

// RequiresIdentity reports the entity types whose DEFAULT scope reads
// current_user, sorted.
//
// This is a warning surface, not an error one. A default that reads the
// identity makes every presentation surface for that type identity-dependent,
// so on a deployment with no resolved principal every such page fails with
// [predicatefns.ErrNoCurrentUser] rather than rendering. That failure is
// correct — the honest outcomes are the right rows or an error, never
// somebody else's rows — but it is baffling without being told which
// declaration caused it, since the operator never named the scope anywhere.
//
// Only the default is reported. A NAMED identity scope is opt-in per view,
// so a view that selects it has accepted the requirement.
//
// Nil: a nil *Compiled requires nothing.
func (c *Compiled) RequiresIdentity() []string {
	if c == nil {
		return nil
	}
	var out []string
	for key, prog := range c.byKey {
		if key.Name == metamodel.DefaultQueryScopeName && predicatefns.RequiresCurrentUser(prog) {
			out = append(out, key.EntityType)
		}
	}
	sort.Strings(out)
	return out
}

// Compile turns a metamodel's `query_scopes:` declarations into compiled
// programs, one per (entity type, scope name).
//
// It reports EVERY problem it finds rather than the first, matching the
// loader's collect-then-report discipline: an operator fixing a schema should
// see the whole list. A nil metamodel compiles to an empty Compiled.
//
// Every scope compiles with current_user DECLARED (see the package doc), so
// whether a given program actually needs an identity is decided per program
// at evaluation, not per call site here.
func Compile(m *metamodel.Metamodel) (Compiled, error) {
	if m == nil {
		return Compiled{}, nil
	}
	var (
		ev    *predicatefns.Evaluator
		byKey map[Key]*predicate.Program
		errs  []string
	)
	for _, typeName := range sortedEntityTypes(m) {
		def := m.Entities[typeName]
		for _, name := range sortedScopeNames(def.QueryScopes) {
			if ev == nil {
				ev = predicatefns.NewEvaluator(m)
			}
			source := def.QueryScopes[name]
			prog, err := ev.CompileWithCurrentUser(typeName, source)
			if err != nil {
				errs = append(errs, fmt.Sprintf(
					"entity %q: query scope %q: %v (expression: %s)",
					typeName, name, err, source))
				continue
			}
			if byKey == nil {
				byKey = map[Key]*predicate.Program{}
			}
			byKey[Key{EntityType: typeName, Name: name}] = prog
		}
	}
	if len(errs) > 0 {
		return Compiled{}, &CompileError{Problems: errs}
	}
	return Compiled{byKey: byKey}, nil
}

// CompileError collects every scope that failed to compile.
type CompileError struct{ Problems []string }

func (e *CompileError) Error() string {
	if len(e.Problems) == 1 {
		return e.Problems[0]
	}
	out := fmt.Sprintf("%d query scopes failed to compile:", len(e.Problems))
	for _, p := range e.Problems {
		out += "\n  - " + p
	}
	return out
}

func sortedEntityTypes(m *metamodel.Metamodel) []string {
	out := make([]string, 0, len(m.Entities))
	for name := range m.Entities {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sortedScopeNames(scopes map[string]string) []string {
	out := make([]string, 0, len(scopes))
	for name := range scopes {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
