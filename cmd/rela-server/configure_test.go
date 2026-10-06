//go:build !postgres && !memorybackend && !sqlite

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/configedit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

const cfgSchema = `version: "1.0"
types:
  status:
    values: [open, doing, done]
entities:
  task:
    label: Task
    id_prefix: TSK
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      notes:
        type: string
      state:
        type: status
`

const cfgDataEntry = `app:
  name: Tasks
`

const cfgACL = `roles:
  admin:
    read: [task]
    write: [task]
    permissions: [config:edit]
  viewer:
    read: [task]
assignments:
  ada: admin
  bob: viewer
`

const cfgTask = `---
id: TSK-001
type: task
title: First
notes: hello
state: doing
---
`

// newConfigProject writes a project the Configure space can edit and
// returns its directory.
func newConfigProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"schema.yaml":               cfgSchema,
		"data-entry.yaml":           cfgDataEntry,
		"acl.yaml":                  cfgACL,
		"entities/tasks/TSK-001.md": cfgTask,
	} {
		p := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	return dir
}

// configServer starts a server over a fresh project with --config-editing.
func configServer(t *testing.T) (srv *server, dir string) {
	t.Helper()
	dir = newConfigProject(t)
	f := &serverFlags{
		projectDir: dir, bind: "127.0.0.1", port: "8080",
		principalHeader: "X-User", jwtHeader: "X-Auth-Assertion", configEditing: true,
	}
	svc, err := discoverProject(f)
	require.NoError(t, err)
	srv, err = newServer(f, svc)
	require.NoError(t, err)
	t.Cleanup(func() {
		g := srv.cur.Load()
		waitRetired(t, srv)
		g.haltScheduler()
		g.app.StopWatching()
		_ = g.svc.Close()
	})
	return srv, dir
}

func waitRetired(t *testing.T, srv *server) {
	t.Helper()
	srv.mu.Lock()
	done := srv.retiring
	srv.mu.Unlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(drainLimit + 5*time.Second):
		t.Fatal("the replaced generation was not retired")
	}
}

func call(t *testing.T, srv *server, user, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Host = srv.addr
	req.Header.Set("Origin", "http://"+srv.addr)
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-User", user)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// snapshotSchema reads the Configure snapshot and returns its version and
// schema tree.
func snapshotSchema(t *testing.T, srv *server) (string, *configedit.Map) {
	t.Helper()
	rec := call(t, srv, "ada", http.MethodGet, dataentry.ConfigurePath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var snap struct {
		Version string          `json:"version"`
		Schema  json.RawMessage `json:"schema"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snap))
	tree, err := configedit.UnmarshalTree(snap.Schema)
	require.NoError(t, err)
	return snap.Version, tree.(*configedit.Map)
}

func taskProperties(t *testing.T, schema *configedit.Map) *configedit.Map {
	t.Helper()
	ents, _ := schema.Get("entities")
	task, _ := ents.(*configedit.Map).Get("task")
	props, _ := task.(*configedit.Map).Get("properties")
	return props.(*configedit.Map)
}

func TestConfigure_Gate(t *testing.T) {
	srv, _ := configServer(t)
	// No user: the ACL middleware refuses the unresolved principal before
	// the Configure gate runs.
	for user, want := range map[string]int{
		"bob": http.StatusForbidden, "": http.StatusInternalServerError, "ada": http.StatusOK,
	} {
		rec := call(t, srv, user, http.MethodGet, dataentry.ConfigurePath, nil)
		require.Equal(t, want, rec.Code, "user %q: %s", user, rec.Body.String())
	}
}

// TestConfigure_RenameSwitchesServer saves a property rename through the
// real wiring: the migration runs, the server switches to the new schema
// without a restart, and the old generation is retired and closed.
func TestConfigure_RenameSwitchesServer(t *testing.T) {
	srv, dir := configServer(t)
	first := srv.cur.Load()

	version, schema := snapshotSchema(t, srv)
	props := taskProperties(t, schema)
	for i, k := range props.Keys {
		if k == "notes" {
			props.Keys[i] = "remarks"
		}
	}
	raw, err := json.Marshal(schema)
	require.NoError(t, err)
	draft := configedit.Draft{
		BaseVersion: version, Schema: raw,
		Renames: []configedit.Rename{{EntityType: "task", From: "notes", To: "remarks"}},
	}
	rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+"/save", draft)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var res configedit.Result
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	require.True(t, res.Saved)
	require.False(t, res.Incomplete)
	require.NotNil(t, res.Migration)

	next := srv.cur.Load()
	require.NotSame(t, first, next, "the save switched generations")
	require.True(t, first.retired.Load())
	_, ok := next.svc.Meta().Entities["task"].Properties["remarks"]
	require.True(t, ok, "the new generation serves the new schema")

	e, err := next.svc.Store().GetEntity(context.Background(), "TSK-001")
	require.NoError(t, err)
	require.Equal(t, "hello", e.Properties["remarks"])
	require.NotContains(t, e.Properties, "notes")

	applied, err := os.ReadFile(filepath.Join(dir, "migrations", "applied.json"))
	require.NoError(t, err)
	require.Contains(t, string(applied), res.Migration.File)

	// The service outlives the rebuild: a second save goes through the new
	// generation.
	version, schema = snapshotSchema(t, srv)
	taskProperties(t, schema).Set("due", mapOf("type", "date"))
	raw, err = json.Marshal(schema)
	require.NoError(t, err)
	rec = call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+"/save",
		configedit.Draft{BaseVersion: version, Schema: raw})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, next.retired.Load())

	waitRetired(t, srv)
	require.Zero(t, first.inflight.Load())
}

func mapOf(kv ...any) *configedit.Map {
	m := &configedit.Map{}
	for i := 0; i < len(kv); i += 2 {
		m.Set(kv[i].(string), kv[i+1])
	}
	return m
}

func TestValidateConfigEditing(t *testing.T) {
	withACL := newConfigProject(t)
	noACL := newConfigProject(t)
	require.NoError(t, os.Remove(filepath.Join(noACL, "acl.yaml")))

	cases := []struct {
		name    string
		dir     string
		flags   serverFlags
		envUser string
		wantErr string
	}{
		{name: "header identity", dir: withACL, flags: serverFlags{principalHeader: "X-User"}},
		{name: "env identity", dir: withACL, envUser: "ada"},
		{name: "read-only", dir: withACL, flags: serverFlags{principalHeader: "X-User", readOnly: true},
			wantErr: "--read-only"},
		{name: "no acl.yaml", dir: noACL, flags: serverFlags{principalHeader: "X-User"}, wantErr: "acl.yaml"},
		{name: "no identity", dir: withACL, wantErr: "identity source"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.flags
			f.projectDir = tc.dir
			svc, err := discoverProject(&f)
			require.NoError(t, err)
			t.Cleanup(func() { _ = svc.Close() })
			err = validateConfigEditing(&f, svc, identityHeader, tc.envUser)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestPause_HoldsRecordWrites pins the write freeze a save puts on the
// serving generation: writes get 503, reads and the Configure API pass, and
// resume lifts it.
func TestPause_HoldsRecordWrites(t *testing.T) {
	srv, _ := configServer(t)
	g := srv.cur.Load()
	write := func() int {
		return call(t, srv, "ada", http.MethodPost, "/api/v1/entities/task",
			map[string]any{"properties": map[string]any{"title": "new"}}).Code
	}

	resume, err := srv.pause(context.Background(), g)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, write())
	require.Equal(t, http.StatusOK, call(t, srv, "ada", http.MethodGet, dataentry.ConfigurePath, nil).Code)
	resume()
	require.NotEqual(t, http.StatusServiceUnavailable, write())

	// A write still running when the deadline passes refuses the pause, and
	// the refusal leaves nothing frozen.
	g.writes.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = srv.pause(ctx, g)
	g.writes.Add(-1)
	require.Error(t, err)
	require.False(t, g.frozen.Load())
}

// TestConfigure_SwapUnderLoad saves while reads run, and pins that every
// request is answered by one generation or the other, never dropped.
func TestConfigure_SwapUnderLoad(t *testing.T) {
	srv, _ := configServer(t)
	stop := make(chan struct{})
	bad := make(chan int, 1)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				req := httptest.NewRequest(http.MethodGet, "/api/v1/_me", http.NoBody)
				req.Host = srv.addr
				req.Header.Set("Origin", "http://"+srv.addr)
				req.Header.Set("X-User", "ada")
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					select {
					case bad <- rec.Code:
					default:
					}
				}
			}
		})
	}

	version, schema := snapshotSchema(t, srv)
	taskProperties(t, schema).Set("due", mapOf("type", "date"))
	raw, err := json.Marshal(schema)
	require.NoError(t, err)
	rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+"/save",
		configedit.Draft{BaseVersion: version, Schema: raw})
	close(stop)
	wg.Wait()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	select {
	case code := <-bad:
		t.Fatalf("a request during the swap got %d", code)
	default:
	}
}

// TestConfigure_RemoveUsedOptionMigrates removes a choice option a record
// uses, maps it to another, and checks the whole chain on a real server: the
// written migration, the applied list, the record's value and the audit log.
func TestConfigure_RemoveUsedOptionMigrates(t *testing.T) {
	srv, dir := configServer(t)

	version, schema := snapshotSchema(t, srv)
	types, _ := schema.Get("types")
	st, _ := types.(*configedit.Map).Get("status")
	st.(*configedit.Map).Set("values", []any{"open", "done"})
	raw, err := json.Marshal(schema)
	require.NoError(t, err)
	draft := configedit.Draft{
		BaseVersion: version, Schema: raw, MigrationTitle: "Fold doing into done",
		Values: []configedit.ValueMapping{{EntityType: "task", Property: "state", From: "doing", To: "done"}},
	}
	rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+"/save", draft)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var res configedit.Result
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	require.True(t, res.Saved)
	require.False(t, res.Incomplete)
	require.NotNil(t, res.Migration)

	written, err := os.ReadFile(filepath.Join(dir, "migrations", res.Migration.File))
	require.NoError(t, err)
	require.Contains(t, string(written), "map_values")
	applied, err := os.ReadFile(filepath.Join(dir, "migrations", "applied.json"))
	require.NoError(t, err)
	require.Contains(t, string(applied), res.Migration.File)

	e, err := srv.cur.Load().svc.Store().GetEntity(context.Background(), "TSK-001")
	require.NoError(t, err)
	require.Equal(t, "done", e.Properties["state"])

	require.Contains(t, auditLog(t, dir), `"data-migration"`)
}

// TestConfigure_NewTypeSurvivesRestart adds an entity type with a plural,
// creates a record of it, and checks that the record sits in its own folder
// and is found by a server started afresh on the same project.
func TestConfigure_NewTypeSurvivesRestart(t *testing.T) {
	srv, dir := configServer(t)
	// acl.yaml is not edited in the app; the save's rebuild reads it from disk.
	acl := strings.Replace(strings.ReplaceAll(cfgACL, "[task]", "[task, project]"),
		"    write:", "    create: [project]\n    write:", 1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acl.yaml"), []byte(acl), 0o644))

	version, schema := snapshotSchema(t, srv)
	entities, _ := schema.Get("entities")
	entities.(*configedit.Map).Set("project", mapOf(
		"label", "Project", "plural", "projects", "id_prefix", "PRJ", "id_type", "sequential",
		"properties", mapOf("title", mapOf("type", "string", "required", true)),
	))
	raw, err := json.Marshal(schema)
	require.NoError(t, err)
	rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+"/save",
		configedit.Draft{BaseVersion: version, Schema: raw})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = call(t, srv, "ada", http.MethodPost, "/api/v1/projects", map[string]any{
		"properties": map[string]any{"title": "Launch"},
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	matches, err := filepath.Glob(filepath.Join(dir, "entities", "projects", "PRJ-*.md"))
	require.NoError(t, err)
	require.Len(t, matches, 1, "the record is stored in the type's own folder")

	// The replaced generation closes after the record was written; what it
	// leaves behind must not hide the record from the next start.
	waitRetired(t, srv)
	f := &serverFlags{projectDir: dir, bind: "127.0.0.1", port: "8080", principalHeader: "X-User"}
	svc, err := discoverProject(f)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	id := strings.TrimSuffix(filepath.Base(matches[0]), ".md")
	e, err := svc.Store().GetEntity(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "Launch", e.Properties["title"])
}

// auditLog returns everything in the project's audit log directory.
func auditLog(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	files, err := filepath.Glob(filepath.Join(dir, ".rela", "audit", "*"))
	require.NoError(t, err)
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		b.Write(data)
	}
	return b.String()
}

// TestConfigure_PreviewCompilesExpressions pins that a malformed expression
// is reported at preview, where the screen can show it, and that saving the
// same draft writes nothing.
func TestConfigure_PreviewCompilesExpressions(t *testing.T) {
	srv, dir := configServer(t)
	before, err := os.ReadFile(filepath.Join(dir, "schema.yaml"))
	require.NoError(t, err)

	version, schema := snapshotSchema(t, srv)
	schema.Set("validations", []any{mapOf(
		"name", "needs-title", "entity_type", "task", "then_condition", "entity.title ==",
	)})
	raw, err := json.Marshal(schema)
	require.NoError(t, err)
	draft := configedit.Draft{BaseVersion: version, Schema: raw}

	// A preview answers 200 with the problems; a refused save answers 422.
	for op, status := range map[string]int{"/preview": http.StatusOK, "/save": http.StatusUnprocessableEntity} {
		rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+op, draft)
		require.Equal(t, status, rec.Code, rec.Body.String())
		var res configedit.Result
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.False(t, res.Saved, op)
		require.Len(t, res.Problems, 1, op)
		require.Contains(t, res.Problems[0].Message, `validation "needs-title"`, op)
	}
	after, err := os.ReadFile(filepath.Join(dir, "schema.yaml"))
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}
