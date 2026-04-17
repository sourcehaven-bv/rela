package lua

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockTransport is a test transport that returns pre-configured events.
type mockTransport struct {
	screens []Screen // Screens received
	events  []Event  // Events to return
	index   int      // Current event index
	err     error    // Error to return (if set)
}

func (m *mockTransport) Present(screen Screen) (Event, error) {
	m.screens = append(m.screens, screen)
	if m.err != nil {
		return Event{}, m.err
	}
	if m.index >= len(m.events) {
		return Event{}, errors.New("no more events configured")
	}
	event := m.events[m.index]
	m.index++
	return event, nil
}

func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	var buf bytes.Buffer
	r := New(Services{ProjectRoot: "/tmp"}, &buf)
	return r
}

func TestFlowRuntime_SimpleForm(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{
			{Action: "submit", Data: map[string]any{"title": "Test"}},
		},
	}

	flow := NewFlowRuntime(r, transport)

	code := `
		local event = rela.flow.emit({
			type = "form",
			title = "Test Form",
			fields = {
				{name = "title", type = "text", required = true},
			},
			actions = {
				{"submit", "Submit"},
				{"cancel", "Cancel"},
			},
		})
		-- Store result for verification
		_G.result_action = event.action
		_G.result_title = event.data.title
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify screen was presented
	if len(transport.screens) != 1 {
		t.Fatalf("expected 1 screen, got %d", len(transport.screens))
	}

	screen := transport.screens[0]
	if screen.Type != "form" {
		t.Errorf("expected type 'form', got '%s'", screen.Type)
	}
	if screen.Title != "Test Form" {
		t.Errorf("expected title 'Test Form', got '%s'", screen.Title)
	}
	if len(screen.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(screen.Fields))
	}
	if len(screen.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(screen.Actions))
	}

	// Verify script received event
	action := r.L.GetGlobal("result_action")
	if action.String() != "submit" {
		t.Errorf("expected action 'submit', got '%s'", action.String())
	}
	title := r.L.GetGlobal("result_title")
	if title.String() != "Test" {
		t.Errorf("expected title 'Test', got '%s'", title.String())
	}
}

func TestFlowRuntime_MultiStepFlow(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{
			{Action: "next", Data: map[string]any{"name": "Alice"}},
			{Action: "submit", Data: map[string]any{"email": "alice@example.com"}},
		},
	}

	flow := NewFlowRuntime(r, transport)

	code := `
		-- Step 1
		local e1 = rela.flow.emit({
			type = "form",
			title = "Step 1",
			fields = {{name = "name", type = "text"}},
			actions = {{"next", "Next"}, {"cancel", "Cancel"}},
		})
		if e1.action == "cancel" then return end

		-- Step 2
		local e2 = rela.flow.emit({
			type = "form",
			title = "Step 2",
			fields = {{name = "email", type = "text"}},
			actions = {{"back", "Back"}, {"submit", "Submit"}},
		})

		_G.result_name = e1.data.name
		_G.result_email = e2.data.email
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both screens were presented
	if len(transport.screens) != 2 {
		t.Fatalf("expected 2 screens, got %d", len(transport.screens))
	}

	if transport.screens[0].Title != "Step 1" {
		t.Errorf("expected first screen title 'Step 1', got '%s'", transport.screens[0].Title)
	}
	if transport.screens[1].Title != "Step 2" {
		t.Errorf("expected second screen title 'Step 2', got '%s'", transport.screens[1].Title)
	}

	// Verify data was collected
	name := r.L.GetGlobal("result_name")
	if name.String() != "Alice" {
		t.Errorf("expected name 'Alice', got '%s'", name.String())
	}
	email := r.L.GetGlobal("result_email")
	if email.String() != "alice@example.com" {
		t.Errorf("expected email 'alice@example.com', got '%s'", email.String())
	}
}

func TestFlowRuntime_CancelFlow(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{
			{Action: "cancel", Data: nil},
		},
	}

	flow := NewFlowRuntime(r, transport)

	code := `
		local event = rela.flow.emit({
			type = "form",
			fields = {{name = "title", type = "text"}},
			actions = {{"submit", "Submit"}, {"cancel", "Cancel"}},
		})
		if event.action == "cancel" then
			_G.was_cancelled = true
			return
		end
		_G.was_cancelled = false
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cancelled := r.L.GetGlobal("was_cancelled")
	if cancelled.String() != "true" {
		t.Errorf("expected was_cancelled=true, got %s", cancelled.String())
	}
}

func TestFlowRuntime_AllFieldTypes(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{
			{Action: "submit", Data: map[string]any{
				"text_field":   "hello",
				"select_field": "option1",
				"multi_field":  []any{"a", "b"},
				"bool_field":   true,
				"number_field": 42.5,
				"date_field":   "2024-01-15",
			}},
		},
	}

	flow := NewFlowRuntime(r, transport)

	code := `
		local event = rela.flow.emit({
			type = "form",
			fields = {
				{name = "text_field", type = "text", placeholder = "Enter text"},
				{name = "select_field", type = "select", options = {{"option1", "Option 1"}, {"option2", "Option 2"}}},
				{name = "multi_field", type = "multi-select", options = {{"a", "A"}, {"b", "B"}, {"c", "C"}}},
				{name = "bool_field", type = "boolean", default = false},
				{name = "number_field", type = "number", min = 0, max = 100},
				{name = "date_field", type = "date", min = "2024-01-01", max = "2024-12-31"},
			},
			actions = {{"submit", "Submit"}},
		})
		_G.result = event.data
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all field types were parsed
	screen := transport.screens[0]
	if len(screen.Fields) != 6 {
		t.Fatalf("expected 6 fields, got %d", len(screen.Fields))
	}

	// Check specific field properties
	textField := screen.Fields[0]
	if textField.Type != "text" || textField.Placeholder != "Enter text" {
		t.Errorf("text field not parsed correctly: %+v", textField)
	}

	selectField := screen.Fields[1]
	if selectField.Type != "select" || len(selectField.Options) != 2 {
		t.Errorf("select field not parsed correctly: %+v", selectField)
	}

	numberField := screen.Fields[4]
	if numberField.Type != "number" || numberField.Min == nil || *numberField.Min != 0 {
		t.Errorf("number field not parsed correctly: %+v", numberField)
	}

	dateField := screen.Fields[5]
	if dateField.Type != "date" || dateField.MinDate != "2024-01-01" {
		t.Errorf("date field not parsed correctly: %+v", dateField)
	}
}

func TestFlowRuntime_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr string
	}{
		{
			name:    "missing type",
			code:    `rela.flow.emit({fields = {}, actions = {}})`,
			wantErr: "missing required field 'type'",
		},
		{
			name:    "unknown screen type",
			code:    `rela.flow.emit({type = "wizard", fields = {}, actions = {}})`,
			wantErr: "unknown screen type 'wizard'",
		},
		{
			name:    "missing fields",
			code:    `rela.flow.emit({type = "form", actions = {}})`,
			wantErr: "missing required field 'fields'",
		},
		{
			name:    "missing actions",
			code:    `rela.flow.emit({type = "form", fields = {}})`,
			wantErr: "missing required field 'actions'",
		},
		{
			name:    "field missing name",
			code:    `rela.flow.emit({type = "form", fields = {{type = "text"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "missing required property 'name'",
		},
		{
			name:    "field missing type",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "missing required property 'type'",
		},
		{
			name:    "unknown field type",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "widget"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "unknown field type 'widget'",
		},
		{
			name:    "invalid field name format",
			code:    `rela.flow.emit({type = "form", fields = {{name = "123invalid", type = "text"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "must match pattern",
		},
		{
			name:    "duplicate field name",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "text"}, {name = "foo", type = "text"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "duplicate field name 'foo'",
		},
		{
			name:    "select without options",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "select"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "requires 'options'",
		},
		{
			name:    "empty options",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "select", options = {}}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "options cannot be empty",
		},
		{
			name:    "duplicate option value",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "select", options = {{"a", "A"}, {"a", "B"}}}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "duplicate option value 'a'",
		},
		{
			name:    "action missing id",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "text"}}, actions = {{nil, "Submit"}}})`,
			wantErr: "action missing id",
		},
		{
			name:    "action missing label",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "text"}}, actions = {{"submit"}}})`,
			wantErr: "missing label",
		},
		{
			name:    "duplicate action id",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "text"}}, actions = {{"submit", "Go"}, {"submit", "Also Go"}}})`,
			wantErr: "duplicate action id 'submit'",
		},
		{
			name:    "invalid action style",
			code:    `rela.flow.emit({type = "form", fields = {{name = "foo", type = "text"}}, actions = {{"submit", "Submit", "fancy"}}})`,
			wantErr: "invalid style 'fancy'",
		},
		{
			name:    "invalid date format",
			code:    `rela.flow.emit({type = "form", fields = {{name = "d", type = "date", min = "01-15-2024"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "must be YYYY-MM-DD format",
		},
		{
			name:    "invalid step value",
			code:    `rela.flow.emit({type = "form", fields = {{name = "n", type = "number", step = 0}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "step must be > 0",
		},
		{
			name:    "number min greater than max",
			code:    `rela.flow.emit({type = "form", fields = {{name = "n", type = "number", min = 100, max = 1}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "min (100) must be <= max (1)",
		},
		{
			name:    "date min greater than max",
			code:    `rela.flow.emit({type = "form", fields = {{name = "d", type = "date", min = "2025-12-31", max = "2024-01-01"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "min date (2025-12-31) must be <= max date (2024-01-01)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRuntime(t)
			defer r.Close()

			transport := &mockTransport{
				events: []Event{{Action: "submit"}},
			}

			flow := NewFlowRuntime(r, transport)
			err := flow.RunString(tt.code)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestFlowRuntime_TransportError(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		err: errors.New("user interrupted"),
	}

	flow := NewFlowRuntime(r, transport)

	code := `
		rela.flow.emit({
			type = "form",
			fields = {{name = "title", type = "text"}},
			actions = {{"submit", "Submit"}},
		})
	`

	err := flow.RunString(code)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "user interrupted") {
		t.Errorf("expected error containing 'user interrupted', got %q", err.Error())
	}
}

func TestFlowRuntime_ScriptWithoutEmit(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{}
	flow := NewFlowRuntime(r, transport)

	code := `
		-- Script that doesn't emit any forms
		_G.result = 1 + 1
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should complete successfully with no screens presented
	if len(transport.screens) != 0 {
		t.Errorf("expected 0 screens, got %d", len(transport.screens))
	}

	result := r.L.GetGlobal("result")
	if result.String() != "2" {
		t.Errorf("expected result=2, got %s", result.String())
	}
}

func TestFlowRuntime_MarkdownField(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{
			{Action: "submit", Data: map[string]any{"title": "Test"}},
		},
	}

	flow := NewFlowRuntime(r, transport)

	code := `
		local event = rela.flow.emit({
			type = "form",
			title = "Form with Markdown",
			fields = {
				{type = "markdown", content = "## Instructions\nPlease fill out the form below."},
				{name = "title", type = "text", required = true},
				{type = "markdown", content = "---\n*Additional options:*"},
				{name = "priority", type = "select", options = {{"high", "High"}, {"low", "Low"}}},
				{type = "markdown", label = "Note", content = "This is a **note** with a title."},
			},
			actions = {{"submit", "Submit"}},
		})
		_G.result_title = event.data.title
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify screen was parsed correctly
	screen := transport.screens[0]
	if len(screen.Fields) != 5 {
		t.Fatalf("expected 5 fields, got %d", len(screen.Fields))
	}

	// Check markdown fields
	md1 := screen.Fields[0]
	if md1.Type != "markdown" {
		t.Errorf("expected type 'markdown', got '%s'", md1.Type)
	}
	if md1.Content != "## Instructions\nPlease fill out the form below." {
		t.Errorf("unexpected content: %s", md1.Content)
	}
	if md1.Name != "" {
		t.Errorf("markdown field should not have a name, got '%s'", md1.Name)
	}

	// Check text field in between
	textField := screen.Fields[1]
	if textField.Type != "text" || textField.Name != "title" {
		t.Errorf("expected text field 'title', got %+v", textField)
	}

	// Check markdown with label
	md3 := screen.Fields[4]
	if md3.Type != "markdown" {
		t.Errorf("expected type 'markdown', got '%s'", md3.Type)
	}
	if md3.Label != "Note" {
		t.Errorf("expected label 'Note', got '%s'", md3.Label)
	}
	if md3.Content != "This is a **note** with a title." {
		t.Errorf("unexpected content: %s", md3.Content)
	}

	// Verify script received event data (markdown fields don't contribute)
	title := r.L.GetGlobal("result_title")
	if title.String() != "Test" {
		t.Errorf("expected title 'Test', got '%s'", title.String())
	}
}

func TestFlowRuntime_MarkdownFieldValidation(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr string
	}{
		{
			name:    "markdown missing content",
			code:    `rela.flow.emit({type = "form", fields = {{type = "markdown"}}, actions = {{"submit", "Submit"}}})`,
			wantErr: "markdown field missing required property 'content'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRuntime(t)
			defer r.Close()

			transport := &mockTransport{
				events: []Event{{Action: "submit"}},
			}

			flow := NewFlowRuntime(r, transport)
			err := flow.RunString(tt.code)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestFlowRuntime_MultipleMarkdownFieldsNoDuplicateError(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{{Action: "submit"}},
	}

	flow := NewFlowRuntime(r, transport)

	// Multiple markdown fields should not cause duplicate name error
	code := `
		rela.flow.emit({
			type = "form",
			fields = {
				{type = "markdown", content = "First markdown"},
				{type = "markdown", content = "Second markdown"},
				{type = "markdown", content = "Third markdown"},
				{name = "title", type = "text"},
			},
			actions = {{"submit", "Submit"}},
		})
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	screen := transport.screens[0]
	if len(screen.Fields) != 4 {
		t.Fatalf("expected 4 fields, got %d", len(screen.Fields))
	}
}

func TestFlowRuntime_LabelDefaultsToTitleCase(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{{Action: "submit"}},
	}

	flow := NewFlowRuntime(r, transport)

	code := `
		rela.flow.emit({
			type = "form",
			fields = {
				{name = "user_name", type = "text"},
				{name = "email", type = "text"},
			},
			actions = {{"submit", "Submit"}},
		})
	`

	err := flow.RunString(code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	screen := transport.screens[0]
	if screen.Fields[0].Label != "User name" {
		t.Errorf("expected label 'User name', got '%s'", screen.Fields[0].Label)
	}
	if screen.Fields[1].Label != "Email" {
		t.Errorf("expected label 'Email', got '%s'", screen.Fields[1].Label)
	}
}

func TestFlowRuntime_RunFile(t *testing.T) {
	r := newTestRuntime(t)
	defer r.Close()

	transport := &mockTransport{
		events: []Event{
			{Action: "submit", Data: map[string]any{"title": "From File"}},
		},
	}

	flow := NewFlowRuntime(r, transport)

	// Write a temp script file
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "test-flow.lua")
	script := `
		local event = rela.flow.emit({
			type = "form",
			fields = {{name = "title", type = "text"}},
			actions = {{"submit", "Submit"}},
		})
		_G.result_title = event.data.title
		-- Verify args are accessible
		_G.arg_count = #rela.args
		_G.first_arg = rela.args[1]
	`
	err := os.WriteFile(scriptPath, []byte(script), 0o644)
	if err != nil {
		t.Fatalf("write script: %v", err)
	}

	err = flow.RunFile(scriptPath, []string{"--verbose"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(transport.screens) != 1 {
		t.Fatalf("expected 1 screen, got %d", len(transport.screens))
	}

	title := r.L.GetGlobal("result_title")
	if title.String() != "From File" {
		t.Errorf("expected title 'From File', got '%s'", title.String())
	}

	argCount := r.L.GetGlobal("arg_count")
	if argCount.String() != "1" {
		t.Errorf("expected arg_count=1, got %s", argCount.String())
	}

	firstArg := r.L.GetGlobal("first_arg")
	if firstArg.String() != "--verbose" {
		t.Errorf("expected first_arg='--verbose', got '%s'", firstArg.String())
	}
}

func TestValidateEventData_UnexpectedField(t *testing.T) {
	screen := Screen{
		Type: "form",
		Fields: []Field{
			{Name: "title", Type: "text"},
		},
	}

	// Valid data
	err := validateEventData(Event{Action: "submit", Data: map[string]any{"title": "ok"}}, screen)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Unexpected field
	err = validateEventData(Event{Action: "submit", Data: map[string]any{"title": "ok", "hacked": "bad"}}, screen)
	if err == nil {
		t.Fatal("expected error for unexpected field")
	}
	if !strings.Contains(err.Error(), "unexpected field 'hacked'") {
		t.Errorf("expected error about 'hacked', got: %v", err)
	}

	// Nil data is fine
	err = validateEventData(Event{Action: "cancel"}, screen)
	if err != nil {
		t.Errorf("expected no error for nil data, got: %v", err)
	}

	// Markdown fields should be excluded from expected
	screenWithMarkdown := Screen{
		Type: "form",
		Fields: []Field{
			{Type: "markdown", Content: "info"},
			{Name: "title", Type: "text"},
		},
	}
	err = validateEventData(Event{Action: "submit", Data: map[string]any{"title": "ok"}}, screenWithMarkdown)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
