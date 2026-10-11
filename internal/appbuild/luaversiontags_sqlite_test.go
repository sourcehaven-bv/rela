//go:build sqlite

package appbuild_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const tagMetamodel = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
  note:
    label: Note
    plural: notes
    id_prefix: "NOTE-"
    id_type: sequential
    properties:
      title: {type: string}
      secret: {type: string}
  memo:
    label: Memo
    plural: memos
    id_prefix: "MEMO-"
    id_type: sequential
    properties:
      title: {type: string}
`

// tagPolicy: alice may do anything and tag sync/*; bob may edit docs but
// not tag sync/*, and cannot read memos at all; carol may edit notes but
// sees only their title.
const tagPolicy = `roles:
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    permissions: ["tag:sync"]
  editor:
    read: [doc]
    update: [doc]
  partial:
    read: [note]
    update: [note]
    visible:
      note:
        - field: title
assignments:
  alice: admin
  bob: editor
  carol: partial
`

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

// TestSQLiteLuaVersionTags is TKT-VO6VG9 end to end on the sqlite build:
// a scheduled script tags, reads by tag, untags, and is refused where the
// policy says so, with a hidden entity failing exactly like a missing one.
func TestSQLiteLuaVersionTags(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, tagMetamodel)
	writePolicy(t, root, tagPolicy)
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	alice := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolScheduler})
	bob := principal.With(context.Background(), principal.Principal{User: "bob", Tool: principal.ToolScheduler})
	carol := principal.With(context.Background(), principal.Principal{User: "carol", Tool: principal.ToolScheduler})
	create := func(typ, title string) string {
		e := entity.New("", typ)
		e.SetString("title", title)
		res, cErr := svc.EntityManager().CreateEntity(alice, e, entity.CreateOptions{})
		require.NoError(t, cErr)
		return res.Entity.ID
	}
	doc := create("doc", "first")
	memo := create("memo", "private")

	t.Run("tag, read, conflict, untag", func(t *testing.T) {
		got, err := runLua(alice, t, svc, `
local tok = rela.version_token("`+doc+`")
local v = rela.tag_version("`+doc+`", "sync/jira", {expect = tok})
rela.output("tagged=" .. v)
rela.output("by_tag=" .. rela.version_by_tag("`+doc+`", "sync/jira").properties.title)
rela.update_entity("`+doc+`", {title = "second"})
local none, why = rela.tag_version("`+doc+`", "sync/jira", {expect = tok})
rela.output("conflict=" .. tostring(none) .. "," .. why)
rela.output("still=" .. rela.version_by_tag("`+doc+`", "sync/jira").properties.title)
local h = rela.history("`+doc+`")
rela.output("tags=" .. table.concat(h[v].tags, ","))
rela.output("untag=" .. tostring(rela.untag_version("`+doc+`", "sync/jira")))
rela.output("again=" .. tostring(rela.untag_version("`+doc+`", "sync/jira")))
rela.output("gone=" .. tostring(rela.version_by_tag("`+doc+`", "sync/jira")))
`)
		require.NoError(t, err)
		lines := outputLines(got)
		for _, want := range []string{
			"tagged=1", "by_tag=first", "conflict=nil,conflict", "still=first",
			"tags=sync/jira", "untag=true", "again=false", "gone=nil",
		} {
			require.Contains(t, lines, want)
		}
	})

	t.Run("namespace needs its permission", func(t *testing.T) {
		_, err := runLua(bob, t, svc, `rela.tag_version("`+doc+`", "sync/jira")`)
		require.ErrorContains(t, err, "tag:sync")
		got, err := runLua(bob, t, svc, `rela.output("v=" .. rela.tag_version("`+doc+`", "reviewed", {version = 1}))`)
		require.NoError(t, err)
		require.Contains(t, outputLines(got), "v=1")
	})

	t.Run("hidden is missing", func(t *testing.T) {
		_, hiddenErr := runLua(bob, t, svc, `rela.tag_version("`+memo+`", "reviewed")`)
		_, missingErr := runLua(bob, t, svc, `rela.tag_version("MEMO-404", "reviewed")`)
		require.Error(t, hiddenErr)
		require.Error(t, missingErr)
		require.Equal(t,
			strings.ReplaceAll(missingErr.Error(), "MEMO-404", "X"),
			strings.ReplaceAll(hiddenErr.Error(), memo, "X"))

		got, err := runLua(bob, t, svc, `
rela.output("by_tag=" .. tostring(rela.version_by_tag("`+memo+`", "reviewed")))
rela.output("token=" .. tostring(rela.version_token("`+memo+`")))
`)
		require.NoError(t, err)
		require.Contains(t, outputLines(got), "by_tag=nil")
		require.Contains(t, outputLines(got), "token=nil")
	})

	t.Run("go writer", func(t *testing.T) {
		tags := appbuild.VersionTags(svc)
		require.NotNil(t, tags)
		name, err := store.ParseVersionTagName("cli")
		require.NoError(t, err)
		meta, err := tags.TagVersion(alice, entity.Ref{ID: doc}, name, 1)
		require.NoError(t, err)
		require.Equal(t, 1, meta.Version)
		unstamped := context.WithoutCancel(principal.With(alice, principal.Principal{}))
		_, err = tags.TagCurrent(unstamped, entity.Ref{ID: doc}, name, "", nil)
		require.ErrorContains(t, err, "principal", "an unstamped ctx must be refused")
	})

	t.Run("a redacted reader tags with its own token", func(t *testing.T) {
		e := entity.New("", "note")
		e.SetString("title", "n")
		e.SetString("secret", "s3cret")
		res, err := svc.EntityManager().CreateEntity(alice, e, entity.CreateOptions{})
		require.NoError(t, err)
		note := res.Entity.ID
		got, err := runLua(carol, t, svc, `
local e = rela.get_entity("`+note+`")
rela.output("sees_secret=" .. tostring(e.properties.secret ~= nil))
local tok = rela.version_token("`+note+`")
local v, why = rela.tag_version("`+note+`", "seen", {expect = tok})
rela.output("tagged=" .. tostring(v ~= nil) .. "," .. tostring(why))
`)
		require.NoError(t, err)
		lines := outputLines(got)
		require.Contains(t, lines, "sees_secret=false")
		require.Contains(t, lines, "tagged=true,nil")
	})
}
