//go:build sqlite || postgres

package appbuild_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

const syncMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "T-"
    id_type: sequential
    properties:
      title: {type: string}
      points: {type: integer}
      due: {type: date}
      done: {type: boolean}
      secret: {type: string}
      basecamp:
        type: external_ref
        system: basecamp
        sync: true
  vault:
    label: Vault
    plural: vaults
    id_prefix: "V-"
    id_type: sequential
    properties:
      bc:
        type: external_ref
        system: basecamp
`

// connector reads every ticket field except secret and may tag sync/*;
// user edits tickets.
const syncPolicy = `roles:
  connector:
    read: [ticket]
    create: [ticket]
    update: [ticket]
    permissions: ["tag:sync"]
    visible:
      ticket:
        - field: title
        - field: points
        - field: due
        - field: done
        - field: basecamp
  user:
    read: [ticket]
    update: [ticket]
  blind:
    read: [ticket]
    visible:
      ticket:
        - field: title
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
assignments:
  connector: connector
  user: user
  blind: blind
  admin: admin
`

func newSyncServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, syncMetamodel)
	writePolicy(t, root, syncPolicy)
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func syncLua(ctx context.Context, t *testing.T, svc *appbuild.Services, src string) (map[string]string, error) {
	t.Helper()
	var out strings.Builder
	rt := lua.NewWriter(svc.ScheduledLuaWriteDeps(), &out, lua.WithContext(ctx),
		lua.WithPrincipal(principal.From(ctx)), lua.WithExternalRefWrites())
	defer rt.Close()
	err := rt.RunString(src) //nolint:contextcheck // the runtime carries ctx via lua.WithContext
	got := map[string]string{}
	for l := range strings.SplitSeq(strings.TrimRight(out.String(), "\n"), "\n") {
		if u, uErr := strconv.Unquote(l); uErr == nil {
			l = u
		}
		if k, v, ok := strings.Cut(l, "="); ok {
			got[k] = v
		}
	}
	return got, err
}

func asUser(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolScheduler})
}

// AC12: a database backend starts with sync refs.
func TestRequireSyncBackend_DB(t *testing.T) {
	require.NoError(t, appbuild.RequireSyncBackend(newSyncServices(t)))
}

// AC11 (TKT-SM20FG, D2/D4): the token a write returns is the token a fresh
// read returns, for decoded integer, date, boolean and map values; a stale
// expect writes nothing; another principal's write after ours makes the tag
// conflict.
func TestLuaWriteToken(t *testing.T) {
	svc := newSyncServices(t)
	connector, user := asUser("connector"), asUser("user")

	got, err := syncLua(connector, t, svc, `
local e, _, tok = rela.create_entity("ticket", {title = "a", points = 3, due = "2026-10-08", done = true,
  basecamp = {id = "42", url = "https://x.test/42"}}, nil, nil, {token = true})
rela.output("id=" .. e.id)
rela.output("create_match=" .. tostring(tok == rela.version_token(e.id)))
local u, _, tok2 = rela.update_entity(e.id, {points = 5}, nil, {expect = tok})
rela.output("update_match=" .. tostring(tok2 == rela.version_token(e.id)))
local none, why = rela.update_entity(e.id, {points = 6}, nil, {expect = tok})
rela.output("stale=" .. tostring(none) .. "," .. tostring(why))
rela.output("points=" .. rela.get_entity(e.id).properties.points)
rela.output("tagged=" .. tostring(rela.tag_version(e.id, "sync/basecamp", {expect = tok2}) ~= nil))
rela.output("tok=" .. tok2)
local _, _, untok = rela.create_entity("ticket", {title = "c"})
rela.output("create_untokened=" .. tostring(untok))
local _, _, untok2 = rela.update_entity(e.id, {title = "a2"})
rela.output("update_untokened=" .. tostring(untok2))
local _, _, asked = rela.update_entity(e.id, {title = "a"}, nil, {token = true})
rela.output("update_asked=" .. tostring(asked == rela.version_token(e.id)))
`)
	require.NoError(t, err)
	require.Equal(t, "true", got["create_match"])
	require.Equal(t, "true", got["update_match"])
	require.Equal(t, "nil,conflict", got["stale"])
	require.Equal(t, "5", got["points"])
	require.Equal(t, "true", got["tagged"])
	// Without expect or token = true a write pays for no token.
	require.Equal(t, "nil", got["create_untokened"])
	require.Equal(t, "nil", got["update_untokened"])
	require.Equal(t, "true", got["update_asked"])
	id := got["id"]

	// A hidden field changing does not move the connector's token.
	_, err = syncLua(user, t, svc, `rela.update_entity("`+id+`", {secret = "t"})`)
	require.NoError(t, err)
	got, err = syncLua(connector, t, svc, `
local _, _, tok = rela.update_entity("`+id+`", {title = "b"}, nil, {expect = "`+got["tok"]+`"})
rela.output("ok=" .. tostring(tok ~= nil))
rela.output("tok=" .. tok)
`)
	require.NoError(t, err)
	require.Equal(t, "true", got["ok"])

	// The user edits after the connector's write: the tag must not take it.
	_, err = syncLua(user, t, svc, `rela.update_entity("`+id+`", {title = "user edit"})`)
	require.NoError(t, err)
	got, err = syncLua(connector, t, svc, `
local v, why = rela.tag_version("`+id+`", "sync/basecamp", {expect = "`+got["tok"]+`"})
rela.output("tag=" .. tostring(v) .. "," .. tostring(why))
`)
	require.NoError(t, err)
	require.Equal(t, "nil,conflict", got["tag"])
}

// AC11 (D2): the write token is the token of the row as decoded, for a
// float written into an integer property (a soft warning, so it is stored)
// and for a nested map, on every database backend.
func TestLuaWriteToken_DecodedValues(t *testing.T) {
	svc := newSyncServices(t)
	got, err := syncLua(asUser("connector"), t, svc, `
local e, _, tok = rela.create_entity("ticket", {title = "f", points = 2.5, basecamp = {id = "f1", url = "https://x.test/f1"}},
  nil, nil, {token = true})
rela.output("create=" .. tostring(tok == rela.version_token(e.id)))
local _, _, tok2 = rela.update_entity(e.id, {points = 7, due = "2026-01-02", basecamp = {id = "f2"}}, "body", {expect = tok})
rela.output("update=" .. tostring(tok2 == rela.version_token(e.id)))
local _, _, tok3 = rela.update_entity(e.id, {points = 1e3}, nil, {expect = tok2})
rela.output("float=" .. tostring(tok3 == rela.version_token(e.id)))
`)
	require.NoError(t, err)
	require.Equal(t, "true", got["create"])
	require.Equal(t, "true", got["update"])
	require.Equal(t, "true", got["float"])
}

// AC7: find_by_external_ref answers through the caller's reader.
func TestLuaFindByExternalRef(t *testing.T) {
	svc := newSyncServices(t)
	connector, blind, admin := asUser("connector"), asUser("blind"), asUser("admin")
	got, err := syncLua(connector, t, svc, `
local e = rela.create_entity("ticket", {title = "a", basecamp = {id = "42"}})
rela.output("id=" .. e.id)
rela.output("found=" .. rela.find_by_external_ref("basecamp", "42").id)
rela.output("unknown=" .. tostring(rela.find_by_external_ref("basecamp", "43")))
`)
	require.NoError(t, err)
	require.Equal(t, got["id"], got["found"])
	require.Equal(t, "nil", got["unknown"])

	_, err = syncLua(admin, t, svc, `rela.create_entity("vault", {bc = {id = "77"}})`)
	require.NoError(t, err)
	got, err = syncLua(connector, t, svc, `rela.output("hidden_row=" .. tostring(rela.find_by_external_ref("basecamp", "77")))`)
	require.NoError(t, err)
	require.Equal(t, "nil", got["hidden_row"])
	got, err = syncLua(blind, t, svc, `rela.output("hidden_ref=" .. tostring(rela.find_by_external_ref("basecamp", "42")))`)
	require.NoError(t, err)
	require.Equal(t, "nil", got["hidden_ref"])

	_, err = syncLua(connector, t, svc, `rela.find_by_external_ref("jira", "42")`)
	require.ErrorContains(t, err, "no type declares")

	// The id is taken even though the connector cannot see its holder (D5).
	_, err = syncLua(connector, t, svc, `rela.create_entity("ticket", {title = "b", basecamp = {id = "77"}})`)
	require.ErrorContains(t, err, "must be unique")
}

// AC8/AC10 through Lua: the result table, EMPTY clearing a property, and
// the refusals.
func TestLuaSyncMerge(t *testing.T) {
	svc := newSyncServices(t)
	connector := asUser("connector")
	got, err := syncLua(connector, t, svc, `
local e, _, tok = rela.create_entity("ticket", {title = "a", points = 1, basecamp = {id = "1"}}, "body", nil,
  {token = true})
rela.tag_version(e.id, "sync/basecamp", {expect = tok})
rela.update_entity(e.id, {points = 2})
local base = rela.version_by_tag(e.id, "sync/basecamp")
local ours = rela.get_entity(e.id)
local r = rela.sync.merge(base, ours, {properties = {title = rela.sync.EMPTY, points = 1}, content = "body\r\n"},
  {fields = {"title", "points", "content"}})
rela.output("base_unknown=" .. tostring(r.base_unknown))
rela.output("write_title_empty=" .. tostring(r.write.title == rela.sync.EMPTY))
rela.output("push_points=" .. tostring(r.push.points))
rela.output("unchanged=" .. table.concat(r.unchanged, ","))
rela.output("conflicts=" .. #r.conflicts)
local w = rela.update_entity(e.id, r.write, r.content)
rela.output("title_after=" .. tostring(w.properties.title))
local ok, msg = pcall(rela.sync.merge, nil, ours, {properties = {}}, {fields = {"nope"}})
rela.output("undeclared=" .. tostring(ok))
ok, msg = pcall(rela.sync.merge, nil, ours, {properties = {points = "lots"}}, {fields = {"points"}})
rela.output("uncoercible=" .. tostring(ok))
`)
	require.NoError(t, err)
	require.Equal(t, "false", got["base_unknown"])
	require.Equal(t, "true", got["write_title_empty"])
	require.Equal(t, "2", got["push_points"])
	require.Equal(t, "content", got["unchanged"])
	require.Equal(t, "0", got["conflicts"])
	require.Equal(t, "nil", got["title_after"])
	require.Equal(t, "false", got["undeclared"])
	require.Equal(t, "false", got["uncoercible"])

	// A synced field the reader cannot see raises.
	_, err = syncLua(asUser("blind"), t, svc, `
local e = rela.list_entities("ticket")[1]
rela.sync.merge(nil, e, {properties = {points = 1}}, {fields = {"points"}})`)
	require.ErrorContains(t, err, "hidden")
}
