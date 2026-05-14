package workspace

import (
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// ValidationError is an alias of [entitymanager.ValidationError]. It
// exists so existing callers (e.g. internal/dataentry/handlers_api.go)
// can keep using `*workspace.ValidationError` while internal/workspace
// is being decomposed.
//
// Deprecated: use [entitymanager.ValidationError] directly. The
// workspace package is being removed (TKT-64R3).
type ValidationError = entitymanager.ValidationError

// ErrHasRelations is an alias for [entitymanager.ErrHasRelations] so
// existing callers (`cli/delete.go`, `workspace_test.go`) can use
// `errors.Is(err, workspace.ErrHasRelations)` unchanged.
//
// Deprecated: use [entitymanager.ErrHasRelations] directly. The
// workspace package is being removed (TKT-64R3).
var ErrHasRelations = entitymanager.ErrHasRelations
