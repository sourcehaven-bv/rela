package apptest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"maps"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Sourcehaven-BV/rela/internal/apptest"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// project copies testdata/project to a temporary root and writes files
// (relative path to content) on top. It writes a .rela/mail.yaml unless
// files has one: .rela/ is not committed.
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join("testdata", "project")
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dst := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o600)
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "relations"), 0o755))
	if _, set := files[".rela/mail.yaml"]; !set {
		// Port 9 is discard; nothing must connect to it anyway.
		files = maps.Clone(files)
		if files == nil {
			files = map[string]string{}
		}
		files[".rela/mail.yaml"] = "transport: smtp\nhost: 127.0.0.1\nport: 9\nfrom: rela@example.com\n" +
			"recipients:\n  also_allow: [ok@example.com]\n"
	}
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	return root
}

// run runs the tests under root with seed 1 unless opts names another.
func run(t *testing.T, root string, opts apptest.Options) *apptest.Report {
	t.Helper()
	opts.Root = root
	if !opts.SeedSet {
		opts.Seed, opts.SeedSet = 1, true
	}
	rep, err := apptest.Run(context.Background(), opts)
	require.NoError(t, err)
	return rep
}

// requireOK fails the test with every failing case's message.
func requireOK(t *testing.T, rep *apptest.Report) {
	t.Helper()
	for _, c := range rep.Cases {
		assert.True(t, c.OK, "%s: %s: %s: %s", c.File, c.Case, c.Location, c.Message)
	}
	require.True(t, rep.OK())
	require.NotZero(t, rep.Passed)
}

// runLua writes one test file and runs it.
func runLua(t *testing.T, code string) *apptest.Report {
	t.Helper()
	return run(t, project(t, map[string]string{"tests/a.test.lua": code}), apptest.Options{})
}

func TestAutomationEffectsInSameCase(t *testing.T) { // AC1
	rep := runLua(t, `
local t = require("rela.test")
t.case("automations", function()
  local carol = t.as("carol")
  local e = carol:create("ticket", {title = "a"})
  t.expect(e.properties.stamp).not_to_be_nil()
  local u = carol:transition(e, "ready")
  t.expect(u.properties.note).to_be("made ready")
  t.expect(carol:get(e).properties.note).to_be("made ready")
  carol:update(e, {note = "ok@example.com"})
  carol:transition(e, "done")
  t.expect_mail{to = "ok@example.com", subject = "done " .. e.id, contains = "finished"}
end)
`)
	requireOK(t, rep)
}

func TestAccessPositiveControlAndDenied(t *testing.T) { // AC2
	rep := runLua(t, `
local t = require("rela.test")
t.case("access", function()
  local ed = t.principal{name = "ed", roles = {"editor"}}
  local vi = t.principal{name = "vi", roles = {"viewer"}}
  local e = ed:create("ticket", {title = "x"})
  ed:update(e, {title = "by editor"})
  t.expect_denied(function() vi:update(e, {title = "by viewer"}) end)
  t.expect(ed:get(e).properties.title).to_be("by editor")
  t.expect_denied(function() vi:create("ticket", {title = "nope"}) end)
  t.expect(#ed:list("ticket")).to_be(1)
  t.expect_access("vi", {types = {ticket = {"read"}, user = {"read"}}})
end)
t.case("several roles through groups", function()
  local both = t.principal{name = "both", roles = {"viewer", "editor"}}
  local e = both:create("ticket", {title = "x"})
  t.expect(both:get(e)).not_to_be_nil()
end)
t.case("real policy roles", function()
  local carol = t.as("carol")
  carol:create("ticket", {title = "x"})
  t.expect_denied(function() t.as("nobody"):create("ticket", {title = "x"}) end)
end)
`)
	requireOK(t, rep)
}

// A role that cannot read the target gets a not-found from the write path,
// as in production; t.expect_denied counts it, because the target exists.
// An error caught inside fn does not count for the one fn ends with.
func TestDeniedHiddenTarget(t *testing.T) {
	rep := runLua(t, `
local t = require("rela.test")
t.case("hidden", function()
  local out = t.principal{name = "olga", roles = {"outsider"}}
  local e = t.as("carol"):create("ticket", {title = "secret"})
  t.expect_denied(function() out:update(e, {title = "x"}) end)
  t.expect_denied(function() out:delete(e) end)
  t.expect(out:get(e)).to_be_nil()
end)
t.case("missing is not denied", function()
  local out = t.principal{name = "olga", roles = {"outsider"}}
  t.expect_denied(function() out:update("TKT-404", {title = "x"}) end)
end)
t.case("stale denial", function()
  local v = t.principal{name = "vic", roles = {"viewer"}}
  t.expect_denied(function()
    pcall(function() v:create("ticket", {title = "x"}) end)
    error("something else")
  end)
end)
`)
	require.Len(t, rep.Cases, 3)
	assert.True(t, rep.Cases[0].OK, rep.Cases[0].Message)
	assert.False(t, rep.Cases[1].OK)
	assert.Contains(t, rep.Cases[1].Message, "failed otherwise")
	assert.False(t, rep.Cases[2].OK)
	assert.Contains(t, rep.Cases[2].Message, "something else")
}

// require finds helpers in tests/ as well as lib/; lib/ wins a name both
// hold.
func TestRequireFromTests(t *testing.T) {
	root := project(t, map[string]string{
		"lib/shared.lua":    "return {from = 'lib'}\n",
		"tests/shared.lua":  "return {from = 'tests'}\n",
		"tests/helpers.lua": "return {answer = 42}\n",
		"tests/a.test.lua": `
local t = require("rela.test")
local helpers = require("helpers")
local shared = require("shared")
t.case("helpers", function()
  t.expect(helpers.answer).to_be(42)
  t.expect(shared.from).to_be("lib")
  t.expect_error(function() require("nothing") end, "looked in lib/, tests/")
end)
`,
	})
	requireOK(t, run(t, root, apptest.Options{}))
}

// A misspelled option or type fails the case, so it cannot make an assertion
// pass by accident.
func TestTyposFail(t *testing.T) {
	rep := runLua(t, `
local t = require("rela.test")
t.case("principal", function() t.principal{name = "z", role = "editor"} end)
t.case("mail", function() t.expect_mail{subject_contains = "x"} end)
t.case("create", function() t.as("carol"):create("ticket", {}, {ids = "x"}) end)
t.case("list", function() t.as("carol"):list("tickte") end)
`)
	require.Len(t, rep.Cases, 4)
	for i, want := range []string{"unknown option role", "unknown option subject_contains",
		"unknown option ids", `declares no entity type "tickte"`} {
		assert.False(t, rep.Cases[i].OK)
		assert.Contains(t, rep.Cases[i].Message, want)
	}
}

func TestValidationFailureIsNotADenial(t *testing.T) {
	rep := runLua(t, `
local t = require("rela.test")
t.case("validation", function()
  local ed = t.principal{name = "ed", roles = {"editor"}}
  t.expect_denied(function() ed:create("ticket", {title = "x", start = "2999-01-01"}) end)
end)
`)
	require.Equal(t, 1, rep.Failed)
	assert.Contains(t, rep.Cases[0].Message, "failed otherwise")
	assert.Equal(t, "tests/a.test.lua:5", rep.Cases[0].Location)
}

func TestPrincipalErrors(t *testing.T) {
	rep := runLua(t, `
local t = require("rela.test")
t.case("unknown role", function() t.principal{name = "x", roles = {"admin"}} end)
t.case("after build", function()
  t.as("carol"):list("ticket")
  t.principal{name = "x", roles = {"viewer"}}
end)
`)
	require.Equal(t, 2, rep.Failed)
	assert.Contains(t, rep.Cases[0].Message, `declares no role "admin"`)
	assert.Contains(t, rep.Cases[1].Message, "must come before")
}

// A generated group id that acl.yaml already assigns is refused, not
// overwritten: the case would otherwise run against a different policy.
func TestPrincipalGroupCollision(t *testing.T) {
	acl, err := os.ReadFile(filepath.Join("testdata", "project", "acl.yaml"))
	require.NoError(t, err)
	amended := strings.Replace(string(acl), "  carol: editor\n", "  carol: editor\n  x-group-1: viewer\n", 1)
	require.NotEqual(t, string(acl), amended)
	root := project(t, map[string]string{
		"acl.yaml": amended,
		"tests/a.test.lua": `
local t = require("rela.test")
t.case("collides", function()
  t.principal{name = "x", roles = {"viewer", "editor"}}:list("ticket")
end)
`,
	})
	rep := run(t, root, apptest.Options{})
	require.Equal(t, 1, rep.Failed)
	assert.Contains(t, rep.Cases[0].Message, "group x-group-1 of x is already assigned role viewer")
}

// With principal_property, a principal name is a natural key such as an
// email. The write path and t.expect_access must agree on its access, and a
// name the id grammar refuses must still get group entities.
func TestPrincipalNaturalKey(t *testing.T) {
	acl, err := os.ReadFile(filepath.Join("testdata", "project", "acl.yaml"))
	require.NoError(t, err)
	root := project(t, map[string]string{
		"acl.yaml": "principal_property: email\n" + string(acl),
		"tests/a.test.lua": `
local t = require("rela.test")
t.case("one role", function()
  local ed = t.principal{name = "ed@example.com", roles = {"editor"}}
  ed:create("ticket", {title = "by ed"})
  t.expect_access("ed@example.com", {
    types = {ticket = {"read", "create", "update"}, user = {"read"}, kinds = {"read", "create", "update"}},
    relations = {blocks = {create = {"ticket"}, update = {"ticket"}}},
  })
end)
t.case("two roles", function()
  local ed = t.principal{name = "ed@example.com", roles = {"viewer", "editor"}}
  t.expect(ed:create("ticket", {title = "by ed"}).properties.title).to_be("by ed")
end)
`,
	})
	requireOK(t, run(t, root, apptest.Options{}))

	rep := runLua(t, `
local t = require("rela.test")
t.case("bad id", function() t.principal{name = "ed@example.com", roles = {"viewer"}} end)
`)
	require.Equal(t, 1, rep.Failed)
	assert.Contains(t, rep.Cases[0].Message, "set principal_property in acl.yaml")
}

// A fixture directory that is a symlink is refused, so a fixture cannot
// load data from outside the project.
func TestFixtureSymlinkRefused(t *testing.T) {
	root := project(t, map[string]string{"tests/a.test.lua": `
local t = require("rela.test")
t.case("symlink", function()
  t.fixtures("fixtures/link")
  t.as("carol"):list("ticket")
end)
`})
	outside := t.TempDir()
	require.NoError(t, os.CopyFS(outside, os.DirFS(filepath.Join(root, "fixtures", "one"))))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "fixtures", "link")))
	rep := run(t, root, apptest.Options{})
	require.Equal(t, 1, rep.Failed)
	assert.Contains(t, rep.Cases[0].Message, "is a symlink")
}

func TestClockReachesEveryPath(t *testing.T) { // AC3
	rep := runLua(t, `
local t = require("rela.test")
t.case("clock", function()
  t.clock("2026-03-01T09:00:00Z")
  local carol = t.as("carol")
  local e = carol:create("ticket", {title = "clock", start = "2026-02-27"})
  t.expect(e.properties.stamp).to_be("2026-03-01")
  e = carol:get(e)
  t.expect(e.properties.note).to_be("2026-03-01")
  t.expect(e.properties.age).to_be(2)
  t.expect(e.mod_time).to_be("2026-03-01T09:00:00Z")
  t.expect(t.today()).to_be("2026-03-01")
  t.advance(t.days(1) + t.hours(1))
  t.expect(t.now()).to_be("2026-03-02T10:00:00Z")
  t.expect_error(function()
    carol:create("ticket", {title = "future", start = "2026-03-09"})
  end, "start")
  t.expect_error(function() t.advance(-1) end, "only moves forward")
end)
`)
	requireOK(t, rep)
}

// Out-of-range clock and random arguments are argument errors, not a clock
// that wraps around or a panic.
func TestClockAndRandomBounds(t *testing.T) {
	rep := runLua(t, `
local t = require("rela.test")
t.case("bounds", function()
  t.clock("2026-01-01")
  t.expect_error(function() t.advance(t.days(200000)) end, "the clock runs to 2261")
  t.expect_error(function() t.advance(0/0) end, "only moves forward")
  t.expect_error(function() t.clock("2300-01-01") end, "the clock runs from")
  t.expect(t.now()).to_be("2026-01-01T00:00:00Z")
  t.expect_error(function() t.random(-2^62, 2^62) end, "interval is too large")
end)
`)
	requireOK(t, rep)
}

// smtpTrap counts connections to a local port that stands in for the SMTP
// server mail.yaml names.
func smtpTrap(t *testing.T) (port string, conns *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	conns = &atomic.Int32{}
	go func() {
		for {
			c, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			conns.Add(1)
			_ = c.Close()
		}
	}()
	_, port, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return port, conns
}

func TestMailStaysInMemory(t *testing.T) { // AC4
	port, conns := smtpTrap(t)
	root := project(t, map[string]string{
		".rela/mail.yaml": "transport: smtp\nhost: 127.0.0.1\nport: " + port + "\nfrom: rela@example.com\n" +
			"recipients:\n  also_allow: [ok@example.com]\n",
		"tests/a.test.lua": `
local t = require("rela.test")
t.case("allowed", function()
  local carol = t.as("carol")
  local e = carol:create("ticket", {title = "m"})
  carol:transition(e, "done")
  t.expect(#t.mail()).to_be(1)
  t.expect(t.mail()[1].to[1]).to_be("ok@example.com")
end)
t.case("blocked", function()
  local carol = t.as("carol")
  local e = carol:create("ticket", {title = "m", note = "out@elsewhere.test"})
  t.expect_error(function() carol:transition(e, "done") end)
  t.expect(#t.mail()).to_be(0)
end)
`,
	})
	requireOK(t, run(t, root, apptest.Options{}))
	assert.Zero(t, conns.Load(), "rela test must not connect to the SMTP server")
}

func TestNetworkRefused(t *testing.T) { // AC5
	rep := runLua(t, `
local t = require("rela.test")
t.case("http", function() t.as("carol"):create("ticket", {title = "http"}) end)
t.case("ai", function() t.as("carol"):create("ticket", {title = "ai"}) end)
t.case("handled", function() t.as("carol"):create("ticket", {title = "http-quiet"}) end)
`)
	require.Equal(t, 3, rep.Failed)
	for _, c := range rep.Cases {
		want := "unexpected HTTP call GET http://127.0.0.1:9/; no stubs are registered"
		if c.Case == "ai" {
			want = "real AI calls are not allowed in rela test"
		}
		assert.Contains(t, c.Message, want, c.Case)
	}
}

// treeHash hashes every path, mode and content under root.
func treeHash(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		_, _ = h.Write([]byte(rel + "\x00" + info.Mode().String() + "\x00"))
		if d.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			_, _ = h.Write(data)
		}
		return nil
	})
	require.NoError(t, err)
	return hex.EncodeToString(h.Sum(nil))
}

// The case writes through every subsystem that keeps state beside the
// store: fixture comments, piles, a write_file script, mail, and a pending
// data migration. None of it may reach the project directory.
func TestProjectTreeUnchanged(t *testing.T) { // AC6
	schema, err := os.ReadFile(filepath.Join("testdata", "project", "schema.yaml"))
	require.NoError(t, err)
	live, err := metamodel.Parse(schema)
	require.NoError(t, err)
	old, err := metamodel.Parse([]byte(strings.Replace(string(schema), "note: {type: string}", "note: {type: integer}", 1)))
	require.NoError(t, err)
	mig, err := datamigration.Generate(metamodel.NewShapeProjection(old), metamodel.NewShapeProjection(live),
		"pending", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NotNil(t, mig)

	sideEffects := `
  - name: side-effects
    on: {entity: [ticket], created: true}
    do:
      - lua: |
          if entity.properties.title == "side" then
            rela.write_file("out.txt", "written")
            rela.piles.add{pile = "Inbox", entities = {entity.id}, create = true}
          end
        capabilities: {write_file: true}
`
	root := project(t, map[string]string{
		"schema.yaml":                string(schema) + sideEffects,
		"migrations/" + mig.FileName: string(mig.Content),
		"fixtures/one/.rela/comments/TKT-100.yaml": "comments:\n- id: c1\n  author: carol\n" +
			"  created_at: 2026-01-01T09:00:00+01:00\n  anchor: {kind: property, ref: title}\n  body: hi\n",
		"tests/a.test.lua": `
local t = require("rela.test")
t.case("writes", function()
  t.fixtures("fixtures/one")
  local carol = t.as("carol")
  local e = carol:create("ticket", {title = "side"})
  carol:transition(e, "done")
  carol:create("kinds", {})
  carol:delete("TKT-100")
end)
`})
	// kinds is not creatable by carol; give the editor role every type.
	require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(`user_entity_type: user
roles:
  editor:
    read: [ticket, user, kinds]
    create: [ticket, kinds]
    update: [ticket, kinds]
    delete: [ticket]
assignments:
  carol: editor
  "system:automation": editor
role_relations:
  member-of:
    requires_permission: manage-members
`), 0o600))
	before := treeHash(t, root)
	requireOK(t, run(t, root, apptest.Options{}))
	assert.Equal(t, before, treeHash(t, root))
}

func TestCasesAreIsolated(t *testing.T) { // AC7
	rep := run(t, project(t, map[string]string{"lib/helper.lua": "return {count = 0}\n", "tests/a.test.lua": `
local t = require("rela.test")
local helper = require("helper")
t.case("one", function()
  t.as("carol"):create("ticket", {title = "a"})
  t.expect(#t.as("carol"):list("ticket")).to_be(1)
  LEAK = 42
  helper.count = helper.count + 1
  t.expect = nil
end)
t.case("two", function()
  t.expect(#t.as("carol"):list("ticket")).to_be(0)
  t.expect(#t.mail()).to_be(0)
  t.expect(LEAK).to_be_nil()
  t.expect(helper.count).to_be(0)
end)
`}), apptest.Options{})
	requireOK(t, rep)
}

func TestFailureReportsLine(t *testing.T) { // AC8
	rep := runLua(t, `
local t = require("rela.test")
t.case("passes", function() t.expect(1).to_be(1) end)
t.case("fails", function()
  t.expect({1, 2}).to_equal({1, 2})
  t.expect("abc").to_contain("b")
  t.expect(1).to_be(2)
end)
t.case("errors", function()
  error("boom")
end)
`)
	require.Equal(t, 1, rep.Passed)
	require.Equal(t, 2, rep.Failed)
	assert.Equal(t, "tests/a.test.lua:7", rep.Cases[1].Location)
	assert.Equal(t, "expected 2, got 1", rep.Cases[1].Message)
	assert.Equal(t, "tests/a.test.lua:10", rep.Cases[2].Location)
	assert.Equal(t, "boom", rep.Cases[2].Message)

	var sb strings.Builder
	require.NoError(t, rep.WriteText(&sb))
	assert.Contains(t, sb.String(), "not ok  tests/a.test.lua: fails\n        tests/a.test.lua:7: expected 2, got 1")
	assert.Contains(t, sb.String(), "1 passed, 2 failed (seed 1)")
}

func TestFileLevelFailures(t *testing.T) {
	root := project(t, map[string]string{
		"tests/syntax.test.lua": "local t = require('rela.test')\nt.case(\n",
		"tests/empty.test.lua":  "local t = require('rela.test')\n",
		"tests/top.test.lua":    "local t = require('rela.test')\nt.as('x')\n",
	})
	rep := run(t, root, apptest.Options{})
	require.Equal(t, 3, rep.Failed)
	byFile := map[string]apptest.CaseResult{}
	for _, c := range rep.Cases {
		byFile[c.File] = c
	}
	assert.Contains(t, byFile["tests/empty.test.lua"].Message, "declares no cases")
	assert.Contains(t, byFile["tests/syntax.test.lua"].Message, "compile")
	assert.Contains(t, byFile["tests/top.test.lua"].Message, "inside a case")
}

func TestSandbox(t *testing.T) { // AC9
	rep := runLua(t, `
local t = require("rela.test")
t.case("no unsafe libraries", function()
  for _, name in ipairs({"os", "io", "debug", "load", "loadstring", "loadfile", "dofile"}) do
    t.expect(_G[name]).to_be_nil()
  end
  t.expect_error(function() math.randomseed(1) end, "--seed")
end)
t.case("random", function()
  t.fail(tostring(math.random(1000000)) .. " " .. tostring(t.random(1000000)))
end)
`)
	require.Equal(t, 1, rep.Passed)
	first := rep.Cases[1].Message
	again := runLua(t, `
local t = require("rela.test")
t.case("no unsafe libraries", function() end)
t.case("random", function()
  t.fail(tostring(math.random(1000000)) .. " " .. tostring(t.random(1000000)))
end)
`)
	assert.Equal(t, first, again.Cases[1].Message)
}

// Each case has its own seed, so the 200 cases draw 200 sets of values.
func TestCreateGeneratesEveryKind(t *testing.T) { // AC11
	rep := runLua(t, `
local t = require("rela.test")
for n = 1, 200 do
  t.case("kinds " .. n, function()
    local carol = t.as("carol")
    local e, warnings = carol:create("kinds", {})
    t.expect(#warnings).to_be(0)
    t.expect(e.id:sub(1, 2)).to_be("K-")
    for _, p in ipairs({"s", "d", "dt", "i", "dec", "b", "e", "r", "f", "st", "neg"}) do
      t.expect(e.properties[p]).not_to_be_nil()
    end
    t.expect(e.properties.i >= 3 and e.properties.i <= 9).to_be(true)
    t.expect(e.properties.st).to_be("todo")
    local neg = tonumber(e.properties.neg)
    t.expect(neg >= -5 and neg <= -1).to_be(true)
    t.expect({"x", "y"}).to_contain(t.fake.value("kinds", "e"))
    t.expect(t.fake.entity("ticket").title).not_to_be_nil()
  end)
end
`)
	requireOK(t, rep)
	require.Equal(t, 200, rep.Passed)
}

// valuesOf returns the message of each case, which these cases set to the
// values they generated.
func valuesOf(rep *apptest.Report) map[string]string {
	out := map[string]string{}
	for _, c := range rep.Cases {
		out[c.Case] = c.Message
	}
	return out
}

const reportValues = `
t.case("values", function()
  t.fail(table.concat({t.fake.name(), t.fake.email(), t.now(), tostring(t.random(1000000)),
    t.fake.date{within = 30}}, " | "))
end)
`

func TestSeedReproduces(t *testing.T) { // AC12, AC13
	root := project(t, map[string]string{"tests/a.test.lua": "local t = require('rela.test')\n" + reportValues})
	seed7 := valuesOf(run(t, root, apptest.Options{Seed: 7, SeedSet: true}))
	assert.Equal(t, seed7, valuesOf(run(t, root, apptest.Options{Seed: 7, SeedSet: true})))
	seed8 := valuesOf(run(t, root, apptest.Options{Seed: 8, SeedSet: true}))
	assert.NotEqual(t, seed7["values"], seed8["values"])
	assert.NotEqual(t, strings.Split(seed7["values"], " | ")[2], strings.Split(seed8["values"], " | ")[2],
		"the starting clock is derived from the seed")

	more := project(t, map[string]string{"tests/a.test.lua": "local t = require('rela.test')\n" +
		"t.case('first', function() t.fake.name() end)\n" + reportValues})
	assert.Equal(t, seed7["values"], valuesOf(run(t, more, apptest.Options{Seed: 7, SeedSet: true}))["values"],
		"adding a case does not change another case's values")

	rep, err := apptest.Run(context.Background(), apptest.Options{Root: root})
	require.NoError(t, err)
	assert.Less(t, rep.Seed, uint64(1)<<53)
}

func TestMissingConfig(t *testing.T) { // AC15
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	_, err := apptest.Run(context.Background(), apptest.Options{Root: root})
	var missing *apptest.MissingConfigError
	require.ErrorAs(t, err, &missing)
	assert.Contains(t, err.Error(), "schema.yaml")
	assert.Contains(t, err.Error(), "TKT-DAUS290")
}

func TestRunFilterAndPaths(t *testing.T) {
	root := project(t, map[string]string{
		"tests/a.test.lua":     "local t = require('rela.test')\nt.case('alpha', function() end)\nt.case('beta', function() end)\n",
		"tests/sub/b.test.lua": "local t = require('rela.test')\nt.case('gamma', function() end)\n",
		"tests/helper.lua":     "return {}\n",
	})
	rep := run(t, root, apptest.Options{Run: "^(alpha|gamma)$"})
	assert.Equal(t, 2, rep.Passed)
	rep = run(t, root, apptest.Options{Paths: []string{"tests/sub"}})
	assert.Equal(t, 1, rep.Passed)
	_, err := apptest.Run(context.Background(), apptest.Options{Root: root, Paths: []string{"../x"}})
	require.Error(t, err)
	_, err = apptest.Run(context.Background(), apptest.Options{Root: root, Run: "("})
	require.Error(t, err)
	_, err = apptest.Run(context.Background(), apptest.Options{Root: project(t, nil)})
	require.ErrorIs(t, err, apptest.ErrNoTests)
}

func TestNoGoroutineLeak(t *testing.T) { // AC16
	ignore := goleak.IgnoreCurrent()
	var code strings.Builder
	code.WriteString("local t = require('rela.test')\nfor i = 1, 100 do\n" +
		"  t.case('c' .. i, function() t.as('carol'):create('ticket', {title = 'x'}) end)\nend\n")
	rep := runLua(t, code.String())
	requireOK(t, rep)
	require.Equal(t, 100, rep.Passed)
	goleak.VerifyNone(t, ignore)
}
