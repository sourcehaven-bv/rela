package appbuild

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/idproles"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// idpRolesFixture seeds people and the IdP roles the webhook stored for
// their subjects: PER-1 holds the cpo claim alone, PER-2 and PER-3 share a
// principal value, PER-4 has none, PER-5 has no stored roles, and PER-6 now
// holds a value whose roles were stored for PER-1.
func idpRolesFixture(t *testing.T, principalProperty string) *Services {
	t.Helper()
	ctx := t.Context()
	st := memstore.New()
	people := map[string]string{"PER-1": "a@x", "PER-2": "dup@x", "PER-3": "dup@x", "PER-4": "", "PER-5": "b@x",
		"PER-6": "moved@x"}
	for id, email := range people {
		props := map[string]any{}
		if email != "" {
			props["email"] = email
		}
		require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "person", Properties: props}))
	}
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "TKT-1", Type: "ticket"}))
	roles, err := idproles.New(memoryKV(), lock.NewMemoryLocker())
	require.NoError(t, err)
	for _, rec := range []struct{ subject, entity, claim string }{
		{"a@x", "PER-1", "idp-cpo"}, {"dup@x", "PER-2", "idp-cpo"}, {"moved@x", "PER-1", "idp-cpo"},
		{"PER-1", "PER-1", "idp-cpo"}, {"PER-5", "PER-5", "idp-other"},
	} {
		require.NoError(t, roles.Set(ctx, rec.subject, "org_1", rec.entity, []string{rec.claim}))
	}
	return &Services{store: st, idpRoles: roles, audit: audit.NewMemory(), aclPolicy: &acl.Policy{
		UserEntityType:    "person",
		PrincipalProperty: principalProperty,
		Roles:             map[string]acl.RoleDef{"cpo": {Read: []string{"*"}}, "other": {}},
		AssertedRoles:     map[string]acl.RoleList{"idp-cpo": {"cpo"}, "idp-other": {"other"}},
	}}
}

// TestScheduledForEachPrincipal_StoredRoles pins TKT-IZL6NX8: a for_each
// child runs as the user entity with the IdP roles stored for its subject,
// and those roles map to rela roles through asserted_role_assignments.
func TestScheduledForEachPrincipal_StoredRoles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		property  string
		entity    string
		wantUser  string
		wantRoles []string
		wantACL   bool
	}{
		{name: "roles keyed by the principal value", property: "email", entity: "PER-1",
			wantUser: "PER-1", wantRoles: []string{"idp-cpo"}, wantACL: true},
		{name: "a shared principal value grants nothing", property: "email", entity: "PER-2", wantUser: "PER-2"},
		{name: "no principal value grants nothing", property: "email", entity: "PER-4", wantUser: "PER-4"},
		{name: "no stored roles", property: "email", entity: "PER-5", wantUser: "PER-5"},
		{name: "roles do not follow a moved principal value", property: "email", entity: "PER-6", wantUser: "PER-6"},
		{name: "without principal_property the id is the subject", entity: "PER-1",
			wantUser: "PER-1", wantRoles: []string{"idp-cpo"}, wantACL: true},
		{name: "a non-user entity has no principal", property: "email", entity: "TKT-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := idpRolesFixture(t, tc.property)
			got, err := svc.ScheduledForEachPrincipal(t.Context(), tc.entity)
			require.NoError(t, err)
			require.Equal(t, tc.wantUser, got.User)
			require.Equal(t, tc.wantRoles, got.Roles())
			if tc.wantUser == "" {
				return
			}
			require.Equal(t, principal.ToolScheduler, got.Tool)

			d, err := acl.NewDeclarative(svc.aclPolicy, acl.NullGraph{}, acl.NullGraphQueryer{},
				acl.WithPrincipalLookup(aclstore.MustNewPrincipalLookup(svc.store)))
			require.NoError(t, err)
			req, err := acl.OpenRequest(t.Context(), d, got)
			require.NoError(t, err)
			holds := slices.ContainsFunc(req.Globals().Attributions, func(a acl.RoleAttribution) bool {
				return a.Role == "cpo"
			})
			require.Equal(t, tc.wantACL, holds, "holds the mapped cpo role")
		})
	}
}

func TestScheduledForEachPrincipal_NoRoleStore(t *testing.T) {
	t.Parallel()
	svc := idpRolesFixture(t, "email")
	svc.idpRoles = nil
	got, err := svc.ScheduledForEachPrincipal(t.Context(), "PER-1")
	require.NoError(t, err)
	require.Equal(t, "PER-1", got.User)
	require.Empty(t, got.Roles())
}

func TestRoleHolders_PrincipalsWithRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		property string
		role     string
		want     []string
		wantErr  string
	}{
		{name: "holders by principal value; a shared value names no one", property: "email", role: "cpo",
			want: []string{"PER-1"}},
		{name: "holders by id", role: "cpo", want: []string{"PER-1"}},
		{name: "other role", role: "other", want: []string{"PER-5"}},
		{name: "role with no holder", property: "email", role: "other"},
		{name: "undeclared role", property: "email", role: "admin", wantErr: "not declared"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			holders := roleHoldersFor(idpRolesFixture(t, tc.property))
			require.NotNil(t, holders)
			typ, ids, err := holders.PrincipalsWithRole(t.Context(), tc.role)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "person", typ)
			var got []string
			for _, ref := range ids {
				got = append(got, ref.ID)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRoleHoldersFor_Unavailable(t *testing.T) {
	t.Parallel()
	svc := idpRolesFixture(t, "email")
	svc.idpRoles = nil
	require.Nil(t, roleHoldersFor(svc))
	svc = idpRolesFixture(t, "email")
	svc.aclPolicy.UserEntityType = ""
	require.Nil(t, roleHoldersFor(svc))
}

// TestIDPRoleWriter_BindsTheResolvedEntity pins the store-time binding: roles
// are stored for the one user entity the subject resolves to, and a subject
// that resolves to none or several clears its record instead.
func TestIDPRoleWriter_BindsTheResolvedEntity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		property   string
		subject    string
		wantEntity string
	}{
		{name: "by principal value", property: "email", subject: "b@x", wantEntity: "PER-5"},
		{name: "a shared value clears", property: "email", subject: "dup@x"},
		{name: "an unknown value clears", property: "email", subject: "nobody@x"},
		{name: "by id", subject: "PER-4", wantEntity: "PER-4"},
		{name: "an id of another type clears", subject: "TKT-1"},
		{name: "an unknown id clears", subject: "NOPE-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := idpRolesFixture(t, tc.property)
			w := IDPRoles(svc)
			require.NotNil(t, w)
			release, err := w.Hold(t.Context(), tc.subject)
			require.NoError(t, err)
			defer release()
			require.NoError(t, w.Set(t.Context(), tc.subject, "org_1", []string{"idp-cpo"}))
			holders, err := svc.idpRoles.HoldersWithAny(t.Context(), []string{"idp-cpo"})
			require.NoError(t, err)
			var got string
			for _, h := range holders {
				if h.Subject == tc.subject {
					got = h.EntityID
				}
			}
			require.Equal(t, tc.wantEntity, got)

			// One audit record per write, naming the bound entity and the
			// role count but never the IdP subject.
			recs := svc.audit.(*audit.Memory).Records()
			require.Len(t, recs, 1)
			require.Equal(t, audit.OpIDPRolesSet, recs[0].Op)
			require.NotContains(t, recs[0].Summary, tc.subject)
			if tc.wantEntity == "" {
				require.Nil(t, recs[0].Subject)
				require.Equal(t, "roles=0", recs[0].Summary)
				return
			}
			require.Equal(t, &audit.Subject{Kind: "entity", Type: "person", ID: tc.wantEntity}, recs[0].Subject)
			require.Equal(t, "roles=1", recs[0].Summary)
		})
	}
}

func TestIDPRoles_NilWithoutStore(t *testing.T) {
	t.Parallel()
	require.Nil(t, IDPRoles(&Services{}))
}

// TestFileKeyLocker keeps the document lock and the subject holds on separate
// files, so Set inside a Hold cannot wait on itself.
func TestFileKeyLocker(t *testing.T) {
	t.Parallel()
	l := fileKeyLocker{dir: t.TempDir()}
	ctx := t.Context()
	hold, err := l.Acquire(ctx, idproles.SubjectLockPrefix+"usr_1")
	require.NoError(t, err)
	doc, err := l.Acquire(ctx, idproles.LockKey)
	require.NoError(t, err, "the document lock is free while a subject is held")
	doc()
	hold()
	again, err := l.Acquire(ctx, idproles.SubjectLockPrefix+"usr_1")
	require.NoError(t, err, "a released subject can be held again")
	again()
}
