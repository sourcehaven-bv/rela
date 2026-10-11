//go:build sqlite

package appbuild_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

// The Basecamp example (examples/basecamp, TKT-01KZSO) run in place
// against a stub of the Basecamp API and the Launchpad token endpoint.

const (
	bcAccount = "999"
	bcList    = "42"
	bcClient  = "client-id"
	bcSecret  = "client-secret"
)

// bcPerson is a person on a stub to-do.
type bcPerson struct {
	ID int64 `json:"id"`
}

// bcTodo is a stub to-do in Basecamp's shape.
type bcTodo struct {
	ID                    int64      `json:"id"`
	Content               string     `json:"content"`
	Description           string     `json:"description"`
	DueOn                 *string    `json:"due_on"`
	StartsOn              *string    `json:"starts_on"`
	Completed             bool       `json:"completed"`
	UpdatedAt             string     `json:"updated_at"`
	AppURL                string     `json:"app_url"`
	Assignees             []bcPerson `json:"assignees"`
	CompletionSubscribers []bcPerson `json:"completion_subscribers"`
}

// bcStub is the Basecamp API and the Launchpad token endpoint. Like
// Basecamp it refuses a request without a contact in its User-Agent, and a
// PUT clears every field it leaves out.
type bcStub struct {
	srv *httptest.Server

	mu       sync.Mutex
	todos    map[int64]*bcTodo
	nextID   int64
	clock    int
	pageSize int

	access, refresh string
	gen             int
	refreshCalls    int
	// calls are the API requests, "METHOD path?query"; 304s are marked.
	calls []string
	// limited answers the next n API requests with 429.
	limited int
	// unavailable answers the next n completion POSTs with 503.
	unavailable int
}

var bcUserAgent = regexp.MustCompile(`\(.+@.+\)`)

func newBCStub(t *testing.T) *bcStub {
	t.Helper()
	s := &bcStub{todos: map[int64]*bcTodo{}, nextID: 100, pageSize: 2, refresh: "R0"}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *bcStub) stamp() string {
	s.clock++
	return time.Date(2026, 1, 1, 0, 0, s.clock, 0, time.UTC).Format(time.RFC3339)
}

// add creates a remote to-do as a Basecamp user would.
func (s *bcStub) add(title, due string, done bool, assignees ...int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	td := &bcTodo{ID: s.nextID, Content: title, Completed: done, UpdatedAt: s.stamp(),
		AppURL: fmt.Sprintf("%s/app/todos/%d", s.srv.URL, s.nextID)}
	if due != "" {
		td.DueOn = &due
	}
	for _, a := range assignees {
		td.Assignees = append(td.Assignees, bcPerson{ID: a})
	}
	s.todos[td.ID] = td
	return td.ID
}

// edit changes a remote to-do as a Basecamp user would.
func (s *bcStub) edit(id int64, fn func(*bcTodo)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.todos[id])
	s.todos[id].UpdatedAt = s.stamp()
}

func (s *bcStub) todo(id int64) bcTodo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.todos[id]
}

func (s *bcStub) remove(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.todos, id)
}

// revoke makes Basecamp reject the current access token.
func (s *bcStub) revoke() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.access = "revoked"
}

// takeCalls returns and clears the recorded API calls.
func (s *bcStub) takeCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.calls
	s.calls = nil
	return c
}

func (s *bcStub) refreshes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCalls
}

func (s *bcStub) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !bcUserAgent.MatchString(r.Header.Get("User-Agent")) {
		http.Error(w, "User-Agent must name a contact", http.StatusBadRequest)
		return
	}
	if r.URL.Path == "/authorization/token" {
		s.serveToken(w, r)
		return
	}
	call := r.Method + " " + r.URL.RequestURI()
	if s.limited > 0 {
		s.limited--
		s.calls = append(s.calls, call+" 429")
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+s.access {
		s.calls = append(s.calls, call+" 401")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	prefix := "/" + bcAccount + "/"
	path, ok := strings.CutPrefix(r.URL.Path, prefix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if path == "todolists/"+bcList+"/todos.json" {
		if r.Method == http.MethodPost {
			s.calls = append(s.calls, call)
			s.create(w, r)
			return
		}
		s.list(w, r, call)
		return
	}
	s.calls = append(s.calls, call)
	rest, ok := strings.CutPrefix(path, "todos/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	idText, completion := strings.CutSuffix(rest, "/completion.json")
	if !completion {
		idText = strings.TrimSuffix(rest, ".json")
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	td := s.todos[id]
	if err != nil || td == nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case completion && r.Method == http.MethodPost && s.unavailable > 0:
		s.unavailable--
		w.WriteHeader(http.StatusServiceUnavailable)
	case completion && r.Method == http.MethodPost:
		td.Completed, td.UpdatedAt = true, s.stamp()
		w.WriteHeader(http.StatusNoContent)
	case completion && r.Method == http.MethodDelete:
		td.Completed, td.UpdatedAt = false, s.stamp()
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet:
		writeJSON(w, td)
	case r.Method == http.MethodPut:
		s.put(w, r, td)
	default:
		http.Error(w, "method", http.StatusMethodNotAllowed)
	}
}

func (s *bcStub) serveToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.URL.RawQuery != "" {
		http.Error(w, "form body only", http.StatusBadRequest)
		return
	}
	f := r.PostForm
	s.refreshCalls++
	if f.Get("grant_type") != "refresh_token" || f.Get("type") != "refresh" ||
		f.Get("client_id") != bcClient || f.Get("client_secret") != bcSecret {

		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}
	if f.Get("refresh_token") != s.refresh {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	// Rotate both tokens, as some providers do.
	s.gen++
	s.access = fmt.Sprintf("A%d", s.gen)
	s.refresh = fmt.Sprintf("R%d", s.gen)
	writeJSON(w, map[string]any{"access_token": s.access, "refresh_token": s.refresh, "expires_in": "1209600"})
}

func (s *bcStub) list(w http.ResponseWriter, r *http.Request, call string) {
	q := r.URL.Query()
	completed := q.Get("completed") == "true"
	var ids []int64
	for id, td := range s.todos {
		if td.Completed == completed {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	page := 1
	if p := q.Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	start := min((page-1)*s.pageSize, len(ids))
	end := min(start+s.pageSize, len(ids))
	items := make([]*bcTodo, 0, end-start)
	for _, id := range ids[start:end] {
		items = append(items, s.todos[id])
	}
	body, err := json.Marshal(items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if end < len(ids) {
		next := q
		next.Set("page", strconv.Itoa(page+1))
		w.Header().Set("Link", fmt.Sprintf(`<%s%s?%s>; rel="next"`, s.srv.URL, r.URL.Path, next.Encode()))
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	if r.Header.Get("If-None-Match") == etag {
		s.calls = append(s.calls, call+" 304")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	s.calls = append(s.calls, call)
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// bcWrite is what a POST or PUT may send.
type bcWrite struct {
	Content                 string  `json:"content"`
	Description             string  `json:"description"`
	DueOn                   *string `json:"due_on"`
	StartsOn                *string `json:"starts_on"`
	AssigneeIDs             []int64 `json:"assignee_ids"`
	CompletionSubscriberIDs []int64 `json:"completion_subscriber_ids"`
}

func (s *bcStub) create(w http.ResponseWriter, r *http.Request) {
	var in bcWrite
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Content == "" {
		http.Error(w, "content required", http.StatusUnprocessableEntity)
		return
	}
	s.nextID++
	td := &bcTodo{ID: s.nextID, Content: in.Content, Description: in.Description, DueOn: in.DueOn,
		UpdatedAt: s.stamp(), AppURL: fmt.Sprintf("%s/app/todos/%d", s.srv.URL, s.nextID)}
	s.todos[td.ID] = td
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, td)
}

// put replaces the to-do: a field the body leaves out is cleared.
func (s *bcStub) put(w http.ResponseWriter, r *http.Request, td *bcTodo) {
	var in bcWrite
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Content == "" {
		http.Error(w, "content required", http.StatusUnprocessableEntity)
		return
	}
	td.Content, td.Description, td.DueOn, td.StartsOn = in.Content, in.Description, in.DueOn, in.StartsOn
	td.Assignees, td.CompletionSubscribers = nil, nil
	for _, id := range in.AssigneeIDs {
		td.Assignees = append(td.Assignees, bcPerson{ID: id})
	}
	for _, id := range in.CompletionSubscriberIDs {
		td.CompletionSubscribers = append(td.CompletionSubscribers, bcPerson{ID: id})
	}
	td.UpdatedAt = s.stamp()
	writeJSON(w, td)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// bcWorld is a sqlite project holding the example's files.
type bcWorld struct {
	t    *testing.T
	stub *bcStub
	svc  *appbuild.Services
	caps lua.Capabilities
}

var connector = principal.Principal{User: "integration:basecamp", Tool: principal.ToolScheduler}

func exampleFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "basecamp", name))
	require.NoError(t, err)
	return data
}

func newBCWorld(t *testing.T) *bcWorld {
	t.Helper()
	stub := newBCStub(t)
	root := t.TempDir()
	writeMetamodelBody(t, root, string(exampleFile(t, "schema.yaml")))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))
	write := func(name string, data []byte) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), data, 0o600))
	}
	write(filepath.Join("scripts", "basecamp.lua"), exampleFile(t, "basecamp.lua"))
	write("schedules.yaml", exampleFile(t, "schedules.yaml"))
	write("connections.yaml", []byte(strings.Replace(string(exampleFile(t, "connections.yaml")),
		"https://launchpad.37signals.com", stub.srv.URL, 1)))

	// The example's role, plus an editor for the person using rela.
	var policy map[string]map[string]any
	require.NoError(t, yaml.Unmarshal(exampleFile(t, "acl.yaml"), &policy))
	policy["roles"]["editor"] = map[string]any{"read": []string{"todo"}, "create": []string{"todo"},
		"update": []string{"todo"}}
	policy["assignments"]["alice"] = "editor"
	aclYAML, err := yaml.Marshal(policy)
	require.NoError(t, err)
	write("acl.yaml", aclYAML)

	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	write(filepath.Join(".rela", "secrets.yaml"), fmt.Appendf(nil,
		"token_key: %s\nbasecamp_client_id: %s\nbasecamp_client_secret: %s\n"+
			"basecamp_account_id: %q\nbasecamp_todolist_id: %q\nbasecamp_api_base: %s\n",
		key, bcClient, bcSecret, bcAccount, bcList, stub.srv.URL))

	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	cfg, err := scheduler.ParseConfig(exampleFile(t, "schedules.yaml"))
	require.NoError(t, err)
	httpOK, ai, mail, writeFile, secrets, tokens := cfg.Tasks[0].Capabilities.Fields()
	w := &bcWorld{t: t, stub: stub, svc: svc, caps: lua.Capabilities{
		HTTP: httpOK, AI: ai, Mail: mail, WriteFile: writeFile, Secrets: secrets, Tokens: tokens,
	}}

	// What `rela token set basecamp` stores after the consent flow.
	broker, err := appbuild.Tokens(svc)
	require.NoError(t, err)
	require.NoError(t, broker.Set(context.Background(), "basecamp", tokenstore.Token{Refresh: "R0"}))
	return w
}

// run executes the example as the scheduler does, or, with e set, as the
// push automation does.
func (w *bcWorld) run(e *entity.Entity) error {
	w.t.Helper()
	ctx := principal.With(context.Background(), connector)
	deps := w.svc.ScheduledLuaWriteDeps()
	deps.Capabilities = w.caps
	return w.svc.ScriptEngine().ExecuteFile(ctx, "basecamp.lua", deps, e, nil)
}

func (w *bcWorld) pull() {
	w.t.Helper()
	require.NoError(w.t, w.run(nil))
}

// byRef returns the todo linked to the remote id.
func (w *bcWorld) byRef(id int64) *entity.Entity {
	w.t.Helper()
	want := strconv.FormatInt(id, 10)
	for e, err := range w.svc.Store().ListEntities(context.Background(), store.EntityQuery{Type: "todo", Faces: store.AllFaces()}) {
		require.NoError(w.t, err)
		if ref, ok := e.Properties["basecamp"].(map[string]any); ok && ref["id"] == want {
			return e
		}
	}
	w.t.Fatalf("no todo for remote %d", id)
	return nil
}

func (w *bcWorld) get(id string) *entity.Entity {
	w.t.Helper()
	e, err := w.svc.Store().GetEntity(context.Background(), entity.Ref{ID: id})
	require.NoError(w.t, err)
	return e
}

// base is the version the sync/basecamp tag points at, 0 for none.
func (w *bcWorld) base(id string) int {
	w.t.Helper()
	name, err := store.ParseVersionTagName("sync/basecamp")
	require.NoError(w.t, err)
	snap, err := appbuild.VersionTagReader(w.svc).VersionByTag(
		principal.With(context.Background(), connector), entity.Ref{ID: id}, name)
	if err != nil {
		return 0
	}
	return snap.Version
}

func (w *bcWorld) alice() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
}

// writes are the calls that change Basecamp.
func writes(calls []string) []string {
	var out []string
	for _, c := range calls {
		if !strings.HasPrefix(c, "GET ") {
			out = append(out, c)
		}
	}
	return out
}

// settle pulls until a round changes nothing on either side; that must
// take at most two working rounds.
func (w *bcWorld) settle() {
	w.t.Helper()
	for range 3 {
		before := w.stamps()
		w.stub.takeCalls()
		w.pull()
		if len(writes(w.stub.takeCalls())) == 0 && w.stamps() == before {
			return
		}
	}
	w.t.Fatal("no fixed point within two working rounds")
}

// stamps fingerprints every todo's content and tag.
func (w *bcWorld) stamps() string {
	w.t.Helper()
	var parts []string
	for e, err := range w.svc.Store().ListEntities(context.Background(), store.EntityQuery{Type: "todo", Faces: store.AllFaces()}) {
		require.NoError(w.t, err)
		full := w.get(e.ID)
		parts = append(parts, fmt.Sprintf("%s=%s@%d", e.ID, store.VersionOf(full), w.base(e.ID)))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func TestBasecampExample(t *testing.T) {
	w := newBCWorld(t)
	stub := w.stub
	plan := stub.add("Plan", "2026-10-01", false, 7)
	write := stub.add("Write", "", false)
	ship := stub.add("Ship", "", true)

	// First pull: three todos across pages, each with a base. The stored
	// token only had a refresh token, so one refresh, which rotated it.
	w.pull()
	require.Equal(t, 1, stub.refreshes())
	planID := w.byRef(plan).ID
	require.Equal(t, "Plan", w.get(planID).Properties["title"])
	require.Equal(t, true, w.byRef(ship).Properties["done"])
	for _, id := range []int64{plan, write, ship} {
		require.Positive(t, w.base(w.byRef(id).ID), "todo for %d has no base", id)
	}
	require.Empty(t, writes(stub.takeCalls()), "the first pull wrote to Basecamp")

	// Fixed point: the next pull changes nothing, and its unchanged pages
	// come back as 304s.
	w.settle()
	w.pull()
	for _, c := range stub.takeCalls() {
		require.True(t, strings.HasSuffix(c, " 304"), "unchanged page fetched in full: %s", c)
	}

	// R7: after a pull the push makes no HTTP call at all.
	for _, id := range []int64{plan, write, ship} {
		require.NoError(t, w.run(w.byRef(id)))
	}
	require.Empty(t, stub.takeCalls(), "push called Basecamp for a todo the pull had just synced")
	require.Equal(t, 1, stub.refreshes())

	t.Run("local edit pushes and keeps the fields rela does not own", func(t *testing.T) {
		title := "Plan v2"
		_, err := w.svc.EntityManager().PatchEntity(w.alice(), planID,
			entity.Patch{Properties: map[string]any{"title": title}})
		require.NoError(t, err)
		require.Equal(t, title, stub.todo(plan).Content)
		require.Equal(t, []bcPerson{{ID: 7}}, stub.todo(plan).Assignees, "the PUT dropped the assignee")
		require.Equal(t, "2026-10-01", *stub.todo(plan).DueOn)
		w.settle()
		require.Equal(t, title, w.get(planID).Properties["title"])
	})

	t.Run("local completion uses the completion endpoint", func(t *testing.T) {
		stub.takeCalls()
		_, err := w.svc.EntityManager().PatchEntity(w.alice(), w.byRef(write).ID,
			entity.Patch{Properties: map[string]any{"done": true}})
		require.NoError(t, err)
		require.True(t, stub.todo(write).Completed)
		require.Contains(t, writes(stub.takeCalls()), fmt.Sprintf("POST /%s/todos/%d/completion.json", bcAccount, write))
		w.settle()
	})

	t.Run("remote edit pulls", func(t *testing.T) {
		stub.edit(ship, func(td *bcTodo) { td.Content = "Ship it"; td.Description = "<div>now</div>" })
		w.settle()
		got := w.byRef(ship)
		require.Equal(t, "Ship it", got.Properties["title"])
		require.Equal(t, "<div>now</div>", w.get(got.ID).Content)
	})

	t.Run("local create posts and links", func(t *testing.T) {
		e := entity.New("", "todo")
		e.SetString("title", "Fresh")
		created, err := w.svc.EntityManager().CreateEntity(w.alice(), e, entity.CreateOptions{})
		require.NoError(t, err)
		got := w.get(created.Entity.ID)
		ref, ok := got.Properties["basecamp"].(map[string]any)
		require.True(t, ok, "push did not link the new todo")
		rid, err := strconv.ParseInt(ref["id"].(string), 10, 64)
		require.NoError(t, err)
		require.Equal(t, "Fresh", stub.todo(rid).Content)
		require.True(t, strings.HasSuffix(ref["url"].(string), "#todolist-"+bcList), "ref url %v", ref["url"])
		require.Positive(t, w.base(got.ID))
		w.settle()
	})

	t.Run("a retried create does not post a second to-do", func(t *testing.T) {
		countFinish := func() int {
			stub.mu.Lock()
			defer stub.mu.Unlock()
			n := 0
			for _, td := range stub.todos {
				if td.Content == "Finish" {
					n++
				}
			}
			return n
		}
		stub.takeCalls()
		stub.unavailable = 1
		e := entity.New("", "todo")
		e.SetString("title", "Finish")
		e.Properties["done"] = true
		created, err := w.svc.EntityManager().CreateEntity(w.alice(), e, entity.CreateOptions{})
		require.NoError(t, err)
		id := created.Entity.ID
		require.Equal(t, 1, countFinish())
		_, linked := w.get(id).Properties["basecamp"].(map[string]any)
		require.True(t, linked, "the todo was not linked before the completion failed")

		// The job queue's retry of the failed push.
		require.NoError(t, w.run(w.get(id)))
		require.Equal(t, 1, countFinish(), "the retry created a second to-do")
		posts := 0
		for _, c := range stub.takeCalls() {
			if c == fmt.Sprintf("POST /%s/todolists/%s/todos.json", bcAccount, bcList) {
				posts++
			}
		}
		require.Equal(t, 1, posts)

		// The pull merges with no base and reports the completion that did
		// not reach Basecamp.
		w.settle()
		require.Contains(t, w.get(id).Properties["sync_conflict"], "done")
	})

	t.Run("edits on both sides are recorded, not pushed", func(t *testing.T) {
		stub.edit(plan, func(td *bcTodo) { td.Content = "Plan (theirs)" })
		before := w.base(planID)
		_, err := w.svc.EntityManager().PatchEntity(w.alice(), planID,
			entity.Patch{Properties: map[string]any{"title": "Plan (ours)"}})
		require.NoError(t, err)
		require.Equal(t, "Plan (theirs)", stub.todo(plan).Content)
		require.Contains(t, w.get(planID).Properties["sync_conflict"], "title")
		require.Equal(t, before, w.base(planID), "the base moved over a conflict")

		// Resolved by agreeing: the next pull clears the note.
		_, err = w.svc.EntityManager().PatchEntity(w.alice(), planID,
			entity.Patch{Properties: map[string]any{"title": "Plan (theirs)"}})
		require.NoError(t, err)
		w.settle()
		require.Nil(t, w.get(planID).Properties["sync_conflict"])
	})

	t.Run("429 ends the pull without moving a tag", func(t *testing.T) {
		stub.edit(write, func(td *bcTodo) { td.Content = "Write more" })
		id := w.byRef(write).ID
		before := w.base(id)
		stub.limited = 1
		w.pull()
		require.Equal(t, "Write", w.get(id).Properties["title"])
		require.Equal(t, before, w.base(id))

		// A push answered 429 fails, so the job queue retries it.
		stub.limited = 1
		_, err := w.svc.EntityManager().PatchEntity(w.alice(), id,
			entity.Patch{Properties: map[string]any{"due": "2026-12-01"}})
		require.NoError(t, err)
		stub.limited = 1
		require.ErrorContains(t, w.run(w.get(id)), "rate limited")
		w.settle()
		require.Equal(t, "Write more", w.get(id).Properties["title"])
		require.Equal(t, "2026-12-01", *stub.todo(write).DueOn)
	})

	t.Run("401 invalidates and retries once with a rotated token", func(t *testing.T) {
		n := stub.refreshes()
		stub.revoke()
		stub.edit(ship, func(td *bcTodo) { td.Content = "Shipped" })
		w.pull()
		require.Equal(t, n+1, stub.refreshes())
		require.Equal(t, "Shipped", w.byRef(ship).Properties["title"])
	})

	t.Run("a vanished to-do is recorded", func(t *testing.T) {
		stub.remove(ship)
		w.pull()
		require.Contains(t, w.byRef(ship).Properties["sync_conflict"], "vanished")
	})
}
