//go:build !postgres && !memorybackend && !sqlite

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/configedit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

const cfgFlows = `triggers:
  open-new:
    description: Open new tasks
    on: {entity: task, created: true}
    outputs:
      - set: {properties: {state: open}}
`

// flowsDraft reads the Configure snapshot, edits its flows.yaml tree with
// fn and returns the draft.
func flowsDraft(t *testing.T, srv *server, fn func(trigger *configedit.Map)) configedit.Draft {
	t.Helper()
	rec := call(t, srv, "ada", http.MethodGet, dataentry.ConfigurePath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var snap struct {
		Version string          `json:"version"`
		Flows   json.RawMessage `json:"flows"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snap))
	tree, err := configedit.UnmarshalTree(snap.Flows)
	require.NoError(t, err)
	trs, _ := tree.(*configedit.Map).Get("triggers")
	tr, _ := trs.(*configedit.Map).Get("open-new")
	fn(tr.(*configedit.Map))
	raw, err := json.Marshal(tree)
	require.NoError(t, err)
	return configedit.Draft{BaseVersion: snap.Version, Flows: raw}
}

// TestConfigure_FlowsTrigger pins that a trigger edit the schema breaks is
// refused at preview and save with a problem naming flows.yaml, and that a
// valid trigger edit is written and served.
func TestConfigure_FlowsTrigger(t *testing.T) {
	srv, dir := configServer(t)
	path := filepath.Join(dir, "flows.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cfgFlows), 0o644))

	broken := flowsDraft(t, srv, func(tr *configedit.Map) {
		check := &configedit.Map{}
		check.Set("where", "title!=")
		check.Set("severity", "fatal")
		check.Set("message", "needs a title")
		out := &configedit.Map{}
		out.Set("check", check)
		outs, _ := tr.Get("outputs")
		tr.Set("outputs", append(outs.([]any), out))
	})
	for op, status := range map[string]int{"/preview": http.StatusOK, "/save": http.StatusUnprocessableEntity} {
		rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+op, broken)
		require.Equal(t, status, rec.Code, rec.Body.String())
		var res configedit.Result
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.False(t, res.Saved, op)
		require.NotEmpty(t, res.Problems, op)
		require.Contains(t, res.Problems[0].Message, "flows.yaml", op)
	}
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, cfgFlows, string(got))

	first := srv.cur.Load()
	valid := flowsDraft(t, srv, func(tr *configedit.Map) { tr.Set("description", "Open every new task") })
	rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+"/save", valid)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(got), "description: Open every new task")
	require.NotSame(t, first, srv.cur.Load())
}

// TestConfigure_FlowsTriggerACL pins that a trigger edit is judged against
// acl.yaml for the principal who makes it: a read the policy hides and a
// write the editor may not make are refused, and other edits are saved.
func TestConfigure_FlowsTriggerACL(t *testing.T) {
	const restricted = `roles:
  admin:
    read: [task]
    create: [task]
    update: [task]
    permissions: [config:edit]
  viewer:
    read: [task]
    visible:
      task: [{field: title}, {field: state}]
assignments:
  ada: admin
  bob: viewer
`
	setTo := func(prop, value string) func(*configedit.Map) {
		return func(tr *configedit.Map) {
			props := &configedit.Map{}
			props.Set(prop, value)
			set := &configedit.Map{}
			set.Set("properties", props)
			out := &configedit.Map{}
			out.Set("set", set)
			tr.Set("outputs", []any{out})
		}
	}
	tests := []struct {
		name string
		acl  string
		edit func(*configedit.Map)
		want string // a refusal message part; empty when the save passes
	}{
		{"a write the editor may not make", cfgACL, setTo("state", "done"), "may not change yourself"},
		{"a read the policy hides", restricted, setTo("title", "{{new.notes}}"), "reads task.notes"},
		{"an allowed write", restricted, setTo("state", "done"), ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, dir := configServerACL(t, tc.acl)
			path := filepath.Join(dir, "flows.yaml")
			require.NoError(t, os.WriteFile(path, []byte(cfgFlows), 0o644))
			d := flowsDraft(t, srv, tc.edit)
			rec := call(t, srv, "ada", http.MethodPost, dataentry.ConfigurePath+"/save", d)
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			if tc.want == "" {
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.Contains(t, string(got), "state: done")
				return
			}
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			var res configedit.Result
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
			require.NotEmpty(t, res.Problems)
			require.Equal(t, "acl_trigger", res.Problems[0].Code)
			require.Contains(t, res.Problems[0].Message, tc.want)
			require.Equal(t, cfgFlows, string(got))
		})
	}
}
