package dataentry

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// --- resolveCommands ---

func TestResolveCommands(t *testing.T) {
	app, _ := testAppInstance()
	app.Cfg.Views = map[string]ViewConfig{
		"ticket_detail": {Title: "Ticket Detail", Entry: ViewEntry{Type: "ticket"}},
	}
	app.Cfg.Commands = map[string]CommandConfig{
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
		cmds := app.resolveCommands("entity", "", "ticket")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "entity-cmd")
		assertContains(t, ids, "unscoped-entity")
		assertNotContains(t, ids, "view-cmd")
		assertNotContains(t, ids, "list-cmd")
		assertNotContains(t, ids, "global-cmd")
	})

	t.Run("entity page for non-matching type", func(t *testing.T) {
		cmds := app.resolveCommands("entity", "", "component")
		ids := cmdIDs(cmds)
		// unscoped entity command still shows (context matches)
		assertContains(t, ids, "unscoped-entity")
		// scoped to ticket only
		assertNotContains(t, ids, "entity-cmd")
	})

	t.Run("view page shows entity and view commands", func(t *testing.T) {
		cmds := app.resolveCommands("view", "ticket_detail", "ticket")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "entity-cmd")
		assertContains(t, ids, "view-cmd")
		assertContains(t, ids, "unscoped-entity")
		assertNotContains(t, ids, "list-cmd")
		assertNotContains(t, ids, "global-cmd")
	})

	t.Run("list page shows list commands", func(t *testing.T) {
		cmds := app.resolveCommands("list", "tickets", "ticket")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "list-cmd")
		assertNotContains(t, ids, "entity-cmd")
		assertNotContains(t, ids, "view-cmd")
	})

	t.Run("dashboard shows global commands", func(t *testing.T) {
		cmds := app.resolveCommands("dashboard", "", "")
		ids := cmdIDs(cmds)
		assertContains(t, ids, "global-cmd")
		assertNotContains(t, ids, "entity-cmd")
	})

	t.Run("empty commands returns nil", func(t *testing.T) {
		app2, _ := testAppInstance()
		cmds := app2.resolveCommands("entity", "", "ticket")
		if cmds != nil {
			t.Errorf("expected nil, got %v", cmds)
		}
	})

	t.Run("auto_open propagated to resolved command", func(t *testing.T) {
		app2, _ := testAppInstance()
		trueVal := true
		app2.Cfg.Commands = map[string]CommandConfig{
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
		cmds := app2.resolveCommands("entity", "", "ticket")
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
		cmds := app.resolveCommands("view", "ticket_detail", "ticket")
		if len(cmds) < 2 {
			t.Skip("need at least 2 commands")
		}
		// Run multiple times and check order is stable
		for i := 0; i < 5; i++ {
			cmds2 := app.resolveCommands("view", "ticket_detail", "ticket")
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
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: "/test/project"}),
		app.meta, app.g)
	app.g.AddEdge(model.NewRelation(entities.ticket1.ID, "depends_on", entities.ticket2.ID))

	input := app.buildEntityInput(entities.ticket1)

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
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded["context"] != "entity" {
		t.Error("JSON context field mismatch")
	}
}

func TestBuildListInput(t *testing.T) {
	app, _ := testAppInstance()
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: "/test/project"}),
		app.meta, app.g)
	entities := app.g.NodesByType("ticket")

	input := app.buildListInput("tickets", entities)

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
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: "/test/project"}),
		app.meta, app.g)
	app.g.AddEdge(model.NewRelation("TKT-001", "belongs_to", "CMP-001"))

	view := ViewConfig{
		Title: "Test View",
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "belongs_to", CollectAs: "components"},
		},
	}
	vr, err := app.executeView(view, "TKT-001")
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}

	input := app.buildViewInput("test_view", vr)

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
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: "/test/project"}),
		app.meta, app.g)

	input := app.buildGlobalInput()

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
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: "/test/project"}),
		app.meta, app.g)

	cmd := CommandConfig{
		Script:  "echo hi",
		Context: "entity",
		Env:     map[string]string{"FORMAT": "pdf"},
	}
	input := app.buildEntityInput(entities.ticket1)
	env := app.buildCommandEnv(cmd, input)

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
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: "/test/project"}),
		app.meta, app.g)

	cmd := CommandConfig{Script: "echo hi", Context: "list"}
	input := app.buildListInput("tickets", nil)
	env := app.buildCommandEnv(cmd, input)

	envMap := envToMap(env)
	if envMap["RELA_LIST_ID"] != "tickets" {
		t.Errorf("expected tickets, got %s", envMap["RELA_LIST_ID"])
	}
}

func TestBuildCommandEnvViewContext(t *testing.T) {
	app, entities := testAppInstance()
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: "/test/project"}),
		app.meta, app.g)

	cmd := CommandConfig{Script: "echo hi", Context: "view"}
	input := &commandInput{
		Context: "view",
		ViewID:  "ticket_detail",
		Entity:  entities.ticket1,
		Project: app.projectInfo(),
	}
	env := app.buildCommandEnv(cmd, input)

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
	app, _ := newHandlerTestApp(t)
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: t.TempDir()}),
		app.meta, app.g)
	app.Cfg.Commands = map[string]CommandConfig{
		"test-echo": {
			Label:   "Test Echo",
			Script:  `echo '::rela::{"type":"message","text":"hello from test"}' && echo 'raw log line'`,
			Context: "entity",
		},
	}

	t.Run("success stream", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/command/test-echo?entity_id=TKT-001&entity_type=ticket", http.NoBody)
		w := httptest.NewRecorder()
		app.handleCommandExec(w, r)

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
		r := httptest.NewRequest(http.MethodGet, "/api/command/nonexistent", http.NoBody)
		w := httptest.NewRecorder()
		app.handleCommandExec(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("entity not found", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/command/test-echo?entity_id=NOPE", http.NoBody)
		w := httptest.NewRecorder()
		app.handleCommandExec(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/api/command/test-echo", http.NoBody)
		w := httptest.NewRecorder()
		app.handleCommandExec(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleCommandExecFailing(t *testing.T) {
	app, _ := newHandlerTestApp(t)
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: t.TempDir()}),
		app.meta, app.g)
	app.Cfg.Commands = map[string]CommandConfig{
		"fail-cmd": {
			Label:   "Fail",
			Script:  "echo 'failing' >&2 && exit 1",
			Context: "entity",
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/command/fail-cmd?entity_id=TKT-001", http.NoBody)
	w := httptest.NewRecorder()
	app.handleCommandExec(w, r)

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
	app, _ := newHandlerTestApp(t)
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: t.TempDir()}),
		app.meta, app.g)
	app.Cfg.Commands = map[string]CommandConfig{
		"global-cmd": {
			Label:   "Global",
			Script:  `echo '::rela::{"type":"message","text":"global ok"}'`,
			Context: "global",
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/command/global-cmd", http.NoBody)
	w := httptest.NewRecorder()
	app.handleCommandExec(w, r)

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
	app, _ := newHandlerTestApp(t)
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: t.TempDir()}),
		app.meta, app.g)
	app.Cfg.Commands = map[string]CommandConfig{
		"list-cmd": {
			Label:   "List",
			Script:  `echo '::rela::{"type":"message","text":"list ok"}'`,
			Context: "list",
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/command/list-cmd?list_id=tickets", http.NoBody)
	w := httptest.NewRecorder()
	app.handleCommandExec(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCommandExecViewContext(t *testing.T) {
	app, _ := newHandlerTestApp(t)
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: t.TempDir()}),
		app.meta, app.g)
	app.Cfg.Commands = map[string]CommandConfig{
		"view-cmd": {
			Label:   "View",
			Script:  `echo '::rela::{"type":"message","text":"view ok"}'`,
			Context: "view",
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/command/view-cmd?view_id=ticket_detail&entity_id=TKT-001", http.NoBody)
	w := httptest.NewRecorder()
	app.handleCommandExec(w, r)

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
		app.handleCommandCancel(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		app, _ := testAppInstance()
		r := httptest.NewRequest(http.MethodGet, "/api/command-cancel/test", http.NoBody)
		w := httptest.NewRecorder()
		app.handleCommandCancel(w, r)
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
		app.handleOpenURL(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/open-url?url=ftp://evil.com", http.NoBody)
		w := httptest.NewRecorder()
		app.handleOpenURL(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/open-url?url=https://example.com", http.NoBody)
		w := httptest.NewRecorder()
		app.handleOpenURL(w, r)
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
		app.handleOpenFile(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/open-file?path=/tmp/test", http.NoBody)
		w := httptest.NewRecorder()
		app.handleOpenFile(w, r)
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
	for _, id := range ids {
		if id == expected {
			return
		}
	}
	t.Errorf("expected %q in %v", expected, ids)
}

func assertNotContains(t *testing.T, ids []string, unexpected string) {
	t.Helper()
	for _, id := range ids {
		if id == unexpected {
			t.Errorf("did not expect %q in %v", unexpected, ids)
			return
		}
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
