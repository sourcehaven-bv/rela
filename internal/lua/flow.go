// This file implements interactive flows using Lua coroutines.
package lua

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

// Flow-related constants.
const (
	maxFieldNameLength = 64
	maxOptionsCount    = 1000
)

// fieldNamePattern validates field names: starts with letter, alphanumeric + underscore.
var fieldNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// Transport presents screens to users and receives events.
type Transport interface {
	// Present displays a screen to the user and blocks until an event is received.
	Present(screen Screen) (Event, error)
}

// Screen represents a UI screen to present to the user.
type Screen struct {
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Fields      []Field  `json:"fields,omitempty"`
	Actions     []Action `json:"actions,omitempty"`
}

// Field represents a form field.
type Field struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Label       string         `json:"label,omitempty"`
	Content     string         `json:"content,omitempty"` // Markdown content (for type="markdown")
	Placeholder string         `json:"placeholder,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Default     any            `json:"default,omitempty"`
	Options     []SelectOption `json:"options,omitempty"`
	Lines       int            `json:"lines,omitempty"`
	Min         *float64       `json:"min,omitempty"`
	Max         *float64       `json:"max,omitempty"`
	Step        *float64       `json:"step,omitempty"`
	MinDate     string         `json:"min_date,omitempty"`
	MaxDate     string         `json:"max_date,omitempty"`
}

// SelectOption represents a select option.
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Action represents a form action button.
type Action struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Style string `json:"style,omitempty"`
}

// Event represents user input from a screen.
type Event struct {
	Action string         `json:"action"`
	Data   map[string]any `json:"data,omitempty"`
}

// FlowRuntime manages the execution of an interactive Lua flow.
type FlowRuntime struct {
	l         *lua.LState // Main Lua state
	runtime   *Runtime
	co        *lua.LState // Coroutine thread
	transport Transport
	fn        *lua.LFunction // The script function to run
}

// NewFlowRuntime creates a new flow runtime with the given transport.
func NewFlowRuntime(r *Runtime, transport Transport) *FlowRuntime {
	return &FlowRuntime{
		l:         r.L,
		runtime:   r,
		transport: transport,
	}
}

// RunFile loads and executes a flow script file.
func (f *FlowRuntime) RunFile(path string, args []string) error {
	// Load the script file
	fn, err := f.l.LoadFile(path)
	if err != nil {
		return fmt.Errorf("load script: %w", err)
	}

	// Set args
	argsTable := f.l.NewTable()
	for i, arg := range args {
		argsTable.RawSetInt(i+1, lua.LString(arg))
	}
	if relaTable, ok := f.l.GetGlobal("rela").(*lua.LTable); ok {
		relaTable.RawSetString("args", argsTable)
	}

	return f.run(fn)
}

// RunString executes a flow script from a string.
func (f *FlowRuntime) RunString(code string) error {
	fn, err := f.l.LoadString(code)
	if err != nil {
		return fmt.Errorf("load script: %w", err)
	}
	return f.run(fn)
}

// run executes the flow using coroutines.
func (f *FlowRuntime) run(fn *lua.LFunction) error {
	// Register emit function that yields
	if err := f.registerEmit(); err != nil {
		return err
	}

	// Disable timeout for interactive flows — human think time is unbounded.
	// The timeout is cleared here before creating the coroutine thread, so
	// the thread won't inherit a context with a deadline.
	f.runtime.clearTimeout()

	// Create coroutine thread
	var coCancel context.CancelFunc
	f.co, coCancel = f.l.NewThread()
	if coCancel != nil {
		defer coCancel()
	}
	f.fn = fn

	// First resume starts the script
	var resumeArgs []lua.LValue

	// Resume loop
	for {
		status, err, values := f.l.Resume(f.co, fn, resumeArgs...)

		switch status {
		case lua.ResumeOK:
			// Script finished normally
			return nil

		case lua.ResumeError:
			return fmt.Errorf("script error: %w", err)

		case lua.ResumeYield:
			// Script yielded a screen
			if len(values) == 0 {
				return fmt.Errorf("emit: no screen provided")
			}

			screenTable, ok := values[0].(*lua.LTable)
			if !ok {
				return fmt.Errorf("emit: expected table, got %s", values[0].Type())
			}

			screen, err := f.parseScreen(screenTable)
			if err != nil {
				return fmt.Errorf("emit: %w", err)
			}

			// Present to user via transport (blocks until user responds)
			event, err := f.transport.Present(screen)
			if err != nil {
				return fmt.Errorf("transport: %w", err)
			}

			// Validate event data matches screen fields
			if err := validateEventData(event, screen); err != nil {
				return fmt.Errorf("transport: %w", err)
			}

			// Set event table as args for next resume
			eventTable := f.eventToTable(event)
			resumeArgs = []lua.LValue{eventTable}

			// fn must be nil after first resume — gopher-lua's Resume
			// only accepts the function on the initial call.
			fn = nil
		}
	}
}

// registerEmit registers the rela.flow.emit function.
func (f *FlowRuntime) registerEmit() error {
	relaTable, ok := f.l.GetGlobal("rela").(*lua.LTable)
	if !ok {
		return fmt.Errorf("rela module not initialized")
	}

	flowTable := f.l.NewTable()
	f.l.SetField(flowTable, "emit", f.l.NewFunction(f.luaEmit))
	f.l.SetField(relaTable, "flow", flowTable)
	return nil
}

// luaEmit implements rela.flow.emit(screen) - yields the screen spec.
func (f *FlowRuntime) luaEmit(ls *lua.LState) int {
	screen := ls.CheckTable(1)
	// Yield with the screen table - the runtime will parse and validate it
	return ls.Yield(screen)
}

// parseScreen converts a Lua table to a Screen struct with validation.
func (f *FlowRuntime) parseScreen(t *lua.LTable) (Screen, error) {
	screen := Screen{}

	// Type (required)
	typeVal := t.RawGetString("type")
	if typeVal == lua.LNil {
		return screen, fmt.Errorf("validation: missing required field 'type'")
	}
	screen.Type = lua.LVAsString(typeVal)

	if screen.Type != "form" {
		return screen, fmt.Errorf("validation: unknown screen type '%s'", screen.Type)
	}

	// Title (optional)
	if v := t.RawGetString("title"); v != lua.LNil {
		screen.Title = lua.LVAsString(v)
	}

	// Description (optional)
	if v := t.RawGetString("description"); v != lua.LNil {
		screen.Description = lua.LVAsString(v)
	}

	// Fields (required for form)
	fieldsVal := t.RawGetString("fields")
	if fieldsVal == lua.LNil {
		return screen, fmt.Errorf("validation: missing required field 'fields'")
	}
	fieldsTable, ok := fieldsVal.(*lua.LTable)
	if !ok {
		return screen, fmt.Errorf("validation: 'fields' must be a table")
	}

	fields, err := f.parseFields(fieldsTable)
	if err != nil {
		return screen, err
	}
	screen.Fields = fields

	// Actions (required for form)
	actionsVal := t.RawGetString("actions")
	if actionsVal == lua.LNil {
		return screen, fmt.Errorf("validation: missing required field 'actions'")
	}
	actionsTable, ok := actionsVal.(*lua.LTable)
	if !ok {
		return screen, fmt.Errorf("validation: 'actions' must be a table")
	}

	actions, err := f.parseActions(actionsTable)
	if err != nil {
		return screen, err
	}
	screen.Actions = actions

	return screen, nil
}

// parseFields parses and validates the fields array.
func (f *FlowRuntime) parseFields(t *lua.LTable) ([]Field, error) {
	var fields []Field
	seenNames := make(map[string]bool)

	var parseErr error
	t.ForEach(func(_, v lua.LValue) {
		if parseErr != nil {
			return
		}

		fieldTable, ok := v.(*lua.LTable)
		if !ok {
			parseErr = fmt.Errorf("validation: field must be a table")
			return
		}

		field, err := f.parseField(fieldTable)
		if err != nil {
			parseErr = err
			return
		}

		// Check uniqueness (skip for markdown fields which have no name)
		if field.Type != "markdown" {
			if seenNames[field.Name] {
				parseErr = fmt.Errorf("validation: duplicate field name '%s'", field.Name)
				return
			}
			seenNames[field.Name] = true
		}

		fields = append(fields, field)
	})

	return fields, parseErr
}

// parseField parses and validates a single field.
func (f *FlowRuntime) parseField(t *lua.LTable) (Field, error) {
	field := Field{}

	// Type (required) - check first to handle markdown differently
	typeVal := t.RawGetString("type")
	if typeVal == lua.LNil {
		return field, fmt.Errorf("validation: field missing required property 'type'")
	}
	field.Type = lua.LVAsString(typeVal)

	validTypes := map[string]bool{
		"text": true, "select": true, "multi-select": true,
		"boolean": true, "number": true, "date": true, "markdown": true,
	}
	if !validTypes[field.Type] {
		return field, fmt.Errorf("validation: unknown field type '%s'", field.Type)
	}

	// Markdown fields have content instead of name
	if field.Type == "markdown" {
		return f.parseMarkdownField(t, field)
	}

	// Parse input field (has name, can collect data)
	return f.parseInputField(t, field)
}

// parseMarkdownField parses a display-only markdown field.
func (f *FlowRuntime) parseMarkdownField(t *lua.LTable, field Field) (Field, error) {
	contentVal := t.RawGetString("content")
	if contentVal == lua.LNil {
		return field, fmt.Errorf("validation: markdown field missing required property 'content'")
	}
	field.Content = lua.LVAsString(contentVal)
	// Label is optional for markdown (used as title)
	if v := t.RawGetString("label"); v != lua.LNil {
		field.Label = lua.LVAsString(v)
	}
	return field, nil
}

// parseInputField parses a data-collecting input field.
func (f *FlowRuntime) parseInputField(t *lua.LTable, field Field) (Field, error) {
	// Name (required)
	nameVal := t.RawGetString("name")
	if nameVal == lua.LNil {
		return field, fmt.Errorf("validation: field missing required property 'name'")
	}
	field.Name = lua.LVAsString(nameVal)

	// Validate name format
	if !fieldNamePattern.MatchString(field.Name) {
		return field, fmt.Errorf("validation: field name '%s' must match pattern [a-zA-Z][a-zA-Z0-9_]*", field.Name)
	}
	if len(field.Name) > maxFieldNameLength {
		return field, fmt.Errorf("validation: field name '%s' exceeds max length %d", field.Name, maxFieldNameLength)
	}

	// Label (optional, defaults to titlecased name)
	if v := t.RawGetString("label"); v != lua.LNil {
		field.Label = lua.LVAsString(v)
	} else {
		field.Label = titleCase(field.Name)
	}

	// Required (optional)
	if v := t.RawGetString("required"); v != lua.LNil {
		field.Required = lua.LVAsBool(v)
	}

	// Placeholder (optional, text only)
	if v := t.RawGetString("placeholder"); v != lua.LNil {
		field.Placeholder = lua.LVAsString(v)
	}

	// Lines (optional, text only)
	if v := t.RawGetString("lines"); v != lua.LNil {
		if n, ok := v.(lua.LNumber); ok {
			field.Lines = int(n)
			if field.Lines < 1 {
				return field, fmt.Errorf("validation: field '%s' lines must be >= 1", field.Name)
			}
		}
	}

	// Default (optional)
	if v := t.RawGetString("default"); v != lua.LNil {
		field.Default = luaValueToGo(v)
	}

	// Options (required for select/multi-select)
	if field.Type == "select" || field.Type == "multi-select" {
		optionsVal := t.RawGetString("options")
		if optionsVal == lua.LNil {
			return field, fmt.Errorf("validation: field '%s' requires 'options' for type '%s'", field.Name, field.Type)
		}
		optionsTable, ok := optionsVal.(*lua.LTable)
		if !ok {
			return field, fmt.Errorf("validation: field '%s' options must be a table", field.Name)
		}

		options, err := f.parseOptions(optionsTable, field.Name)
		if err != nil {
			return field, err
		}
		field.Options = options
	}

	// Parse type-specific constraints
	if err := f.parseFieldConstraints(t, &field); err != nil {
		return field, err
	}

	return field, nil
}

// parseFieldConstraints parses type-specific field constraints (min/max/step for number, min/max for date).
func (f *FlowRuntime) parseFieldConstraints(t *lua.LTable, field *Field) error {
	switch field.Type {
	case "number":
		return f.parseNumberConstraints(t, field)
	case "date":
		return f.parseDateConstraints(t, field)
	}
	return nil
}

// parseNumberConstraints parses min, max, step for number fields.
func (f *FlowRuntime) parseNumberConstraints(t *lua.LTable, field *Field) error {
	if v := t.RawGetString("min"); v != lua.LNil {
		if n, ok := v.(lua.LNumber); ok {
			val := float64(n)
			field.Min = &val
		}
	}
	if v := t.RawGetString("max"); v != lua.LNil {
		if n, ok := v.(lua.LNumber); ok {
			val := float64(n)
			field.Max = &val
		}
	}
	if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
		return fmt.Errorf("validation: field '%s' min (%v) must be <= max (%v)", field.Name, *field.Min, *field.Max)
	}
	if v := t.RawGetString("step"); v != lua.LNil {
		if n, ok := v.(lua.LNumber); ok {
			val := float64(n)
			if val <= 0 {
				return fmt.Errorf("validation: field '%s' step must be > 0", field.Name)
			}
			field.Step = &val
		}
	}
	return nil
}

// parseDateConstraints parses min, max for date fields.
func (f *FlowRuntime) parseDateConstraints(t *lua.LTable, field *Field) error {
	if v := t.RawGetString("min"); v != lua.LNil {
		field.MinDate = lua.LVAsString(v)
		if !isValidDateFormat(field.MinDate) {
			return fmt.Errorf("validation: field '%s' min date must be YYYY-MM-DD format", field.Name)
		}
	}
	if v := t.RawGetString("max"); v != lua.LNil {
		field.MaxDate = lua.LVAsString(v)
		if !isValidDateFormat(field.MaxDate) {
			return fmt.Errorf("validation: field '%s' max date must be YYYY-MM-DD format", field.Name)
		}
	}
	if field.MinDate != "" && field.MaxDate != "" && field.MinDate > field.MaxDate {
		return fmt.Errorf("validation: field '%s' min date (%s) must be <= max date (%s)", field.Name, field.MinDate, field.MaxDate)
	}
	return nil
}

// parseOptions parses and validates options for select fields.
func (f *FlowRuntime) parseOptions(t *lua.LTable, fieldName string) ([]SelectOption, error) {
	var options []SelectOption
	seenValues := make(map[string]bool)
	count := 0

	var parseErr error
	t.ForEach(func(_, v lua.LValue) {
		if parseErr != nil {
			return
		}

		count++
		if count > maxOptionsCount {
			parseErr = fmt.Errorf("validation: field '%s' exceeds max options count %d", fieldName, maxOptionsCount)
			return
		}

		optTable, ok := v.(*lua.LTable)
		if !ok {
			parseErr = fmt.Errorf("validation: field '%s' option must be a {value, label} tuple", fieldName)
			return
		}

		// Options are {value, label} tuples (array indices 1, 2)
		valueVal := optTable.RawGetInt(1)
		labelVal := optTable.RawGetInt(2)

		if valueVal == lua.LNil || labelVal == lua.LNil {
			parseErr = fmt.Errorf("validation: field '%s' option must be a {value, label} tuple", fieldName)
			return
		}

		value := lua.LVAsString(valueVal)
		label := lua.LVAsString(labelVal)

		if value == "" {
			parseErr = fmt.Errorf("validation: field '%s' option value cannot be empty", fieldName)
			return
		}
		if strings.ContainsRune(value, 0) {
			parseErr = fmt.Errorf("validation: field '%s' option value contains null byte", fieldName)
			return
		}
		if label == "" {
			parseErr = fmt.Errorf("validation: field '%s' option label cannot be empty", fieldName)
			return
		}
		if seenValues[value] {
			parseErr = fmt.Errorf("validation: field '%s' has duplicate option value '%s'", fieldName, value)
			return
		}
		seenValues[value] = true

		options = append(options, SelectOption{Value: value, Label: label})
	})

	if parseErr != nil {
		return nil, parseErr
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("validation: field '%s' options cannot be empty", fieldName)
	}

	return options, nil
}

// parseActions parses and validates the actions array.
func (f *FlowRuntime) parseActions(t *lua.LTable) ([]Action, error) {
	var actions []Action
	seenIDs := make(map[string]bool)

	var parseErr error
	t.ForEach(func(_, v lua.LValue) {
		if parseErr != nil {
			return
		}

		actionTable, ok := v.(*lua.LTable)
		if !ok {
			parseErr = fmt.Errorf("validation: action must be a table")
			return
		}

		// Actions are {id, label, style?} tuples
		idVal := actionTable.RawGetInt(1)
		labelVal := actionTable.RawGetInt(2)
		styleVal := actionTable.RawGetInt(3)

		if idVal == lua.LNil {
			parseErr = fmt.Errorf("validation: action missing id")
			return
		}
		id := lua.LVAsString(idVal)
		if id == "" {
			parseErr = fmt.Errorf("validation: action id cannot be empty")
			return
		}

		if labelVal == lua.LNil {
			parseErr = fmt.Errorf("validation: action '%s' missing label", id)
			return
		}
		label := lua.LVAsString(labelVal)
		if label == "" {
			parseErr = fmt.Errorf("validation: action '%s' label cannot be empty", id)
			return
		}

		if seenIDs[id] {
			parseErr = fmt.Errorf("validation: duplicate action id '%s'", id)
			return
		}
		seenIDs[id] = true

		action := Action{ID: id, Label: label}

		if styleVal != lua.LNil {
			style := lua.LVAsString(styleVal)
			validStyles := map[string]bool{"primary": true, "warning": true, "danger": true}
			if style != "" && !validStyles[style] {
				parseErr = fmt.Errorf("validation: action '%s' has invalid style '%s'", id, style)
				return
			}
			action.Style = style
		}

		actions = append(actions, action)
	})

	if parseErr != nil {
		return nil, parseErr
	}

	if len(actions) == 0 {
		return nil, fmt.Errorf("validation: actions cannot be empty")
	}

	return actions, nil
}

// eventToTable converts an Event to a Lua table.
func (f *FlowRuntime) eventToTable(e Event) *lua.LTable {
	t := f.l.NewTable()
	t.RawSetString("action", lua.LString(e.Action))

	if e.Data != nil {
		dataTable := f.l.NewTable()
		for k, v := range e.Data {
			dataTable.RawSetString(k, GoToLuaValue(f.l, v))
		}
		t.RawSetString("data", dataTable)
	}

	return t
}

// validateEventData checks that event data only contains field names from the screen spec.
func validateEventData(event Event, screen Screen) error {
	if event.Data == nil {
		return nil
	}
	expected := make(map[string]bool, len(screen.Fields))
	for _, f := range screen.Fields {
		if f.Type != "markdown" && f.Name != "" {
			expected[f.Name] = true
		}
	}
	for k := range event.Data {
		if !expected[k] {
			return fmt.Errorf("unexpected field '%s' in event data", k)
		}
	}
	return nil
}

// titleCase converts a snake_case name to Title Case.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	// Simple implementation: capitalize first letter, replace _ with space
	result := strings.ReplaceAll(s, "_", " ")
	if result != "" {
		first, size := utf8.DecodeRuneInString(result)
		if first != utf8.RuneError {
			result = strings.ToUpper(string(first)) + result[size:]
		}
	}
	return result
}

// isValidDateFormat checks if a string is in YYYY-MM-DD format and represents a valid date.
func isValidDateFormat(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
