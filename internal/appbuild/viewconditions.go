package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/conditionlint"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ViewConditionMatcher restates the method set internal/dataentry
// declares, WITHOUT importing it: dataentry's own tests import appbuild, so
// naming its types here would close an import cycle. NextActionMatchers
// avoids the same cycle by returning nextaction.Matcher — a type from a third
// package both sides see — but view conditions have no such package, so the
// structural type is restated. Go's structural interfaces make the two
// identical at the call site; a drift would be a compile error at the
// SetViewConditions call in cmd/, which is where they meet.
type ViewConditionMatcher interface {
	Matches(ctx context.Context, e *entity.Entity) (bool, error)
}

// ViewConditions compiles the `condition:` of every list and kanban, adapting
// conditionlint's compiler to the seam internal/dataentry declares.
//
// Lives at the composition root for the same reason [NextActionMatchers]
// does: the condition engine sits above the data-entry app, so dataentry
// takes a seam and this bridges the two.
//
// Takes config + metamodel rather than returning a prebuilt lookup because
// both reload at runtime — a lookup captured at boot would keep evaluating a
// condition the operator has since edited.
func ViewConditions(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (lookup func(kind, id string) (ViewConditionMatcher, bool), problems []string) {
	compiled, problems := conditionlint.ViewConditionMatchers(cfg, meta)
	if len(problems) > 0 || compiled == nil {
		return nil, problems
	}
	return func(kind, id string) (ViewConditionMatcher, bool) {
		m, ok := compiled(kind, id)
		if !ok {
			// Returning m directly would hand back a non-nil interface
			// wrapping a nil *ViewConditionMatcher — the typed-nil trap
			// NextActionMatchers guards against for the same reason.
			return nil, false
		}
		return m, true
	}, nil
}
