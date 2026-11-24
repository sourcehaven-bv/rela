package conditionlint

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// compileAssignExpr compiles an `assign:` value expression (TKT-FGE5G3I) for
// one property. It takes no entity: the value is the same for every item, so
// `entity` and `related(...)` are unavailable.
func compileAssignExpr(meta *metamodel.Metamodel, def *metamodel.PropertyDef, expr string) (*predicate.Program, error) {
	typ, ok := predicatefns.ScalarTypeForProp(meta, def)
	if !ok {
		return nil, fmt.Errorf("property type %q is not supported by expressions", def.Type)
	}
	env := predicate.NewEnv()
	if err := predicatefns.Declare(env); err != nil {
		return nil, err
	}
	prog, err := predicate.CompileValue(env, expr, predicate.ValueProfile(typ))
	if err != nil {
		return nil, err
	}
	if len(prog.Traversals()) > 0 {
		return nil, fmt.Errorf("%s(...) is not available in an assign expression", predicate.FuncRelated)
	}
	return prog, nil
}

// assignExprProblems compiles every expression of every list and page
// `assign:`, so one that does not compile fails the load.
func assignExprProblems(cfg *dataentryconfig.Config, meta *metamodel.Metamodel) []string {
	var problems []string
	check := func(at, itemType string, a *dataentryconfig.Assign) {
		if a == nil {
			return
		}
		def, ok := meta.GetEntityDef(itemType)
		if !ok {
			return // reported by dataentryconfig
		}
		for _, prop := range slices.Sorted(maps.Keys(a.Properties)) {
			v := a.Properties[prop]
			pd, ok := def.Properties[prop]
			if v.Expr == "" || !ok {
				continue
			}
			if _, err := compileAssignExpr(meta, &pd, v.Expr); err != nil {
				problems = append(problems, fmt.Sprintf("%s: property %q: expr does not compile: %v", at, prop, err))
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Lists)) {
		l := cfg.Lists[id]
		check(fmt.Sprintf("list %q: assign", id), l.EntityType, l.Assign)
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Pages)) {
		p := cfg.Pages[id]
		if p.Assign != nil {
			check(fmt.Sprintf("pages[%s]: assign", id), p.Assign.EntityType, p.Assign)
		}
	}
	return problems
}

// EvalAssignExpr evaluates an `assign:` value expression with the clock now,
// the one list conditions use, so an assigned today() and a list's today()
// agree. The result is in the property's wire form; a date is formatted in
// the property's date format, and nil means unset.
func EvalAssignExpr(
	ctx context.Context, meta *metamodel.Metamodel, def *metamodel.PropertyDef, expr string, now time.Time,
) (any, error) {
	prog, err := compileAssignExpr(meta, def, expr)
	if err != nil {
		return nil, err
	}
	b := predicate.NewBindings()
	if bindErr := predicatefns.Bind(b, now); bindErr != nil {
		return nil, bindErr
	}
	v, err := prog.Eval(ctx, b)
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case predicate.Nil:
		return nil, nil //nolint:nilnil // nil is the value "unset", not a missing result
	case predicate.String:
		return x.String(), nil
	case predicate.Int:
		return x.Int64(), nil
	case predicate.Bool:
		return x.Bool(), nil
	case predicate.Date:
		return x.Time().Format(def.GetDateFormat()), nil
	case predicate.Decimal:
		text, ok := x.Text()
		if !ok {
			return nil, errors.New("decimal result has no finite form")
		}
		return text, nil
	default:
		return nil, fmt.Errorf("unsupported result %T", v)
	}
}
