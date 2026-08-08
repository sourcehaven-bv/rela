package dataentry

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// --- resolveCommands ---

func TestResolveCommands(t *testing.T) {
	app, _ := testAppInstance()
	app.Cfg().Views = map[string]ViewConfig{
		"ticket_detail": {Title: "Ticket Detail", Entry: ViewEntry{Type: "ticket"}},
	}
	app.Cfg().Commands = map[string]CommandConfig{
		"entity-cmd": {
			Label:   "Entity Cmd",
			Script:  "echo hi",
			Context: "entity",
			AvailableOn: &CommandScope{
				EntityTypes: []string{"ticket"},
			},
		},
		"view-cmd": {
			Label:   "View Cmd",
			Script:  "echo hi",
			Context: "view",
			AvailableOn: &CommandScope{
				Views: []string{"ticket_detail"},
			},
		},
		"list-cmd": {
			Label:   "List Cmd",
			Script:  "echo hi",
			Context: "list",
			AvailableOn: &CommandScope{
				Lists: []string{"tickets"},
			},
		},
		"global-cmd": {
			Label:   "Global Cmd",
			Script:  "echo hi",
			Context: "global",
			AvailableOn: &CommandScope{
				Dashboard: true,
			},
		},
		"unscoped-entity": {
			Label:   "Unscoped",
			Script:  "echo hi",
			Context: "entity",
		},
	}

	t.Run("entity page shows entity commands", func(t *testing.T) {
		cmds := app.commands.resolveCommands(context.Background(), "entity", "", "ticket")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "entity-cmd")
		assertContains(t, ids, "unscoped-entity")
		assertNotContains(t, ids, "view-cmd")
		assertNotContains(t, ids, "list-cmd")
		assertNotContains(t, ids, "global-cmd")
	})

	t.Run("entity page for non-matching type", func(t *testing.T) {
		cmds := app.commands.resolveCommands(context.Background(), "entity", "", "component")
		ids := cmdIDs(cmds)
		// unscoped entity command still shows (context matches)
		assertContains(t, ids, "unscoped-entity")
		// scoped to ticket only
		assertNotContains(t, ids, "entity-cmd")
	})

	t.Run("view page shows entity and view commands", func(t *testing.T) {
		cmds := app.commands.resolveCommands(context.Background(), "view", "ticket_detail", "ticket")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "entity-cmd")
		assertContains(t, ids, "view-cmd")
		assertContains(t, ids, "unscoped-entity")
		assertNotContains(t, ids, "list-cmd")
		assertNotContains(t, ids, "global-cmd")
	})

	t.Run("list page shows list commands", func(t *testing.T) {
		cmds := app.commands.resolveCommands(context.Background(), "list", "tickets", "ticket")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "list-cmd")
		assertNotContains(t, ids, "entity-cmd")
		assertNotContains(t, ids, "view-cmd")
	})

	t.Run("dashboard shows global commands", func(t *testing.T) {
		cmds := app.commands.resolveCommands(context.Background(), "dashboard", "", "")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "global-cmd")
		assertNotContains(t, ids, "entity-cmd")
	})

	t.Run("empty commands returns nil", func(t *testing.T) {
		app2, _ := testAppInstance()
		cmds := app2.commands.resolveCommands(context.Background(), "entity", "", "ticket")
		if cmds != nil {
			t.Errorf("expected nil, got %v", cmds)
		}
	})

	t.Run("auto_open propagated to resolved command", func(t *testing.T) {
		app2, _ := testAppInstance()
		trueVal := true
		app2.Cfg().Commands = map[string]CommandConfig{
			"auto-cmd": {
				Label:    "Auto",
				Script:   "echo hi",
				Context:  "entity",
				AutoOpen: &trueVal,
			},
			"normal-cmd": {
				Label:   "Normal",
				Script:  "echo hi",
				Context: "entity",
			},
		}
		cmds := app2.commands.resolveCommands(context.Background(), "entity", "", "ticket")
		for _, c := range cmds {
			if c.ID == "auto-cmd" {
				if c.AutoOpen == nil || !*c.AutoOpen {
					t.Error("expected auto-cmd to have AutoOpen=true")
				}
			}
			if c.ID == "normal-cmd" {
				if c.AutoOpen != nil {
					t.Error("expected normal-cmd to have AutoOpen=nil")
				}
			}
		}
	})

	t.Run("deterministic order", func(t *testing.T) {
		cmds := app.commands.resolveCommands(context.Background(), "view", "ticket_detail", "ticket")
		if len(cmds) < 2 {
			t.Skip("need at least 2 commands")
		}
		// Run multiple times and check order is stable
		for range 5 {
			cmds2 := app.commands.resolveCommands(context.Background(), "view", "ticket_detail", "ticket")
			for j := range cmds {
				if cmds[j].ID != cmds2[j].ID {
					t.Fatalf("order not deterministic: %v vs %v", cmdIDs(cmds), cmdIDs(cmds2))
				}
			}
		}
	})
}

// --- parseCommandOutput ---

func TestParseCommandOutput(t *testing.T) {
	t.Run("structured message", func(t *testing.T) {
		msg := parseCommandOutput(`::rela::{"type":"message","text":"hello"}`)
		if msg.Type != "message" || msg.Text != "hello" {
			t.Errorf("unexpected: %+v", msg)
		}
	})

	t.Run("file message", func(t *testing.T) {
		msg := parseCommandOutput(`::rela::{"type":"file","path":"/tmp/report.pdf","label":"Report","action":"open"}`)
		if msg.Type != "file" || msg.Path != "/tmp/report.pdf" || msg.Action != "open" {
			t.Errorf("unexpected: %+v", msg)
		}
	})

	t.Run("entity message", func(t *testing.T) {
		msg := parseCommandOutput(`::rela::{"type":"entity","id":"TKT-001","entity_type":"ticket","action":"updated"}`)
		if msg.Type != "entity" || msg.ID != "TKT-001" || msg.EntityType != "ticket" {
			t.Errorf("unexpected: %+v", msg)
		}
	})

	t.Run("error message", func(t *testing.T) {
		msg := parseCommandOutput(`::rela::{"type":"error","text":"something failed"}`)
		if msg.Type != "error" || msg.Text != "something failed" {
			t.Errorf("unexpected: %+v", msg)
		}
	})

	t.Run("group and endgroup", func(t *testing.T) {
		msg := parseCommandOutput(`::rela::{"type":"group","label":"Files"}`)
		if msg.Type != "group" || msg.Label != "Files" {
			t.Errorf("unexpected: %+v", msg)
		}
		msg2 := parseCommandOutput(`::rela::{"type":"endgroup"}`)
		if msg2.Type != "endgroup" {
			t.Errorf("unexpected: %+v", msg2)
		}
	})

	t.Run("open URL", func(t *testing.T) {
		msg := parseCommandOutput(`::rela::{"type":"open","url":"https://example.com"}`)
		if msg.Type != "open" || msg.URL != "https://example.com" {
			t.Errorf("unexpected: %+v", msg)
		}
	})

	t.Run("warning level", func(t *testing.T) {
		msg := parseCommandOutput(`::rela::{"type":"message","level":"warning","text":"watch out"}`)
		if msg.Level != "warning" {
			t.Errorf("unexpected level: %s", msg.Level)
		}
	})

	t.Run("raw log line", func(t *testing.T) {
		msg := parseCommandOutput("some raw output")
		if msg.Type != "log" || msg.Text != "some raw output" {
			t.Errorf("unexpected: %+v", msg)
		}
	})

	t.Run("malformed JSON falls back to log", func(t *testing.T) {
		msg := parseCommandOutput("::rela::{broken json")
		if msg.Type != "log" {
			t.Errorf("expected log type for malformed JSON, got: %s", msg.Type)
		}
	})

	t.Run("empty line", func(t *testing.T) {
		msg := parseCommandOutput("")
		if msg.Type != "log" {
			t.Errorf("expected log type for empty line, got: %s", msg.Type)
		}
	})

	t.Run("prefix only", func(t *testing.T) {
		msg := parseCommandOutput("::rela::")
		// Empty JSON after prefix — should fall back to log
		if msg.Type != "log" {
			t.Errorf("expected log for prefix-only, got: %s", msg.Type)
		}
	})
}

// --- Stdin JSON builders ---

func TestBuildEntityInput(t *testing.T) {
	app, entities := testAppInstance()
	bindRepo(app, "/test/project")
	seedRelation(app, entity.NewRelation(entities.ticket1.ID, "depends_on", entities.ticket2.ID))

	input := app.commands.buildEntityInput(context.Background(), entities.ticket1)

	if input.Context != "entity" {
		t.Errorf("expected entity context, got %s", input.Context)
	}
	if input.Entity.ID != entities.ticket1.ID {
		t.Errorf("expected %s, got %s", entities.ticket1.ID, input.Entity.ID)
	}
	if input.Project.Root != "/test/project" {
		t.Errorf("expected /test/project, got %s", input.Project.Root)
	}
	if len(input.Relations) == 0 {
		t.Error("expected relations to be populated")
	}

	// Verify it marshals to valid JSON
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded["context"] != "entity" {
		t.Error("JSON context field mismatch")
	}
}

func TestBuildListInput(t *testing.T) {
	app, _ := testAppInstance()
	bindRepo(app, "/test/project")
	entities := entitiesByType(app, "ticket")

	input := app.commands.buildListInput("tickets", entities)

	if input.Context != "list" {
		t.Errorf("expected list context, got %s", input.Context)
	}
	if input.ListID != "tickets" {
		t.Errorf("expected tickets, got %s", input.ListID)
	}
	if len(input.Entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(input.Entities))
	}
}

func TestBuildViewInput(t *testing.T) {
	app, _ := testAppInstance()
	bindRepo(app, "/test/project")
	seedRelation(app, entity.NewRelation("TKT-001", "belongs_to", "CMP-001"))

	view := ViewConfig{
		Title: "Test View",
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "belongs_to", CollectAs: "components"},
		},
	}
	vr, err := app.views.executeView(context.Background(), view, "TKT-001")
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}

	input := app.commands.buildViewInput(context.Background(), "test_view", vr)

	if input.Context != "view" {
		t.Errorf("expected view context, got %s", input.Context)
	}
	if input.ViewID != "test_view" {
		t.Errorf("expected test_view, got %s", input.ViewID)
	}
	if input.Entity.ID != "TKT-001" {
		t.Errorf("expected TKT-001, got %s", input.Entity.ID)
	}
	if len(input.Collections["components"]) == 0 {
		t.Error("expected components collection")
	}
	if len(input.Relations) == 0 {
		t.Error("expected relations between entities in view")
	}
}

func TestBuildGlobalInput(t *testing.T) {
	app, _ := testAppInstance()
	bindRepo(app, "/test/project")

	input := app.commands.buildGlobalInput()

	if input.Context != "global" {
		t.Errorf("expected global context, got %s", input.Context)
	}
	if input.Entity != nil {
		t.Error("expected no entity for global context")
	}
}

// --- buildCommandEnv ---

func TestBuildCommandEnv(t *testing.T) {
	app, entities := testAppInstance()
	bindRepo(app, "/test/project")

	cmd := CommandConfig{
		Script:  "echo hi",
		Context: "entity",
		Env:     map[string]string{"FORMAT": "pdf"},
	}
	input := app.commands.buildEntityInput(context.Background(), entities.ticket1)
	env := app.commands.buildCommandEnv(cmd, input)

	envMap := envToMap(env)
	if envMap["RELA_PROJECT_ROOT"] != "/test/project" {
		t.Errorf("expected project root, got %s", envMap["RELA_PROJECT_ROOT"])
	}
	if envMap["RELA_CONTEXT"] != "entity" {
		t.Errorf("expected entity context, got %s", envMap["RELA_CONTEXT"])
	}
	if envMap["RELA_ENTITY_ID"] != entities.ticket1.ID {
		t.Errorf("expected %s, got %s", entities.ticket1.ID, envMap["RELA_ENTITY_ID"])
	}
	if envMap["RELA_ENTITY_TYPE"] != "ticket" {
		t.Errorf("expected ticket, got %s", envMap["RELA_ENTITY_TYPE"])
	}
	if envMap["FORMAT"] != "pdf" {
		t.Errorf("expected custom env FORMAT=pdf, got %s", envMap["FORMAT"])
	}
}

func TestBuildCommandEnvListContext(t *testing.T) {
	app, _ := testAppInstance()
	bindRepo(app, "/test/project")

	cmd := CommandConfig{Script: "echo hi", Context: "list"}
	input := app.commands.buildListInput("tickets", nil)
	env := app.commands.buildCommandEnv(cmd, input)

	envMap := envToMap(env)
	if envMap["RELA_LIST_ID"] != "tickets" {
		t.Errorf("expected tickets, got %s", envMap["RELA_LIST_ID"])
	}
}

func TestBuildCommandEnvViewContext(t *testing.T) {
	app, entities := testAppInstance()
	bindRepo(app, "/test/project")

	cmd := CommandConfig{Script: "echo hi", Context: "view"}
	input := &commandInput{
		Context: "view",
		ViewID:  "ticket_detail",
		Entity:  entities.ticket1,
		Project: app.commands.projectInfo(),
	}
	env := app.commands.buildCommandEnv(cmd, input)

	envMap := envToMap(env)
	if envMap["RELA_VIEW_ID"] != "ticket_detail" {
		t.Errorf("expected ticket_detail, got %s", envMap["RELA_VIEW_ID"])
	}
	if envMap["RELA_ENTITY_ID"] != entities.ticket1.ID {
		t.Errorf("expected %s, got %s", entities.ticket1.ID, envMap["RELA_ENTITY_ID"])
	}
}

// --- Config validation ---

func TestValidateCommandConfig(t *testing.T) {
	meta := testMeta()
	emptyYAML := []byte(`version: "1.0"`)

	t.Run("valid command", func(t *testing.T) {
		cfg := &Config{
			Lists: map[string]List{"tickets": {EntityType: "ticket"}},
			Views: map[string]ViewConfig{"ticket_detail": {Entry: ViewEntry{Type: "ticket"}}},
			Commands: map[string]CommandConfig{
				"test": {
					Label:   "Test",
					Script:  "echo hi",
					Context: "entity",
					AvailableOn: &CommandScope{
						EntityTypes: []string{"ticket"},
						Views:       []string{"ticket_detail"},
						Lists:       []string{"tickets"},
					},
				},
			},
		}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
	})

	t.Run("missing label", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"bad": {Script: "echo", Context: "entity"},
		}}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if !hasErrorStr(err, "label") {
			t.Errorf("expected label error, got %v", err)
		}
	})

	t.Run("missing script", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"bad": {Label: "Test", Context: "entity"},
		}}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if !hasErrorStr(err, "script") {
			t.Errorf("expected script error, got %v", err)
		}
	})

	t.Run("invalid context", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"bad": {Label: "Test", Script: "echo", Context: "invalid"},
		}}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if !hasErrorStr(err, "invalid context") {
			t.Errorf("expected context error, got %v", err)
		}
	})

	t.Run("unknown view reference", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"bad": {
				Label: "Test", Script: "echo", Context: "view",
				AvailableOn: &CommandScope{Views: []string{"nonexistent"}},
			},
		}}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if !hasErrorStr(err, "unknown view") {
			t.Errorf("expected view error, got %v", err)
		}
	})

	t.Run("unknown list reference", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"bad": {
				Label: "Test", Script: "echo", Context: "list",
				AvailableOn: &CommandScope{Lists: []string{"nonexistent"}},
			},
		}}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if !hasErrorStr(err, "unknown list") {
			t.Errorf("expected list error, got %v", err)
		}
	})

	t.Run("unknown entity type reference", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"bad": {
				Label: "Test", Script: "echo", Context: "entity",
				AvailableOn: &CommandScope{EntityTypes: []string{"nonexistent"}},
			},
		}}
		err := ValidateConfig(emptyYAML, cfg, meta)
		if !hasErrorStr(err, "unknown entity type") {
			t.Errorf("expected entity type error, got %v", err)
		}
	})
}

func TestCommandConfigAutoOpenYAML(t *testing.T) {
	t.Run("true, false, and omitted", func(t *testing.T) {
		yamlData := []byte(`
commands:
  gen-pdf:
    label: Generate PDF
    script: echo hi
    context: entity
    auto_open: true
  no-auto:
    label: No Auto
    script: echo hi
    context: entity
    auto_open: false
  export:
    label: Export
    script: echo hi
    context: entity
`)
		var cfg Config
		if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		genPDF := cfg.Commands["gen-pdf"]
		if genPDF.AutoOpen == nil || !*genPDF.AutoOpen {
			t.Error("expected gen-pdf auto_open to be true")
		}
		noAuto := cfg.Commands["no-auto"]
		if noAuto.AutoOpen == nil {
			t.Fatal("expected no-auto auto_open to be non-nil")
		}
		if *noAuto.AutoOpen {
			t.Error("expected no-auto auto_open to be false")
		}
		export := cfg.Commands["export"]
		if export.AutoOpen != nil {
			t.Errorf("expected export auto_open to be nil, got %v", *export.AutoOpen)
		}
	})
}

// --- SSE Handler integration test ---

func TestHandleCommandExec(t *testing.T) {
	app := newHandlerTestApp(t)
	bindRepo(app, t.TempDir())
	app.Cfg().Commands = map[string]CommandConfig{
		"test-echo": {
			Label:   "Test Echo",
			Script:  `echo '::rela::{"type":"message","text":"hello from test"}' && echo 'raw log line'`,
			Context: "entity",
		},
	}

	t.Run("success stream", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/command/test-echo?entity_id=TKT-001&entity_type=ticket", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleCommandExec(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
			t.Errorf("expected text/event-stream, got %s", ct)
		}

		events := parseSSEEvents(t, w.Body)
		// Should have a message event, a log event, and a done event
		var hasMessage, hasLog, hasDone bool
		for _, ev := range events {
			switch ev.event {
			case "message":
				hasMessage = true
				if !strings.Contains(ev.data, "hello from test") {
					t.Errorf("unexpected message data: %s", ev.data)
				}
			case "log":
				hasLog = true
			case "done":
				hasDone = true
				if !strings.Contains(ev.data, `"success":true`) {
					t.Errorf("expected success=true, got: %s", ev.data)
				}
			}
		}
		if !hasMessage {
			t.Error("expected message event")
		}
		if !hasLog {
			t.Error("expected log event")
		}
		if !hasDone {
			t.Error("expected done event")
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/command/nonexistent", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleCommandExec(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("entity not found", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/command/test-echo?entity_id=NOPE", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleCommandExec(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/api/command/test-echo", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleCommandExec(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleCommandExecFailing(t *testing.T) {
	app := newHandlerTestApp(t)
	bindRepo(app, t.TempDir())
	app.Cfg().Commands = map[string]CommandConfig{
		"fail-cmd": {
			Label:   "Fail",
			Script:  "echo 'failing' >&2 && exit 1",
			Context: "entity",
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/api/command/fail-cmd?entity_id=TKT-001", http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandExec(w, r)

	events := parseSSEEvents(t, w.Body)
	var hasError, hasDone bool
	for _, ev := range events {
		if ev.event == "error" {
			hasError = true
		}
		if ev.event == "done" {
			hasDone = true
			if strings.Contains(ev.data, `"success":true`) {
				t.Error("expected success=false for failing command")
			}
		}
	}
	if !hasError {
		t.Error("expected error event for failing command")
	}
	if !hasDone {
		t.Error("expected done event for failing command")
	}
}

func TestHandleCommandExecGlobalContext(t *testing.T) {
	app := newHandlerTestApp(t)
	bindRepo(app, t.TempDir())
	app.Cfg().Commands = map[string]CommandConfig{
		"global-cmd": {
			Label:   "Global",
			Script:  `echo '::rela::{"type":"message","text":"global ok"}'`,
			Context: "global",
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/api/command/global-cmd", http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandExec(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	events := parseSSEEvents(t, w.Body)
	var hasMessage bool
	for _, ev := range events {
		if ev.event == "message" && strings.Contains(ev.data, "global ok") {
			hasMessage = true
		}
	}
	if !hasMessage {
		t.Error("expected global message")
	}
}

func TestHandleCommandExecListContext(t *testing.T) {
	app := newHandlerTestApp(t)
	bindRepo(app, t.TempDir())
	app.Cfg().Commands = map[string]CommandConfig{
		"list-cmd": {
			Label:   "List",
			Script:  `echo '::rela::{"type":"message","text":"list ok"}'`,
			Context: "list",
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/api/command/list-cmd?list_id=tickets", http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandExec(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCommandExecViewContext(t *testing.T) {
	app := newHandlerTestApp(t)
	bindRepo(app, t.TempDir())
	app.Cfg().Commands = map[string]CommandConfig{
		"view-cmd": {
			Label:   "View",
			Script:  `echo '::rela::{"type":"message","text":"view ok"}'`,
			Context: "view",
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/api/command/view-cmd?view_id=ticket_detail&entity_id=TKT-001", http.NoBody)
	w := httptest.NewRecorder()
	app.commands.handleCommandExec(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Cancel handler ---

func TestHandleCommandCancel(t *testing.T) {
	t.Run("no running command", func(t *testing.T) {
		app, _ := testAppInstance()
		r := httptest.NewRequest(http.MethodPost, "/api/command-cancel/nonexistent", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleCommandCancel(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		app, _ := testAppInstance()
		r := httptest.NewRequest(http.MethodGet, "/api/command-cancel/test", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleCommandCancel(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

// --- Open URL handler ---

func TestHandleOpenURL(t *testing.T) {
	app, _ := testAppInstance()

	t.Run("missing url", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/open-url", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleOpenURL(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/open-url?url=ftp://evil.com", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleOpenURL(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/open-url?url=https://example.com", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleOpenURL(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

// --- Open File handler ---

func TestHandleOpenFile(t *testing.T) {
	app, _ := testAppInstance()

	t.Run("missing path", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/open-file", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleOpenFile(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/open-file?path=/tmp/test", http.NoBody)
		w := httptest.NewRecorder()
		app.commands.handleOpenFile(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

// --- matchesPage ---

func TestMatchesPage(t *testing.T) {
	t.Run("nil scope matches context", func(t *testing.T) {
		cmd := CommandConfig{Context: "entity"}
		if !matchesPage(cmd, "entity", "", "ticket") {
			t.Error("expected entity command to match entity page")
		}
		if !matchesPage(cmd, "view", "", "ticket") {
			t.Error("expected entity command to match view page")
		}
		if matchesPage(cmd, "list", "", "") {
			t.Error("entity command should not match list page")
		}
	})

	t.Run("view context only matches view", func(t *testing.T) {
		cmd := CommandConfig{Context: "view"}
		if matchesPage(cmd, "entity", "", "ticket") {
			t.Error("view command should not match entity page")
		}
		if !matchesPage(cmd, "view", "", "ticket") {
			t.Error("view command should match view page")
		}
	})

	t.Run("global context matches dashboard", func(t *testing.T) {
		cmd := CommandConfig{Context: "global"}
		if !matchesPage(cmd, "dashboard", "", "") {
			t.Error("global command should match dashboard")
		}
		if matchesPage(cmd, "entity", "", "ticket") {
			t.Error("global command should not match entity page")
		}
	})
}

// --- contains ---

func TestContains(t *testing.T) {
	if !contains([]string{"a", "b", "c"}, "b") {
		t.Error("expected true")
	}
	if contains([]string{"a", "b", "c"}, "d") {
		t.Error("expected false")
	}
	if contains(nil, "a") {
		t.Error("expected false for nil")
	}
}

// --- Helpers ---

type testSSEEvent struct {
	event string
	data  string
}

func parseSSEEvents(t *testing.T, body io.Reader) []testSSEEvent {
	t.Helper()
	var events []testSSEEvent
	scanner := bufio.NewScanner(body)
	var current testSSEEvent
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			current.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		case line == "" && current.event != "":
			events = append(events, current)
			current = testSSEEvent{}
		}
	}
	if current.event != "" {
		events = append(events, current)
	}
	return events
}

func cmdIDs(cmds []ResolvedCommand) []string {
	ids := make([]string, len(cmds))
	for i, c := range cmds {
		ids[i] = c.ID
	}
	return ids
}

func assertContains(t *testing.T, ids []string, expected string) {
	t.Helper()
	if slices.Contains(ids, expected) {
		return
	}
	t.Errorf("expected %q in %v", expected, ids)
}

func assertNotContains(t *testing.T, ids []string, unexpected string) {
	t.Helper()
	if slices.Contains(ids, unexpected) {
		t.Errorf("did not expect %q in %v", unexpected, ids)
		return
	}
}

func hasErrorStr(err error, substring string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substring))
}

func envToMap(env []string) map[string]string {
	m := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

// --- openFileCommand / openURLCommand ---

func TestOpenFileCommand(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		action   string
		path     string
		wantArgs []string // expected args including argv[0]
	}{
		{"darwin open", "darwin", "open", "/tmp/x.pdf", []string{"open", "/tmp/x.pdf"}},
		{"darwin reveal", "darwin", "reveal", "/tmp/x.pdf", []string{"open", "-R", "/tmp/x.pdf"}},
		{"linux open", "linux", "open", "/tmp/x.pdf", []string{"xdg-open", "/tmp/x.pdf"}},
		{"linux reveal", "linux", "reveal", "/tmp/sub/x.pdf", []string{"xdg-open", "/tmp/sub"}},
		{"windows open", "windows", "open", `C:\x.pdf`, []string{"cmd", "/c", "start", "", `C:\x.pdf`}},
		{"windows reveal", "windows", "reveal", `C:\x.pdf`, []string{"explorer", "/select,", `C:\x.pdf`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := openFileCommand(tc.goos, tc.action, tc.path)
			if cmd == nil {
				t.Fatalf("openFileCommand returned nil for %s", tc.name)
			}
			if got := cmd.Args; !equalArgs(got, tc.wantArgs) {
				t.Errorf("args = %v, want %v", got, tc.wantArgs)
			}
		})
	}

	if got := openFileCommand("plan9", "open", "/tmp/x"); got != nil {
		t.Errorf("unsupported platform should return nil, got %v", got)
	}
}

func TestOpenURLCommand(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		url      string
		wantArgs []string
	}{
		{"darwin", "darwin", "https://example.com", []string{"open", "https://example.com"}},
		{"linux", "linux", "https://example.com", []string{"xdg-open", "https://example.com"}},
		{"windows", "windows", "https://example.com", []string{"cmd", "/c", "start", "", "https://example.com"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := openURLCommand(tc.goos, tc.url)
			if cmd == nil {
				t.Fatalf("openURLCommand returned nil for %s", tc.name)
			}
			if got := cmd.Args; !equalArgs(got, tc.wantArgs) {
				t.Errorf("args = %v, want %v", got, tc.wantArgs)
			}
		})
	}

	if got := openURLCommand("plan9", "https://example.com"); got != nil {
		t.Errorf("unsupported platform should return nil, got %v", got)
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// exec.Command resolves argv[0] to an absolute path if found on PATH;
	// compare by basename for the program name and exact for the rest.
	aProg := a[0]
	if idx := strings.LastIndexByte(aProg, '/'); idx >= 0 {
		aProg = aProg[idx+1:]
	}
	if idx := strings.LastIndexByte(aProg, '\\'); idx >= 0 {
		aProg = aProg[idx+1:]
	}
	if aProg != b[0] {
		return false
	}
	for i := 1; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Command authorization (TKT-MJ02AO, policy DEC-EIHQSU) ---

// TestCommandExecReadOnlyDenied is the canary for RR-CWWJGW. Command exec
// builds no acl.WriteRequest, so ReadOnlyACL's only method (AuthorizeWrite)
// is never consulted; and readGateFromContext hands back nopReadGate under
// BOTH NopACL and ReadOnlyACL, whose HoldsPermission returns true
// unconditionally (readgate.go). A guard written against the read gate alone
// therefore fails OPEN here. The permission is deliberately set AND would be
// granted, so the only thing that can produce a 403 is the read-only check
// itself.
func TestCommandExecReadOnlyDenied(t *testing.T) {
	for _, ctxName := range []string{"entity", "list", "view", "global"} {
		t.Run(ctxName, func(t *testing.T) {
			app := newHandlerTestApp(t)
			bindRepo(app, t.TempDir())
			app.acl = acl.ReadOnlyACL{}
			app.Cfg().Commands = map[string]CommandConfig{
				"cmd": {
					Label:      "Cmd",
					Script:     `echo '::rela::{"type":"message","text":"ran"}'`,
					Context:    ctxName,
					Permission: "command:cmd",
				},
			}

			r := httptest.NewRequest(http.MethodPost,
				"/api/command/cmd?entity_id=TKT-001&list_id=tickets&view_id=ticket_detail",
				http.NoBody)
			w := httptest.NewRecorder()
			app.commands.handleCommandExec(w, r)

			if w.Code != http.StatusForbidden {
				t.Fatalf("read-only must deny command exec: expected 403, got %d: %s",
					w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "command:cmd") {
				t.Errorf("403 body must not echo the permission name: %s", w.Body.String())
			}
		})
	}
}

// TestCommandExecNopACLFailsOpen pins the fail-open half of DEC-EIHQSU: with
// no policy configured, a command with no permission: runs exactly as before
// this ticket, in all four contexts including the deferred view context.
func TestCommandExecNopACLFailsOpen(t *testing.T) {
	cases := []struct{ name, ctxName, query string }{
		{"entity", "entity", "?entity_id=TKT-001"},
		{"list", "list", "?list_id=tickets"},
		{"view", "view", "?view_id=ticket_detail&entity_id=TKT-001"},
		{"global", "global", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			bindRepo(app, t.TempDir())
			app.acl = acl.NopACL{}
			app.Cfg().Commands = map[string]CommandConfig{
				"cmd": {
					Label:   "Cmd",
					Script:  `echo '::rela::{"type":"message","text":"ran"}'`,
					Context: tc.ctxName,
				},
			}

			r := httptest.NewRequest(http.MethodPost, "/api/command/cmd"+tc.query, http.NoBody)
			w := httptest.NewRecorder()
			app.commands.handleCommandExec(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("NopACL must fail open: expected 200, got %d: %s",
					w.Code, w.Body.String())
			}
		})
	}
}

// commandPolicyACL grants alice "command:allowed" and gives bob no
// permissions at all. Both hold a role, so a deny is attributable to the
// missing permission rather than to an unknown principal.
func commandPolicyACL(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	return mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"operator": {Read: []string{"ticket"}, Permissions: []string{"command:allowed"}},
			"viewer":   {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "operator", "bob": "viewer"},
	}, app.store)
}

// execCommandAs runs the exec handler with the ACL request + read gate
// attached to ctx, mirroring what attachACLRequest does in production (test
// handlers bypass the middleware).
func execCommandAs(
	ctx context.Context, t *testing.T, app *App, d *acl.Declarative, target string,
) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, target, http.NoBody).
		WithContext(gateCtxFor(ctx, t, d))
	w := httptest.NewRecorder()
	app.commands.handleCommandExec(w, r)
	return w
}

// TestCommandExecDeclarativeFailsClosed pins the fail-closed half of
// DEC-EIHQSU: once an acl.yaml is configured, a command runs only when its
// permission: is set AND held.
func TestCommandExecDeclarativeFailsClosed(t *testing.T) {
	// user is "alice" (holds command:allowed) or "bob" (holds nothing); the
	// principal ctx is built per-case rather than stored, since a struct field
	// holding a context.Context is a lint error (containedctx).
	cases := []struct {
		name       string
		ctxName    string
		query      string
		permission string
		user       string
		wantCode   int
	}{
		{"granted entity", "entity", "?entity_id=TKT-001", "command:allowed", "alice", http.StatusOK},
		{"granted list", "list", "?list_id=tickets", "command:allowed", "alice", http.StatusOK},
		{"granted global", "global", "", "command:allowed", "alice", http.StatusOK},

		{"not held entity", "entity", "?entity_id=TKT-001", "command:allowed", "bob", http.StatusForbidden},
		{"not held global", "global", "", "command:allowed", "bob", http.StatusForbidden},

		// No permission: under a configured policy ⇒ ungoverned ⇒ denied.
		{"no permission entity", "entity", "?entity_id=TKT-001", "", "alice", http.StatusForbidden},
		{"no permission list", "list", "?list_id=tickets", "", "alice", http.StatusForbidden},
		{"no permission global", "global", "", "", "alice", http.StatusForbidden},

		// View has no fine-grained control yet: a SET AND GRANTED permission
		// must still not open the gate (TKT-MJ02AO deferral).
		{
			"view denied despite granted permission", "view",
			"?view_id=ticket_detail&entity_id=TKT-001",
			"command:allowed", "alice", http.StatusForbidden,
		},
		{
			"view denied without permission", "view",
			"?view_id=ticket_detail&entity_id=TKT-001",
			"", "alice", http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			bindRepo(app, t.TempDir())
			d := commandPolicyACL(t, app)
			app.acl = d
			app.Cfg().Commands = map[string]CommandConfig{
				"cmd": {
					Label:      "Cmd",
					Script:     `echo '::rela::{"type":"message","text":"ran"}'`,
					Context:    tc.ctxName,
					Permission: tc.permission,
				},
			}

			w := execCommandAs(principalCtx(tc.user), t, app, d, "/api/command/cmd"+tc.query)

			if w.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, w.Code, w.Body.String())
			}
			deniedWithPermission := tc.wantCode == http.StatusForbidden && tc.permission != ""
			if deniedWithPermission && strings.Contains(w.Body.String(), tc.permission) {
				t.Errorf("403 body must not echo the permission name: %s", w.Body.String())
			}
		})
	}
}

// TestResolveCommandsFiltersUnauthorized pins that the button set the SPA
// renders matches what exec will actually allow. resolveCommands is
// presentation, but a mismatch means users see buttons that 403 on click.
func TestResolveCommandsFiltersUnauthorized(t *testing.T) {
	setup := func(t *testing.T) (*App, *acl.Declarative) {
		t.Helper()
		app := newHandlerTestApp(t)
		bindRepo(app, t.TempDir())
		d := commandPolicyACL(t, app)
		app.acl = d
		app.Cfg().Commands = map[string]CommandConfig{
			"allowed":       {Label: "A", Script: "echo hi", Context: "entity", Permission: "command:allowed"},
			"not-granted":   {Label: "B", Script: "echo hi", Context: "entity", Permission: "command:other"},
			"no-permission": {Label: "C", Script: "echo hi", Context: "entity"},
		}
		return app, d
	}

	t.Run("declarative shows only held permissions", func(t *testing.T) {
		app, d := setup(t)
		ids := cmdIDs(app.commands.resolveCommands(
			gateCtxFor(aliceCtx(), t, d), "entity", "", "ticket"))
		assertContains(t, ids, "allowed")
		assertNotContains(t, ids, "not-granted")
		assertNotContains(t, ids, "no-permission")
	})

	t.Run("principal without the permission sees none", func(t *testing.T) {
		app, d := setup(t)
		ids := cmdIDs(app.commands.resolveCommands(
			gateCtxFor(bobCtx(), t, d), "entity", "", "ticket"))
		assertNotContains(t, ids, "allowed")
		assertNotContains(t, ids, "not-granted")
		assertNotContains(t, ids, "no-permission")
	})

	t.Run("read-only hides every command", func(t *testing.T) {
		app, _ := setup(t)
		app.acl = acl.ReadOnlyACL{}
		cmds := app.commands.resolveCommands(context.Background(), "entity", "", "ticket")
		if len(cmds) != 0 {
			t.Errorf("read-only must hide all commands, got %v", cmdIDs(cmds))
		}
	})

	t.Run("nop acl shows all", func(t *testing.T) {
		app, _ := setup(t)
		app.acl = acl.NopACL{}
		ids := cmdIDs(app.commands.resolveCommands(context.Background(), "entity", "", "ticket"))
		assertContains(t, ids, "allowed")
		assertContains(t, ids, "not-granted")
		assertContains(t, ids, "no-permission")
	})
}

// TestAuthorizeCommandNilACLDenies pins that a wiring omission fails closed.
// A guard that granted on a nil field would be worse than no guard, since it
// would look present in review.
func TestAuthorizeCommandNilACLDenies(t *testing.T) {
	if authorizeCommand(context.Background(), nil, CommandConfig{Context: "global"}) {
		t.Error("nil ACL must deny")
	}
	h := &commandHandler{} // no aclImpl closure
	if h.currentACL() != nil {
		t.Error("currentACL must return nil when unwired, not panic")
	}
}

// TestAuthorizeCommandUnknownACLDenies pins the inverted default (RR-CAUBAZ).
// The switch must be closed by construction: an acl.ACL implementation with no
// explicit arm denies rather than granting shell execution.
//
// The pointer cases matter because ReadOnlyACL/NopACL declare AuthorizeWrite on
// a VALUE receiver, so &acl.ReadOnlyACL{} satisfies acl.ACL while being a
// distinct dynamic type. Matching only the value form put it in the default
// arm — when that arm granted, `--read-only` was bypassable by one `&`.
func TestAuthorizeCommandUnknownACLDenies(t *testing.T) {
	cmd := CommandConfig{Context: "global", Permission: "command:x"}

	t.Run("pointer ReadOnlyACL denies", func(t *testing.T) {
		if authorizeCommand(context.Background(), &acl.ReadOnlyACL{}, cmd) {
			t.Error("pointer ReadOnlyACL must deny — value-only match was a --read-only bypass")
		}
	})

	t.Run("pointer NopACL still fails open", func(t *testing.T) {
		if !authorizeCommand(context.Background(), &acl.NopACL{}, cmd) {
			t.Error("pointer NopACL should behave like the value form")
		}
	})

	t.Run("unrecognized implementation denies", func(t *testing.T) {
		// *acl.Request implements acl.ACL and has no arm in the switch.
		var unknown acl.ACL = (*acl.Request)(nil)
		if authorizeCommand(context.Background(), unknown, cmd) {
			t.Error("an ACL implementation with no explicit arm must deny")
		}
	})

	t.Run("typed-nil Declarative denies", func(t *testing.T) {
		var typedNil acl.ACL = (*acl.Declarative)(nil)
		if authorizeCommand(context.Background(), typedNil, cmd) {
			t.Error("typed-nil Declarative must deny")
		}
	})
}

// TestCommandCancelOwnerBound pins RR-YZV7SY: a command may be cancelled only
// by the principal that started it. execID is client-supplied and the registry
// is process-global, so an unbound cancel let any caller kill any run —
// including a caller whose own exec attempts were being 403'd.
func TestCommandCancelOwnerBound(t *testing.T) {
	newApp := func(t *testing.T) *App {
		t.Helper()
		app := newHandlerTestApp(t)
		bindRepo(app, t.TempDir())
		return app
	}

	cancelAs := func(t *testing.T, app *App, ctx context.Context, execID string) int {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/api/command-cancel/"+execID, http.NoBody).
			WithContext(ctx)
		w := httptest.NewRecorder()
		app.commands.handleCommandCancel(w, r)
		return w.Code
	}

	t.Run("non-owner gets 404, identical to unknown id", func(t *testing.T) {
		app := newApp(t)
		proc := exec.Command("sleep", "30")
		if err := proc.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		t.Cleanup(func() { _ = proc.Process.Kill() })

		runningCommands.Store("owned-by-alice", &runningCommand{
			cmd:   proc,
			owner: principal.From(principalCtx("alice")),
		})
		t.Cleanup(func() { runningCommands.Delete("owned-by-alice") })

		if code := cancelAs(t, app, principalCtx("bob"), "owned-by-alice"); code != http.StatusNotFound {
			t.Errorf("bob canceling alice's command: expected 404, got %d", code)
		}
		// Indistinguishable from a genuinely unknown id — no running-command oracle.
		if code := cancelAs(t, app, principalCtx("bob"), "no-such-id"); code != http.StatusNotFound {
			t.Errorf("unknown id: expected 404, got %d", code)
		}
		if proc.ProcessState != nil {
			t.Error("victim process must not have been signaled")
		}
	})

	t.Run("owner can cancel", func(t *testing.T) {
		app := newApp(t)
		proc := exec.Command("sleep", "30")
		if err := proc.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		t.Cleanup(func() { _ = proc.Process.Kill() })

		runningCommands.Store("owned-by-alice-2", &runningCommand{
			cmd:   proc,
			owner: principal.From(principalCtx("alice")),
		})
		t.Cleanup(func() { runningCommands.Delete("owned-by-alice-2") })

		if code := cancelAs(t, app, principalCtx("alice"), "owned-by-alice-2"); code != http.StatusOK {
			t.Errorf("owner canceling own command: expected 200, got %d", code)
		}
	})
}

// TestBuildEntityInput_CarriesRedactedNames pins the command-stdin contract
// for ACL-redacted properties (RR-TD74AU). commandInput embeds the DOMAIN
// entity, so `redacted` ships on stdin alongside the pre-existing
// `inaccessible`. That is deliberate — a command hitting a stripped property
// otherwise cannot tell "withheld" from "never set" — and it discloses names
// only, matching the `_redacted` wire field and the Lua binding.
func TestBuildEntityInput_CarriesRedactedNames(t *testing.T) {
	app, entities := testAppInstance()
	bindRepo(app, "/test/project")

	e := entities.ticket1.Clone()
	delete(e.Properties, "status")
	e.Redacted = []string{"status"}

	input := app.commands.buildEntityInput(context.Background(), e)

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded struct {
		Entity struct {
			Properties map[string]any `json:"properties"`
			Redacted   []string       `json:"redacted"`
		} `json:"entity"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.Entity.Redacted) != 1 || decoded.Entity.Redacted[0] != "status" {
		t.Errorf("redacted names missing from stdin payload: %v", decoded.Entity.Redacted)
	}
	if _, present := decoded.Entity.Properties["status"]; present {
		t.Error("redacted property VALUE reached the command payload")
	}
}
