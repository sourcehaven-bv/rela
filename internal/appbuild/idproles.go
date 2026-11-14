package appbuild

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/idproles"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// idpRolesLockFile is the lock file next to the role document in the cache
// directory on the file tiers. Subject holds use idpRolesSubjectLocks
// striped files beside it.
const (
	idpRolesLockFile     = "idp-roles.lock"
	idpRolesSubjectLocks = 16
)

// IDPRoleWriter is what the IdP webhook receiver stores roles through
// (TKT-IZL6NX8). It binds each record to the user entity the subject resolves
// to now, so the roles never follow a principal_property value to another
// entity.
type IDPRoleWriter struct {
	roles  *idproles.Store
	policy *acl.Policy
	st     store.Store
	audit  audit.Audit
}

// IDPRoles returns the role writer of s. It is a package function rather than
// a method because [Services] is at its plimsoll cap.
//
// Nil: returned only for a Services built by [NewFromCollaborators]; every
// assembled build has a store.
func IDPRoles(s *Services) *IDPRoleWriter {
	if s.idpRoles == nil {
		return nil
	}
	return &IDPRoleWriter{roles: s.idpRoles, policy: s.aclPolicy, st: s.store, audit: s.audit}
}

// Hold takes the lock of one subject; see [idproles.Store.Hold].
func (w *IDPRoleWriter) Hold(ctx context.Context, subject string) (func(), error) {
	return w.roles.Hold(ctx, subject)
}

// Set stores roles for subject, bound to the user entity the subject resolves
// to. A subject that resolves to no single user entity has its record
// cleared instead: there is no one to bind the roles to.
func (w *IDPRoleWriter) Set(ctx context.Context, subject, orgID string, roles []string) error {
	entityID, err := w.resolve(ctx, subject)
	if err != nil {
		return err
	}
	if entityID == "" {
		if len(roles) > 0 {
			slog.Warn("idp roles: subject resolves to no single user entity; roles cleared, not stored")
		}
		roles = nil
	}
	if err := w.roles.Set(ctx, subject, orgID, entityID, roles); err != nil {
		return err
	}
	w.record(ctx, entityID, len(roles))
	return nil
}

// Forget removes every record bound to the user entity entityID, for an
// erasure request about that person (GH-2118). It audits one cleared role
// list when it removed anything, and returns how many records it removed.
func (w *IDPRoleWriter) Forget(ctx context.Context, entityID string) (int, error) {
	n, err := w.roles.ForgetEntity(ctx, entityID)
	if err != nil || n == 0 {
		return n, err
	}
	w.record(ctx, entityID, 0)
	return n, nil
}

// idpRoleEraser drops the records bound to a user entity when that entity
// leaves the graph or changes id, so a record never outlives the person it
// describes (GH-2118). A renamed entity loses its record rather than having
// it rebound: the next verified webhook binds the roles again.
//
// It rides the [entitymanager.AliasRewriter] hook for the reason
// buildRefHolders gives.
type idpRoleEraser struct{ w *IDPRoleWriter }

// newIDPRoleEraser returns the eraser over roles.
//
// Nil: returns nil when roles is nil; newAliasFanout drops it.
func newIDPRoleEraser(
	roles *idproles.Store, policy *acl.Policy, st store.Store, aud audit.Audit,
) *idpRoleEraser {
	if roles == nil {
		return nil
	}
	return &idpRoleEraser{w: &IDPRoleWriter{roles: roles, policy: policy, st: st, audit: aud}}
}

// EntityRenamed drops the records bound to oldID.
func (e *idpRoleEraser) EntityRenamed(ctx context.Context, oldID, _ string) error {
	_, err := e.w.Forget(ctx, oldID)
	return err
}

// EntityDeleted drops the records bound to entityID.
func (e *idpRoleEraser) EntityDeleted(ctx context.Context, entityID string) error {
	_, err := e.w.Forget(ctx, entityID)
	return err
}

// EntityFaceDeleted does nothing: a record is bound to the entity, which
// still exists.
func (*idpRoleEraser) EntityFaceDeleted(context.Context, string, entity.Face) error { return nil }

// record audits one stored or cleared role list. It names the bound entity
// and the role count only: the IdP subject and the role names stay out of
// the audit log.
func (w *IDPRoleWriter) record(ctx context.Context, entityID string, count int) {
	if w.audit == nil {
		return
	}
	rec := audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpIDPRolesSet,
		Principal:   principal.From(ctx),
		AssertedBy:  audit.AssertedByFrom(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     fmt.Sprintf("roles=%d", count),
	}
	if entityID != "" {
		rec.Subject = &audit.Subject{Kind: "entity", ID: entityID}
		if w.policy != nil {
			rec.Subject.Type = w.policy.UserEntityType
		}
	}
	w.audit.Record(rec)
}

// resolve returns the user entity subject names: the one entity whose
// principal_property equals it, or the user entity with that id when no
// principal_property is set. "" when there is none or more than one.
func (w *IDPRoleWriter) resolve(ctx context.Context, subject string) (string, error) {
	if w.policy == nil || strings.TrimSpace(w.policy.UserEntityType) == "" {
		return "", nil
	}
	prop := strings.TrimSpace(w.policy.PrincipalProperty)
	if prop == "" {
		headers, err := store.FamilyHeaders(ctx, w.st, subject)
		if err != nil || len(headers) == 0 || headers[0].Type != w.policy.UserEntityType {
			return "", err
		}
		return subject, nil
	}
	lookup, err := aclstore.NewPrincipalLookup(w.st)
	if err != nil {
		return "", err
	}
	ids, err := lookup.LookupEntityByProperty(ctx, w.policy.UserEntityType, prop, subject)
	if err != nil || len(ids) != 1 {
		return "", err
	}
	return ids[0], nil
}

// buildIDPRoles returns the role store over stateKV, or the predecessor's on
// a re-assembly.
//
// The lock follows the KV. On postgres the KV is the tenant's state table and
// the store's advisory lock serializes every process. On the cache-directory
// KV a lock file does, because the webhook receiver, `rela scheduler` and CLI
// scripts are separate processes over one directory. Sqlite keeps the KV in
// its database and admits one process, so the lock file there is a formality
// that costs nothing. Without a cache
// directory the document lives in process memory and is lost on restart; a
// warning says so.
func buildIDPRoles(base *SharedBase, st store.Store, stateKV state.KV) (*idproles.Store, error) {
	if base.idpRoles != nil {
		return base.idpRoles, nil
	}
	if _, nop := stateKV.(nopKV); nop || stateKV == nil {
		slog.Warn("idp roles: no cache directory; asserted roles are kept in this process only")
		return idproles.New(memoryKV(), lock.NewMemoryLocker())
	}
	if _, pg := st.(lock.KeyedLockBackend); pg {
		return idproles.New(stateKV, lock.For(st))
	}
	if inMemoryFS(base.cfg.FS) || base.cfg.Paths == nil {
		return idproles.New(stateKV, lock.NewMemoryLocker())
	}
	return idproles.New(stateKV, fileKeyLocker{dir: base.cfg.Paths.CacheDir})
}

// fileKeyLocker adapts lock files in dir to the keyed locker idproles takes.
// The document key has its own file. A subject key maps to one of a fixed
// set of striped files by hash, so a subject never reaches a file name and
// two subjects at worst wait for each other.
type fileKeyLocker struct{ dir string }

func (f fileKeyLocker) Acquire(ctx context.Context, key string) (func(), error) {
	name := idpRolesLockFile
	if key != idproles.LockKey {
		h := fnv.New32a()
		_, _ = h.Write([]byte(key))
		name = fmt.Sprintf("idp-roles-%02d.lock", h.Sum32()%idpRolesSubjectLocks)
	}
	l, err := filelock.New(filepath.Join(f.dir, name))
	if err != nil {
		return nil, err
	}
	return l.Lock(ctx)
}

// storedAssertedRoles returns the stored IdP roles of the user entity whose
// family headers are given, for a principal rela mints itself (a scheduler
// `for_each` child).
//
// The store is keyed by the IdP subject, which is the user entity's
// principal_property value, or its id when no principal_property is set. A
// value the entity shares with another user entity grants nothing: the
// webhook named one subject, and an ambiguous value cannot say which entity
// that was. Faces that disagree on the value grant nothing either.
func storedAssertedRoles(
	ctx context.Context, s *Services, entityID string, headers []store.EntityHeader,
) ([]string, error) {
	if s.idpRoles == nil || s.aclPolicy == nil || len(s.aclPolicy.AssertedRoles) == 0 {
		return nil, nil
	}
	subject, err := principalSubject(ctx, s, entityID, headers)
	if err != nil || subject == "" {
		return nil, err
	}
	return s.idpRoles.RolesFor(ctx, subject, entityID)
}

// principalSubject returns the IdP subject of a user entity, or "" when it
// has none or the value is ambiguous.
func principalSubject(
	ctx context.Context, s *Services, entityID string, headers []store.EntityHeader,
) (string, error) {
	prop := strings.TrimSpace(s.aclPolicy.PrincipalProperty)
	if prop == "" {
		return entityID, nil
	}
	var subject string
	for i, h := range headers {
		v, _ := h.Properties[prop].(string)
		if i > 0 && v != subject {
			return "", nil
		}
		subject = v
	}
	if subject == "" {
		return "", nil
	}
	lookup, err := aclstore.NewPrincipalLookup(s.store)
	if err != nil {
		return "", err
	}
	ids, err := lookup.LookupEntityByProperty(ctx, s.aclPolicy.UserEntityType, prop, subject)
	if err != nil {
		return "", err
	}
	if len(ids) != 1 || ids[0] != entityID {
		slog.Warn("idp roles: principal value is not unique; stored roles not applied", "entity", entityID)
		return "", nil
	}
	return subject, nil
}

// forEachPrincipal is the principal a scheduler `for_each` child runs as:
// the user entity id, carrying the IdP roles stored for it. The roles still
// map through asserted_role_assignments, exactly as a JWT `roles` claim does.
func forEachPrincipal(user string, roles []string) principal.Principal {
	return principal.VerifiedFrom(user, principal.ToolScheduler, principal.Claims{Roles: roles})
}

// roleHolders answers rela.principals_with_role from the stored IdP roles.
type roleHolders struct {
	roles  *idproles.Store
	policy *acl.Policy
	st     store.Store
}

// roleHoldersFor returns the Lua role-holder capability, or nil when there is
// no store or no person mapping.
func roleHoldersFor(s *Services) lua.RoleHolders {
	if s.idpRoles == nil || s.aclPolicy == nil || strings.TrimSpace(s.aclPolicy.UserEntityType) == "" {
		return nil
	}
	return roleHolders{roles: s.idpRoles, policy: s.aclPolicy, st: s.store}
}

// errUnknownRole is raised for a role acl.yaml does not declare. Role names
// are configuration, not a secret, so naming the problem is fine.
var errUnknownRole = errors.New("role is not declared in acl.yaml")

// PrincipalsWithRole returns the user entity rows whose stored IdP roles map
// to role through asserted_role_assignments. It reads the user type's headers
// once, whatever the number of holders, and leaves the read gate to the
// binding.
func (r roleHolders) PrincipalsWithRole(ctx context.Context, role string) (string, []entity.Ref, error) {
	if _, ok := r.policy.Roles[role]; !ok {
		return "", nil, fmt.Errorf("%q: %w", role, errUnknownRole)
	}
	var claims []string
	for claim, targets := range r.policy.AssertedRoles {
		if slices.Contains(targets, role) {
			claims = append(claims, claim)
		}
	}
	userType := r.policy.UserEntityType
	holders, err := r.roles.HoldersWithAny(ctx, claims)
	if err != nil || len(holders) == 0 {
		return userType, nil, err
	}
	// The subject of a user entity is its principal_property value, or its
	// id when none is set.
	prop := strings.TrimSpace(r.policy.PrincipalProperty)
	bySubject := map[string][]store.EntityHeader{}
	for h, err := range store.ListEntityHeaders(ctx, r.st, store.EntityQuery{
		Type: userType, Faces: store.AtFaces(entity.ImplicitFace),
	}) {
		if err != nil {
			return "", nil, err
		}
		subject := h.ID
		if prop != "" {
			subject, _ = h.Properties[prop].(string)
		}
		if subject != "" {
			bySubject[subject] = append(bySubject[subject], h)
		}
	}
	var refs []entity.Ref
	for _, holder := range holders {
		// The roles apply to the entity they were stored for, and only
		// while it alone holds the subject, as for a for_each child.
		if match := bySubject[holder.Subject]; len(match) == 1 && match[0].ID == holder.EntityID {
			refs = append(refs, entity.Ref{ID: match[0].ID, Face: match[0].Face})
		}
	}
	slices.SortFunc(refs, func(a, b entity.Ref) int { return strings.Compare(a.ID, b.ID) })
	return userType, refs, nil
}
