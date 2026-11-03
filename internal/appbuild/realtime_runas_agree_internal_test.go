//go:build !postgres && !memorybackend && !sqlite

package appbuild

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

const agreeSchema = `version: "1.0"
types:
  severity:
    values: [low, high]
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
      status: {type: string}
      severity: {type: severity}
      phase: {type: string, values: [open, shut]}
      secret: {type: string}
  page:
    label: Page
    id_prefix: "PAGE-"
    id_type: sequential
    faces:
      draft: {}
      published: {}
    properties:
      title: {type: string}
      kind: {type: string, values: [a, b]}
  person:
    label: Person
    id_prefix: "P-"
    id_type: sequential
    properties:
      title: {type: string}
relations:
  links:
    from: [doc]
    to: [doc]
  refers:
    from: [page]
    to: [doc]
  owns:
    from: [doc]
    to: [doc]
  holds:
    from: [person]
    to: [doc]
`

// agreePolicies are the policies the grant check is compared on. Each
// gives system:bot its grants a different way; "owner" is a role only a
// relation or an asserted claim confers, which a job identity never holds.
var agreePolicies = []struct{ name, policy string }{
	{"plain", `roles:
  bot:
    read: [doc, "page@draft"]
    create: [doc, "page@draft"]
    update: [doc, "page@draft"]
assignments:
  system:bot: bot
`},
	{"fields and options", `roles:
  bot:
    read: [doc]
    create: [doc]
    update: [doc]
    fields:
      doc:
        - field: status
        - field: severity
    options:
      doc:
        - field: severity
          option: low
assignments:
  system:bot: bot
`},
	{"everyone only", `roles:
  bot:
    read: [doc]
  everyone:
    read: [doc]
    update: [doc]
assignments:
  system:bot: bot
`},
	{"local role only", localRolePolicy},
	{"asserted only", assertedPolicy},
	{"union of a blocked and a blockless role", `roles:
  bot:
    read: [doc]
    update: [doc]
    fields:
      doc:
        - field: title
    options:
      doc:
        - field: severity
          option: low
  everyone:
    read: [doc]
    update: [doc]
assignments:
  system:bot: bot
`},
	{"face-keyed blocks and inline enums", `roles:
  bot:
    read: [doc, page]
    create: [doc, "page@draft", "page@published"]
    update: [doc, "page@draft", "page@published"]
    fields:
      "page@draft":
        - field: kind
    visible:
      page:
        - field: kind
    options:
      "page@draft":
        - field: kind
          option: a
      doc:
        - field: phase
          option: shut
assignments:
  system:bot: bot
`},
	{"visible block", `roles:
  bot:
    read: [doc]
    update: [doc]
    visible:
      doc:
        - field: title
        - field: status
assignments:
  system:bot: bot
`},
	{"relation grants", `roles:
  bot:
    read: [doc, page]
    create: [doc]
    update: [doc]
    permissions: [links.write]
assignments:
  system:bot: bot
relation_grants:
  links:
    create: links.write
role_relations:
  owns:
    confers: bot
    requires_permission: owners.write
`},
}

// localRolePolicy confers owner, which may update docs, along a holds edge
// from a person. A job identity has no user entity, so no edge reaches it.
const localRolePolicy = `roles:
  bot:
    read: [doc]
  owner:
    read: [doc]
    update: [doc]
assignments:
  system:bot: bot
role_relations:
  holds:
    confers: owner
`

// assertedPolicy maps the admin claim of a verified assertion to owner. A
// job identity carries no assertion.
const assertedPolicy = `roles:
  bot:
    read: [doc]
  owner:
    read: [doc]
    update: [doc]
assignments:
  system:bot: bot
asserted_role_assignments:
  admin: owner
`

// TestRunAsGrantCheck_AgreesWithRequest compares, for every policy above,
// what [acl.JobIdentityGrants] answers at load with what a request for the
// same job identity decides at the write: the verb with
// [acl.Request.AuthorizeWrite], the fields and options with the
// affordance resolver the field gate uses, and a relation with
// [entitymanager.CheckRelate]. None of these policies has a when:, so the
// answers must agree exactly.
func TestRunAsGrantCheck_AgreesWithRequest(t *testing.T) {
	for _, pc := range agreePolicies {
		t.Run(pc.name, func(t *testing.T) {
			env := runAsServices(t, map[string]string{
				"schema.yaml": agreeSchema, "acl.yaml": pc.policy, "realtime.yaml": "",
			})
			svc := env.svc
			grants := acl.JobIdentityGrants(svc.aclDeclarative.Policy(), "system:bot")
			ctx, err := acl.BindRequest(principal.With(t.Context(),
				principal.Principal{User: "system:bot", Tool: principal.ToolRealtime}), svc.aclDeclarative)
			require.NoError(t, err)
			req, err := svc.aclDeclarative.RequestFrom(ctx)
			require.NoError(t, err)
			res, err := affordances.New(svc.meta, storeRelationLookup{st: svc.store}, svc.aclDeclarative)
			require.NoError(t, err)
			for _, typ := range []string{"doc", "page"} {
				def, ok := svc.meta.GetEntityDef(typ)
				require.True(t, ok)
				for _, face := range agreeFaces(svc.meta, typ) {
					for _, op := range []acl.Op{acl.OpCreate, acl.OpUpdate} {
						where := fmt.Sprintf("%s %s@%s", op, typ, face)
						d := req.AuthorizeWrite(ctx, acl.WriteRequest{
							Op: op, Subject: acl.NewEntitySubject(typ, "X-1", face),
						})
						require.Equal(t, d.Allow, grants.Verb(op, typ, face), where)
						if !d.Allow {
							continue
						}
						e := &entity.Entity{ID: "X-1", Type: typ, Face: face}
						fv := res.FieldVerdicts(ctx, e)
						if op == acl.OpCreate {
							fv = res.CreateFieldVerdicts(ctx, e)
						}
						for name, pd := range def.Properties {
							// The verdict maps are sparse: only a denial is listed.
							writable := allowed(fv.Writable, name) && allowed(fv.Visible, name)
							require.Equal(t, writable, grants.FieldWrite(op, typ, face, name), where+" field "+name)
							for _, v := range agreeEnum(svc.meta, pd) {
								require.Equal(t, allowed(fv.Options[name], v), grants.Option(op, typ, face, name, v),
									where+" option "+name+"="+v)
							}
						}
					}
				}
			}
			for _, rel := range []string{"links", "refers", "owns"} {
				rd, ok := svc.meta.GetRelationDef(rel)
				require.True(t, ok)
				for _, from := range rd.From {
					for _, face := range agreeFaces(svc.meta, from) {
						tail := face
						if rd.Scope.IsIdentity() {
							tail = ""
						}
						var family []entity.Face
						if tail.IsImplicit() {
							family = agreeFaces(svc.meta, from)
						}
						want := entitymanager.CheckRelate(ctx, svc.entityManager, rel, from, face) == nil
						require.Equal(t, want, grants.Relates(rel, from, tail, family),
							"relate %s from %s@%s", rel, from, face)
					}
				}
			}
		})
	}
}

// agreeEnum returns the values of an enum property, inline or through a
// custom type.
func agreeEnum(meta *metamodel.Metamodel, pd metamodel.PropertyDef) []string {
	if len(pd.Values) > 0 {
		return pd.Values
	}
	return meta.Types[pd.Type].Values
}

// allowed reads a sparse verdict map: a key is allowed unless listed false.
func allowed(m map[string]bool, key string) bool {
	v, ok := m[key]
	return !ok || v
}

// agreeFaces returns the faces typ declares, or the zero face alone.
func agreeFaces(meta *metamodel.Metamodel, typ string) []entity.Face {
	names := metamodel.FaceOrderOf(meta, typ)
	if len(names) == 0 {
		return []entity.Face{""}
	}
	out := make([]entity.Face, len(names))
	for i, n := range names {
		out[i] = entity.Face(n)
	}
	return out
}

// TestRunAsGrantCheck_PersonOnlyRoles pins that the local-role and
// asserted rows above are not vacuous: the same policy grants a person
// the update through a real edge or a real claim, while both the request
// and the grant check deny the job identity.
func TestRunAsGrantCheck_PersonOnlyRoles(t *testing.T) {
	update := func(t *testing.T, svc *Services, p principal.Principal) bool {
		t.Helper()
		ctx, err := acl.BindRequest(principal.With(t.Context(), p), svc.aclDeclarative)
		require.NoError(t, err)
		req, err := svc.aclDeclarative.RequestFrom(ctx)
		require.NoError(t, err)
		return req.AuthorizeWrite(ctx, acl.WriteRequest{
			Op: acl.OpUpdate, Subject: acl.NewEntitySubject("doc", "DOC-1", ""),
		}).Allow
	}
	bot := principal.Principal{User: "system:bot", Tool: principal.ToolRealtime}

	t.Run("local role", func(t *testing.T) {
		env := runAsServices(t, map[string]string{
			"schema.yaml": agreeSchema, "acl.yaml": localRolePolicy, "realtime.yaml": "",
		})
		svc := env.svc
		ctx := context.Background()
		for _, e := range []*entity.Entity{{ID: "DOC-1", Type: "doc"}, {ID: "P-1", Type: "person"}} {
			require.NoError(t, svc.store.CreateEntity(ctx, e))
		}
		_, err := svc.store.CreateRelation(ctx, entity.RelationKey{From: "P-1", Type: "holds", To: "DOC-1"}, nil)
		require.NoError(t, err)
		require.True(t, update(t, svc, principal.Principal{User: "P-1", Tool: principal.ToolDataEntry}),
			"the edge grants the person")
		require.False(t, update(t, svc, bot))
		require.False(t, acl.JobIdentityGrants(svc.aclDeclarative.Policy(), "system:bot").Verb(acl.OpUpdate, "doc", ""))
	})

	t.Run("asserted claim", func(t *testing.T) {
		env := runAsServices(t, map[string]string{
			"schema.yaml": agreeSchema, "acl.yaml": assertedPolicy, "realtime.yaml": "",
		})
		svc := env.svc
		require.NoError(t, svc.store.CreateEntity(context.Background(), &entity.Entity{ID: "DOC-1", Type: "doc"}))
		alice := principal.Verified("alice", principal.ToolDataEntry, "org", "org", []string{"admin"})
		require.True(t, update(t, svc, alice), "the claim grants the person")
		require.False(t, update(t, svc, bot))
		require.False(t, acl.JobIdentityGrants(svc.aclDeclarative.Policy(), "system:bot").Verb(acl.OpUpdate, "doc", ""))
	})
}
