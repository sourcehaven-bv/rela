//go:build sqlite || postgres

package appbuild_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// loopMetamodel has automations that rewrite the entity after a save: mark
// runs inline on a create and on a title change, inside the connector's
// write, so the connector's tag after that write always conflicts and the
// loop must re-read and re-merge (D4); stamp runs as a background job,
// whenever the queue gets to it.
const loopMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "T-"
    id_type: sequential
    properties:
      title: {type: string}
      due: {type: date}
      done: {type: boolean}
      stamp: {type: string}
      mark: {type: string}
      basecamp:
        type: external_ref
        system: basecamp
        sync: true
automations:
  - name: stamp
    on:
      entity: ticket
      created: true
      updated: true
    do:
      - lua_file: stamp.lua
        background: true
  - name: mark-new
    on:
      entity: ticket
      created: true
    do:
      - lua_file: mark.lua
  - name: mark
    on:
      entity: ticket
      property: title
    do:
      - lua_file: mark.lua
`

const stampScript = `
local want = "seen:" .. tostring(entity.properties.title) .. ":" .. tostring(entity.properties.done)
if entity.properties.stamp ~= want then
  rela.update_entity(entity.id, {stamp = want})
end
`

const markScript = `
local want = "m:" .. tostring(entity.properties.title)
if entity.properties.mark ~= want then
  rela.update_entity(entity.id, {mark = want})
end
`

// loopScript is the reference loop of the Lua guide, run over a stub remote
// passed in as JSON. It prints the remote back and how many writes, pushes,
// creates and tag retries the round made.
const loopScript = `
local remote = rela.json.decode(REMOTE)
local FIELDS = {"title", "due", "done", "content"}
local changes, retries = 0, 0

local function theirs_of(r)
  return {
    properties = {
      title = r.title,
      due = r.due_on or rela.sync.EMPTY,
      done = r.completed,
    },
    content = r.description,
  }
end

local function push(r, fields, etag)
  if r.etag ~= etag then return false end
  for k, v in pairs(fields) do
    if v == rela.sync.EMPTY then v = nil end
    if k == "title" then r.title = v
    elseif k == "due" then r.due_on = v
    elseif k == "done" then r.completed = v
    elseif k == "content" then r.description = v end
  end
  r.etag = r.etag + 1
  return true
end

local function sync_once(id, r)
  local base = rela.version_by_tag(id, "sync/basecamp")
  local read_tok = rela.version_token(id)
  local ours = rela.get_entity(id)
  local m = rela.sync.merge(base, ours, theirs_of(r), {fields = FIELDS})
  if #m.conflicts > 0 then return "conflict" end
  local tok = read_tok
  if next(m.write) or m.content then
    local w, _, t = rela.update_entity(id, m.write, m.content, {expect = read_tok})
    if not w then return "retry" end
    tok = t
    changes = changes + 1
  end
  if next(m.push) then
    if not push(r, m.push, r.etag) then return "done" end
    changes = changes + 1
  end
  -- Move the base only after a complete report: a field the remote left
  -- out may carry a local edit that was never pushed.
  if m.complete and (next(m.write) or m.content or next(m.push) or m.retag) then
    if not rela.tag_version(id, "sync/basecamp", {expect = tok}) then return "retry" end
    rela.output("tagged_" .. id .. "=" .. tok)
  end
  return "done"
end

for _, r in ipairs(remote) do
  local e = rela.find_by_external_ref("basecamp", r.id)
  if not e then
    local created, _, tok = rela.create_entity("ticket", {
      title = r.title, due = r.due_on, done = r.completed,
      basecamp = {id = r.id},
    }, r.description, nil, {token = true})
    changes = changes + 1
    if rela.tag_version(created.id, "sync/basecamp", {expect = tok}) then
      rela.output("tagged_" .. created.id .. "=" .. tok)
    else
      -- An automation rewrote the entity after the create. Merge it with
      -- no base: a complete, converged report retags with the read token.
      retries = retries + 1
      e = created
    end
  end
  if e then
    local outcome
    for _ = 1, 3 do
      outcome = sync_once(e.id, r)
      if outcome ~= "retry" then break end
      retries = retries + 1
    end
    if outcome ~= "done" then rela.output("unsettled=" .. e.id .. ":" .. outcome) end
  end
end
rela.output("changes=" .. changes)
rela.output("retries=" .. retries)
rela.output("remote=" .. rela.json.encode(remote))
`

// stubTodo is one remote item. The stub stores what it is sent in its own
// form: a date as a midnight timestamp, a body with CRLF line endings and a
// trailing newline. A loop that compared by form would push it back forever.
type stubTodo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	DueOn       *string `json:"due_on,omitempty"`
	Completed   bool    `json:"completed"`
	Description string  `json:"description"`
	ETag        int     `json:"etag"`
}

func (s *stubTodo) normalize() {
	if s.DueOn != nil && len(*s.DueOn) == len("2006-01-02") {
		v := *s.DueOn + "T00:00:00Z"
		s.DueOn = &v
	}
	body := strings.ReplaceAll(strings.TrimRight(s.Description, " \r\n"), "\r\n", "\n")
	s.Description = strings.ReplaceAll(body, "\n", "\r\n") + "\r\n"
}

type loopWorld struct {
	t      *testing.T
	svc    *appbuild.Services
	remote []stubTodo
	// tagged is, per entity id, the token the loop last moved its base to.
	tagged map[string]string
}

// round runs the connector once and returns how many changes it made and
// how many tag retries it needed.
func (w *loopWorld) round() (changes, retries int) {
	w.t.Helper()
	for i := range w.remote {
		w.remote[i].normalize()
	}
	data, err := json.Marshal(w.remote)
	require.NoError(w.t, err)
	src := "local REMOTE = " + strconv.Quote(string(data)) + "\n" + loopScript
	got, err := syncLua(asUser("connector"), w.t, w.svc, src)
	require.NoError(w.t, err)
	require.Empty(w.t, got["unsettled"])
	require.NoError(w.t, json.Unmarshal([]byte(got["remote"]), &w.remote))
	for k, v := range got {
		if id, ok := strings.CutPrefix(k, "tagged_"); ok {
			w.tagged[id] = v
		}
	}
	changes, err = strconv.Atoi(got["changes"])
	require.NoError(w.t, err)
	retries, err = strconv.Atoi(got["retries"])
	require.NoError(w.t, err)
	return changes, retries
}

// settle runs rounds until one changes nothing; it must take at most two
// working rounds after a perturbation.
func (w *loopWorld) settle() (retries int) {
	w.t.Helper()
	for range 2 {
		c, r := w.round()
		retries += r
		if c == 0 {
			return retries
		}
	}
	c, _ := w.round()
	require.Zero(w.t, c, "no fixed point after two working rounds")
	return retries
}

// baseToken is the token of the state the sync/basecamp tag of id points
// at, or "" when there is no tag. The connector reads every field, so a
// version's token is store.VersionOf of its snapshot.
func (w *loopWorld) baseToken(id string) string {
	w.t.Helper()
	tr := appbuild.VersionTagReader(w.svc)
	require.NotNil(w.t, tr, "the store serves no version tags")
	name, err := store.ParseVersionTagName("sync/basecamp")
	require.NoError(w.t, err)
	snap, err := tr.VersionByTag(asUser("connector"), entity.Ref{ID: id}, name)
	if errors.Is(err, store.ErrNotFound) {
		return ""
	}
	require.NoError(w.t, err)
	return string(store.VersionOf(&entity.Entity{
		ID: id, Type: snap.Type, Content: snap.Content, Properties: snap.Properties,
	}))
}

// currentToken is rela.version_token(id) as the connector reads it.
func (w *loopWorld) currentToken(id string) string {
	w.t.Helper()
	got, err := syncLua(asUser("connector"), w.t, w.svc, `rela.output("tok=" .. rela.version_token("`+id+`"))`)
	require.NoError(w.t, err)
	return got["tok"]
}

// requireBaseAt asserts that the sync/basecamp tag of id points at the
// version whose token the loop last tagged with and, when atCurrent, that
// this is also the entity's current token: the base moved onto exactly the
// state the loop merged (D1).
func (w *loopWorld) requireBaseAt(id string, atCurrent bool) {
	w.t.Helper()
	want := w.tagged[id]
	require.NotEmpty(w.t, want, "the loop never tagged %s", id)
	require.Equal(w.t, want, w.baseToken(id), "the tag is not on the state the loop tagged")
	if atCurrent {
		require.Equal(w.t, want, w.currentToken(id), "the tag is not on the current state")
	}
}

func (w *loopWorld) local(id string) *entity.Entity {
	w.t.Helper()
	got, err := w.svc.Store().GetEntity(asUser("connector"), entity.Ref{ID: id})
	require.NoError(w.t, err)
	return got
}

// AC13 (TKT-SM20FG, D12): a pull/push loop against a normalizing stub, with
// an automation rewriting every save, reaches a fixed point within two
// rounds and loses neither side's edit.
func TestSyncLoop_ReachesFixedPoint(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, loopMetamodel)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "stamp.lua"), []byte(stampScript), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "mark.lua"), []byte(markScript), 0o600))
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	due := "2026-10-01"
	w := &loopWorld{t: t, svc: svc, tagged: map[string]string{}, remote: []stubTodo{{
		ID: "bc-1", Title: "Plan", DueOn: &due, Description: "first  \nsecond",
	}}}

	// First sync creates the ticket. The inline mark automation rewrites it
	// inside the create, so the tag with the create's token conflicts; the
	// loop merges with no base, finds the complete report converged, and
	// retags with the token it read (base unknown, D1).
	changes, retries := w.round()
	require.Equal(t, 1, changes)
	require.Positive(t, retries, "the create's tag should have conflicted with the mark automation")
	found := findTicket(t, w, "bc-1")
	require.Equal(t, "m:Plan", found.Properties["mark"])
	w.requireBaseAt(found.ID, false)
	// The next round changes nothing, though the stub rewrote the date and
	// the body in its own form.
	w.settle()
	require.Equal(t, "Plan", w.local(found.ID).Properties["title"])
	waitStamp(t, w, found.ID, "seen:Plan:false")

	// Concurrent edits on both sides: rela edits the body, the remote
	// renames and completes.
	body := "first\nsecond\nthird"
	_, err = svc.EntityManager().PatchEntity(asUser("user"), found.ID, entity.Patch{Content: &body})
	require.NoError(t, err)
	w.remote[0].Title = "Plan v2"
	w.remote[0].Completed = true
	w.remote[0].ETag++

	retries = w.settle()
	got := w.local(found.ID)
	require.Equal(t, "Plan v2", got.Properties["title"])
	require.Equal(t, true, got.Properties["done"])
	require.Equal(t, body, got.Content)
	require.Equal(t, "first\r\nsecond\r\nthird\r\n", w.remote[0].Description)
	require.True(t, w.remote[0].Completed)
	require.Equal(t, "m:Plan v2", got.Properties["mark"])
	// The write of the title ran the inline mark automation after the
	// connector's write, so its first tag conflicted and the loop re-merged.
	require.Positive(t, retries)
	w.requireBaseAt(found.ID, false)
	waitStamp(t, w, found.ID, "seen:Plan v2:true")

	// Both sides clear the due date the same way: converged, the base moves
	// with a retag onto the current state and nothing is written or pushed.
	// The stamp has settled and does not change for this edit, so the tag
	// must end on the current token.
	before := w.tagged[found.ID]
	_, err = svc.EntityManager().PatchEntity(asUser("user"), found.ID, entity.Patch{MetaUnset: []string{"due"}})
	require.NoError(t, err)
	w.remote[0].DueOn = nil
	w.remote[0].ETag++
	changes, _ = w.round()
	require.Zero(t, changes, "a converged clear writes and pushes nothing")
	_, hasDue := w.local(found.ID).Properties["due"]
	require.False(t, hasDue)
	require.Nil(t, w.remote[0].DueOn)
	require.NotEqual(t, before, w.tagged[found.ID], "the converged clear did not retag")
	w.requireBaseAt(found.ID, true)
	w.settle()
	w.requireBaseAt(found.ID, true)
}

func waitStamp(t *testing.T, w *loopWorld, id, want string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return w.local(id).Properties["stamp"] == want
	}, 10*time.Second, 20*time.Millisecond, "the background stamp never ran")
}

func findTicket(t *testing.T, w *loopWorld, extID string) *entity.Entity {
	t.Helper()
	got, err := syncLua(asUser("connector"), t, w.svc,
		`rela.output("id=" .. rela.find_by_external_ref("basecamp", "`+extID+`").id)`)
	require.NoError(t, err)
	return w.local(got["id"])
}
