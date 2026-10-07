package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// tagPermissionGuard answers the namespace check of a version tag write
// (`tag:<ns>`, TKT-VO6VG9) with the subject-aware resolver, like
// [transitionGuard]. It reuses the [acl.Request] on ctx when the caller
// attached one (the data-entry server and rela-server MCP do), so its role
// walk is shared with the rest of the operation. Otherwise it opens one for
// the ctx principal, the way [acl.Declarative.AuthorizeWrite] does, because
// the CLI and scheduled scripts write with a stamped principal and no
// attached Request. An unstamped principal is denied.
//
// The check is per entity id, not per face: the acl package answers named
// permissions for an entity only, and roles conferred through relations are
// not face-scoped.
//
// d nil means no policy is configured, and the guard allows, matching the
// "no policy, nothing to enforce" tier.
type tagPermissionGuard struct{ d *acl.Declarative }

func (g tagPermissionGuard) HoldsPermission(ctx context.Context, entityID, permission string) bool {
	if g.d == nil {
		return true
	}
	r := acl.FromContext(ctx)
	if r == nil {
		var err error
		if r, err = g.d.ForPrincipal(principal.From(ctx)); err != nil {
			return false
		}
	}
	return r.HoldsPermissionForEntity(ctx, entityID, permission)
}

// buildVersionTags builds the version tag writer over s's manager and
// store. A store without version history yields a writer whose every call
// returns [entitymanager.ErrVersionTagsUnsupported].
func buildVersionTags(s *Services) (*entitymanager.VersionTags, error) {
	tagger, err := versionTaggerFor(s.store, s.meta)
	if err != nil {
		return nil, err
	}
	var st entitymanager.VersionTagStore
	if tagger != nil {
		st = tagger
	}
	return entitymanager.NewVersionTags(s.entityManager, st, tagPermissionGuard{d: s.aclDeclarative})
}

// VersionTags returns the version tag writer (TKT-VO6VG9): it authorizes
// tag writes for the ctx principal, attributes and audits them. Never nil on
// an assembled Services. A package function because Services sits at its
// plimsoll exported-method line, like [FieldGatedEntityManager].
func VersionTags(s *Services) *entitymanager.VersionTags { return s.versionTags }

// VersionTagReader returns the tag lookup a script read surface resolves
// rela.version_by_tag through, or nil when the store keeps no history.
// For hosts that build their own script readers (the data-entry server).
func VersionTagReader(s *Services) visibility.VersionTagReader {
	return versionTagReaderFor(s.store)
}

// ScriptVersionTags is [VersionTags] as the Lua field type, for hosts that
// build their own script write deps (the data-entry server). It is a
// genuinely nil interface when there is no writer, never a typed nil, so the
// runtime's nil check holds.
func ScriptVersionTags(s *Services) lua.VersionTagWriter {
	if s.versionTags == nil {
		return nil
	}
	return s.versionTags
}
