//go:build sqlite || postgres

package appbuild_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

const baselineMetamodel = `version: "1.0"
entities:
  maatregel:
    label: Maatregel
    plural: maatregelen
    id_prefix: "M-"
    id_type: sequential
    properties:
      title: {type: string}
  memo:
    label: Memo
    plural: memos
    id_prefix: "MEMO-"
    id_type: sequential
    properties:
      title: {type: string}
`

// baselinePolicy: alice does everything; the auditor reads everything and
// deleted history; the reader reads maatregelen only and holds no
// history:read.
const baselinePolicy = `roles:
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    permissions: ["tag:baseline", "history:read"]
  auditor:
    read: ["*"]
    permissions: ["history:read"]
  reader:
    read: [maatregel]
assignments:
  alice: admin
  dave: auditor
  erin: reader
`

// TestLuaTaggedEntities is TKT-7LRUPB end to end on the database builds: a
// script tags a baseline, one tagged entity is deleted, and each principal
// lists what the history rules let it see.
func TestLuaTaggedEntities(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, baselineMetamodel)
	writePolicy(t, root, baselinePolicy)
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	alice := boundAs(t, svc, principal.Principal{User: "alice", Tool: principal.ToolScheduler})
	dave := boundAs(t, svc, principal.Principal{User: "dave", Tool: principal.ToolScheduler})
	erin := boundAs(t, svc, principal.Principal{User: "erin", Tool: principal.ToolScheduler})
	create := func(typ, title string) string {
		e := entity.New("", typ)
		e.SetString("title", title)
		res, cErr := svc.EntityManager().CreateEntity(alice, e, entity.CreateOptions{})
		require.NoError(t, cErr)
		return res.Entity.ID
	}
	kept := create("maatregel", "kept")
	gone := create("maatregel", "gone")
	memo := create("memo", "private")

	_, err = runLua(alice, t, svc, `
for _, id in ipairs({"`+kept+`", "`+gone+`", "`+memo+`"}) do
  rela.tag_version(id, "baseline/audit-2026")
end`)
	require.NoError(t, err)
	_, err = svc.EntityManager().DeleteEntity(alice, gone, false)
	require.NoError(t, err)

	list := func(t *testing.T, ctx context.Context, opts string) []string {
		t.Helper()
		got, lErr := runLua(ctx, t, svc, `
for _, r in ipairs(rela.tagged_entities("baseline/audit-2026", `+opts+`)) do
  rela.output(r.id .. "|" .. r.type .. "|" .. r.version .. "|" .. tostring(r.deleted))
end`)
		require.NoError(t, lErr)
		lines := outputLines(got)
		if len(lines) == 1 && lines[0] == "" {
			return nil
		}
		slices.Sort(lines)
		return lines
	}
	row := func(id, typ string, deleted bool) string {
		if deleted {
			return id + "|" + typ + "|1|true"
		}
		return id + "|" + typ + "|1|false"
	}
	sorted := func(rows ...string) []string { slices.Sort(rows); return rows }

	t.Run("auditor sees live and deleted", func(t *testing.T) {
		require.Equal(t, sorted(row(kept, "maatregel", false), row(gone, "maatregel", true), row(memo, "memo", false)),
			list(t, dave, "nil"))
	})
	t.Run("type filter", func(t *testing.T) {
		require.Equal(t, sorted(row(kept, "maatregel", false), row(gone, "maatregel", true)),
			list(t, dave, `{type = "maatregel"}`))
	})
	t.Run("without history:read deleted rows and unreadable types are left out", func(t *testing.T) {
		require.Equal(t, []string{row(kept, "maatregel", false)}, list(t, erin, "nil"))
	})
	t.Run("unknown tag lists nothing", func(t *testing.T) {
		got, lErr := runLua(dave, t, svc, `rela.output("n=" .. #rela.tagged_entities("baseline/none"))`)
		require.NoError(t, lErr)
		require.Contains(t, outputLines(got), "n=0")
	})
}

// outputLines splits script output into its lines, so an assertion matches a
// whole line rather than a prefix of a longer one ("v=1" in "v=12").
// rela.output writes each value as a quoted string.
func outputLines(got string) []string {
	var out []string
	for l := range strings.SplitSeq(strings.TrimRight(got, "\n"), "\n") {
		if u, err := strconv.Unquote(l); err == nil {
			l = u
		}
		out = append(out, l)
	}
	return out
}

func runLua(ctx context.Context, t *testing.T, svc *appbuild.Services, src string) (string, error) {
	t.Helper()
	var out strings.Builder
	rt := lua.NewWriter(svc.ScheduledLuaWriteDeps(), &out, lua.WithContext(ctx), lua.WithPrincipal(principal.From(ctx)))
	defer rt.Close()
	err := rt.RunString(src) //nolint:contextcheck // the runtime carries ctx via lua.WithContext
	return out.String(), err
}
