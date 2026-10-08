//go:build sqlite

package appbuild_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestSQLiteLuaReadsHistory is TKT-EC7F65 end to end: versions the sweep
// captured on the sqlite build reach a scheduled script through
// rela.history and rela.get_version.
func TestSQLiteLuaReadsHistory(t *testing.T) {
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "20ms")
	t.Setenv("RELA_VERSION_SWEEP_IDLE", "1ms")

	svc, err := discover(t, writeMinimalProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolScheduler})
	e := entity.New("", "doc")
	e.SetString("title", "first")
	created, err := svc.EntityManager().CreateEntity(ctx, e, entity.CreateOptions{})
	require.NoError(t, err)
	id := created.Entity.ID
	waitForVersions(t, svc.Versions().ListVersions, id, 1)

	_, err = svc.EntityManager().PatchEntity(ctx, id, entity.Patch{
		Properties: map[string]any{"title": "second"},
	})
	require.NoError(t, err)
	waitForVersions(t, svc.Versions().ListVersions, id, 2)

	var out strings.Builder
	rt := lua.NewWriter(svc.ScheduledLuaWriteDeps(), &out, lua.WithContext(ctx), lua.WithPrincipal(principal.From(ctx)))
	defer rt.Close()
	require.NoError(t, rt.RunString(`
local h = rela.history("`+id+`")
rela.output("versions=" .. #h .. " first=" .. h[1].op .. " user=" .. h[1].user)
rela.output("v1=" .. rela.get_version("`+id+`", 1).properties.title)
rela.output("v2=" .. rela.get_version("`+id+`", 2).properties.title)
`))
	got := out.String()
	for _, want := range []string{"versions=2 first=create user=alice", "v1=first", "v2=second"} {
		require.Contains(t, got, want)
	}
}

// historyPolicyMetamodel and historyPolicy make doc.secret visible to bob
// only while the doc has a `blocks` edge, a condition the live graph answers
// and a snapshot cannot.
const historyPolicyMetamodel = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
      secret: {type: string}
relations:
  blocks:
    from: [doc]
    to: [doc]
`

const historyPolicy = `roles:
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
  viewer:
    read: [doc]
    visible:
      doc:
        - field: title
        - field: secret
          when: "has_relation(entity, 'blocks')"
assignments:
  alice: admin
  bob: viewer
`

// TestSQLiteLuaHistoryRedactsUnderPolicy runs the history bindings through
// the gated reader appbuild wires when acl.yaml exists. A field whose grant
// depends on a relation is visible on the live entity and hidden in its
// snapshot, because the live graph cannot say what the edges were then.
func TestSQLiteLuaHistoryRedactsUnderPolicy(t *testing.T) {
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "20ms")
	t.Setenv("RELA_VERSION_SWEEP_IDLE", "1ms")

	root := t.TempDir()
	writeMetamodelBody(t, root, historyPolicyMetamodel)
	writePolicy(t, root, historyPolicy)
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	alice := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolScheduler})
	var ids []string
	for _, title := range []string{"blocked", "blocker"} {
		e := entity.New("", "doc")
		e.SetString("title", title)
		e.SetString("secret", "s-"+title)
		created, createErr := svc.EntityManager().CreateEntity(alice, e, entity.CreateOptions{})
		require.NoError(t, createErr)
		ids = append(ids, created.Entity.ID)
	}
	_, err = svc.EntityManager().CreateRelation(alice,
		entity.RelationKey{From: ids[0], Type: "blocks", To: ids[1]}, entity.RelationOptions{})
	require.NoError(t, err)
	waitForVersions(t, svc.Versions().ListVersions, ids[0], 1)

	bob := principal.With(context.Background(), principal.Principal{User: "bob", Tool: principal.ToolScheduler})
	var out strings.Builder
	rt := lua.NewWriter(svc.ScheduledLuaWriteDeps(), &out, lua.WithContext(bob), lua.WithPrincipal(principal.From(bob)))
	defer rt.Close()
	require.NoError(t, rt.RunString(`
rela.output("live=" .. tostring(rela.get_entity("`+ids[0]+`").properties.secret))
local v = rela.get_version("`+ids[0]+`", 1)
rela.output("title=" .. tostring(v.properties.title))
rela.output("historical=" .. tostring(v.properties.secret))
`))
	got := out.String()
	for _, want := range []string{"live=s-blocked", "title=blocked", "historical=nil"} {
		require.Contains(t, got, want)
	}
}

func waitForVersions[M any](
	t *testing.T, list func(context.Context, entity.Ref) ([]M, error), id string, n int,
) {
	t.Helper()
	require.Eventually(t, func() bool {
		metas, err := list(context.Background(), entity.Ref{ID: id})
		return err == nil && len(metas) == n
	}, 5*time.Second, 20*time.Millisecond, "the sweep never captured version %d of %s", n, id)
}
