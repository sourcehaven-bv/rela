package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// fakeActions records the calls the action tools make and answers with resp,
// or with err when set.
type fakeActions struct {
	err  error
	resp *script.ActionResponse

	address string
	id      string
	in      script.ActionRunInput
	calls   []string
	// user is the principal the last run ran as.
	user string
}

func (f *fakeActions) ListActions(_ context.Context, address string) ([]script.ActionInfo, error) {
	f.calls, f.address = append(f.calls, "list"), address
	if f.err != nil {
		return nil, f.err
	}
	return []script.ActionInfo{{
		ID: "regenerate", Label: "Regenerate", Confirm: true, ConfirmText: "Sure?", EntityBound: true,
	}}, nil
}

func (f *fakeActions) RunAction(
	ctx context.Context, id string, in script.ActionRunInput,
) (*script.ActionResponse, error) {
	f.calls, f.id, f.in = append(f.calls, "run"), id, in
	f.user = principal.From(ctx).User
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newActionServer(t *testing.T, fake *fakeActions) *Server {
	t.Helper()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	if fake != nil {
		deps.Commands.Actions = fake
	}
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

var actionTools = []string{"list_actions", "run_action"}

func TestActionTools_Registration(t *testing.T) {
	t.Parallel()

	t.Run("stdio: none", func(t *testing.T) {
		t.Parallel()
		names := listedTools(t, newActionServer(t, nil))
		for _, name := range actionTools {
			if slices.Contains(names, name) {
				t.Errorf("%s registered without an action runner", name)
			}
		}
	})
	t.Run("remote: both", func(t *testing.T) {
		t.Parallel()
		names := listedTools(t, newActionServer(t, &fakeActions{}))
		for _, name := range actionTools {
			if !slices.Contains(names, name) {
				t.Errorf("%s not registered", name)
			}
		}
	})
}

func TestActionTools_List(t *testing.T) {
	t.Parallel()
	fake := &fakeActions{}
	srv := newActionServer(t, fake)

	text, isErr := callTool(t, srv, "list_actions", `{}`)
	if isErr {
		t.Fatal(text)
	}
	if fake.address != "" {
		t.Errorf("address = %q, want empty", fake.address)
	}

	text, isErr = callTool(t, srv, "list_actions", `{"id":" DOC-1@concept "}`)
	if isErr {
		t.Fatal(text)
	}
	if fake.address != "DOC-1@concept" {
		t.Errorf("address = %q", fake.address)
	}
	var got struct {
		Actions []map[string]any `json:"actions"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"id": "regenerate", "label": "Regenerate", "confirm": true, "confirm_text": "Sure?", "entity_bound": true,
	}
	if len(got.Actions) != 1 || !mapsEqual(got.Actions[0], want) {
		t.Errorf("actions = %v, want [%v]", got.Actions, want)
	}
}

func mapsEqual(a, b map[string]any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return bytes.Equal(ja, jb)
}

func TestActionTools_Run(t *testing.T) {
	t.Parallel()

	t.Run("passes arguments", func(t *testing.T) {
		t.Parallel()
		fake := &fakeActions{resp: &script.ActionResponse{Message: "Done", MessageType: "success", Redirect: "/x"}}
		text, isErr := callTool(t, newActionServer(t, fake), "run_action",
			`{"action":"regenerate","id":" DOC-1@concept ","request":{"host":"web1"},"confirm":true}`)
		if isErr {
			t.Fatal(text)
		}
		if fake.id != "regenerate" || fake.in.Address != "DOC-1@concept" || !fake.in.Confirmed ||
			fake.in.Body["host"] != "web1" {

			t.Errorf("run(%q, %+v)", fake.id, fake.in)
		}
		if fake.user != "tester" {
			t.Errorf("ran as %q", fake.user)
		}
		if want := `{"message":"Done","message_type":"success","redirect":"/x"}`; compactJSON(t, text) != want {
			t.Errorf("result = %s, want %s", text, want)
		}
	})
	t.Run("omitted arguments", func(t *testing.T) {
		t.Parallel()
		fake := &fakeActions{}
		text, isErr := callTool(t, newActionServer(t, fake), "run_action", `{"action":"ping"}`)
		if isErr {
			t.Fatal(text)
		}
		if fake.in.Address != "" || fake.in.Body != nil || fake.in.Confirmed {
			t.Errorf("input = %+v, want zero", fake.in)
		}
		if compactJSON(t, text) != `{}` {
			t.Errorf("result = %s, want {}", text)
		}
	})
	t.Run("rich response", func(t *testing.T) {
		t.Parallel()
		fake := &fakeActions{resp: &script.ActionResponse{Status: 202, Body: "ok", ContentType: "text/plain"}}
		text, isErr := callTool(t, newActionServer(t, fake), "run_action", `{"action":"alert"}`)
		if isErr {
			t.Fatal(text)
		}
		if want := `{"body":"ok","content_type":"text/plain","status":202}`; compactJSON(t, text) != want {
			t.Errorf("result = %s, want %s", text, want)
		}
	})
	t.Run("rich error status is an error result", func(t *testing.T) {
		t.Parallel()
		fake := &fakeActions{resp: &script.ActionResponse{Status: 409, Body: "busy", ContentType: "text/plain"}}
		text, isErr := callTool(t, newActionServer(t, fake), "run_action", `{"action":"alert"}`)
		if !isErr || !strings.Contains(text, "busy") {
			t.Errorf("result = %q (error %v), want an error result carrying the body", text, isErr)
		}
	})
	t.Run("action id passed as given", func(t *testing.T) {
		t.Parallel()
		fake := &fakeActions{}
		callTool(t, newActionServer(t, fake), "run_action", `{"action":" ping "}`)
		if fake.id != " ping " {
			t.Errorf("id = %q, want it untrimmed so the runner refuses it", fake.id)
		}
	})
	t.Run("refusal carries the code", func(t *testing.T) {
		t.Parallel()
		fake := &fakeActions{err: &script.ActionError{
			Code: "script_error", Title: "Script error", Detail: "actions/boom.lua:2: kaboom", CorrelationID: "abc123",
		}}
		text, isErr := callTool(t, newActionServer(t, fake), "run_action", `{"action":"boom"}`)
		if !isErr {
			t.Fatalf("want an error result, got %s", text)
		}
		for _, part := range []string{"[script_error]", "kaboom", "abc123"} {
			if !strings.Contains(text, part) {
				t.Errorf("error %q lacks %q", text, part)
			}
		}
	})
	t.Run("other error", func(t *testing.T) {
		t.Parallel()
		fake := &fakeActions{err: errors.New("broken")}
		text, isErr := callTool(t, newActionServer(t, fake), "run_action", `{"action":"x"}`)
		if !isErr || !strings.Contains(text, "broken") {
			t.Errorf("result = %q (error %v)", text, isErr)
		}
	})
	t.Run("bad arguments", func(t *testing.T) {
		t.Parallel()
		for _, args := range []string{
			`{}`,
			`{"action":"x","request":"text"}`,
			`{"action":"x","confirm":"yes"}`,
			`{"action":"x","id":3}`,
		} {
			fake := &fakeActions{}
			text, isErr := callTool(t, newActionServer(t, fake), "run_action", args)
			if !isErr {
				t.Errorf("%s: want an error, got %s", args, text)
			}
			if len(fake.calls) != 0 {
				t.Errorf("%s: runner called", args)
			}
		}
	})
}

// A reload can drop the runner while the tools stay registered. The tools
// must then refuse instead of calling a nil interface.
func TestActionTools_AfterAReloadDropsActions(t *testing.T) {
	t.Parallel()
	fake := &fakeActions{}
	srv := newActionServer(t, fake)
	reloaded := srv.deps()
	reloaded.Commands.Actions = nil
	if err := srv.ReloadDeps(reloaded); err != nil {
		t.Fatalf("ReloadDeps: %v", err)
	}
	for name, args := range map[string]string{"list_actions": `{}`, "run_action": `{"action":"x"}`} {
		text, isErr := callTool(t, srv, name, args)
		if !isErr || !strings.Contains(text, "not available") {
			t.Errorf("%s after reload = %q (error %v)", name, text, isErr)
		}
	}
	if len(fake.calls) != 0 {
		t.Errorf("the dropped runner was called: %v", fake.calls)
	}
}

func compactJSON(t *testing.T, text string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
