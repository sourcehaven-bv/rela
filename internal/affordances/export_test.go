package affordances

import "github.com/Sourcehaven-BV/rela/internal/acl"

// DeclarativeOf exposes the evaluator a Resolver was built with, so external
// tests can bind a request identity against the same Declarative.
func DeclarativeOf(r *PolicyResolver) *acl.Declarative { return r.declarative }
