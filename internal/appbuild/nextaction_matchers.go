package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/conditionlint"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
)

// NextActionMatchers compiles the `condition:` of every next-action source,
// adapting conditionlint's compiler to the seam internal/dataentry declares.
//
// Lives at the composition root for the same reason the userstate backend
// does: the condition/policy engine sits above the data-entry app, so
// dataentry takes a seam and this bridges the two.
//
// Takes config + metamodel rather than returning a prebuilt lookup because
// both reload at runtime — a lookup captured at boot would keep evaluating a
// condition the operator has since edited.
func NextActionMatchers(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (lookup func(string) (nextaction.Matcher, bool), problems []string) {
	compiled, issues := conditionlint.NextActionMatchers(cfg, meta)
	if len(issues) > 0 || compiled == nil {
		return nil, issues
	}
	return func(id string) (nextaction.Matcher, bool) {
		m, ok := compiled(id)
		if !ok {
			// Returning m directly would hand back a non-nil interface
			// wrapping a nil *NextActionMatcher — the classic typed-nil trap.
			return nil, false
		}
		return m, true
	}, nil
}
