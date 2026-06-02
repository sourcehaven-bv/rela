package cli

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// FlowCmd runs an interactive Lua flow. Self-discovers project from
// the script's directory so shebang scripts work from any cwd.
type FlowCmd struct {
	OutputDir string   `name:"output-dir" help:"Directory for write_file output (default: {project}/output)."`
	Script    string   `arg:"" help:"Path to Lua script."`
	Args      []string `arg:"" optional:"" help:"Arguments passed to the script."`
}

// Run dispatches `rela flow <script.lua> [args...]`.
func (c *FlowCmd) Run(ctx context.Context) error {
	scriptPath := c.Script
	if !filepath.IsAbs(scriptPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		scriptPath = filepath.Join(cwd, scriptPath)
	}

	scriptDir := filepath.Dir(scriptPath)
	//nolint:contextcheck // appbuild.Discover does not take ctx; matches rela-server bootstrap
	flowSvc, err := appbuild.Discover(scriptDir, script.NewEngine())
	if err != nil {
		return fmt.Errorf("no project found for script %s", scriptPath)
	}

	opts := []lua.Option{
		lua.WithContext(ctx),
		lua.WithCache(flowSvc.ScriptEngine().LuaCache()),
	}
	if c.OutputDir != "" {
		opts = append(opts, lua.WithOutputDir(c.OutputDir))
	}

	runtime, rtErr := script.NewWriterRuntime(flowSvc.LuaWriteDeps(),
		scriptPath, os.Stdout, opts...)
	if rtErr != nil {
		return rtErr
	}
	defer runtime.Close()

	transport := &TerminalTransport{}
	flow := lua.NewFlowRuntime(runtime, transport)

	return flow.RunFile(scriptPath, c.Args)
}

// TerminalTransport implements lua.Transport using charmbracelet/huh.
type TerminalTransport struct{}

// Present displays a screen using huh and returns the user's response.
func (t *TerminalTransport) Present(screen lua.Screen) (lua.Event, error) {
	if screen.Type != "form" {
		return lua.Event{}, fmt.Errorf("unsupported screen type: %s", screen.Type)
	}
	return t.presentForm(screen)
}

type fieldValue struct {
	ptr       any
	fieldType string
}

func (t *TerminalTransport) presentForm(screen lua.Screen) (lua.Event, error) {
	groups := make([]*huh.Group, 0, 1)
	fieldValues := make(map[string]fieldValue)

	fields := make([]huh.Field, 0, len(screen.Fields)+1)
	for _, f := range screen.Fields {
		field, valuePtr, err := t.buildField(f)
		if err != nil {
			return lua.Event{}, err
		}
		fields = append(fields, field)
		if valuePtr != nil {
			fieldValues[f.Name] = fieldValue{ptr: valuePtr, fieldType: f.Type}
		}
	}

	var selectedAction string
	actionOptions := make([]huh.Option[string], 0, len(screen.Actions))
	for _, a := range screen.Actions {
		actionOptions = append(actionOptions, huh.NewOption(a.Label, a.ID))
	}

	actionSelect := huh.NewSelect[string]().
		Title("Action").
		Options(actionOptions...).
		Value(&selectedAction)

	fields = append(fields, actionSelect)
	groups = append(groups, huh.NewGroup(fields...))

	form := huh.NewForm(groups...)
	form = form.WithAccessible(false)

	if screen.Title != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", screen.Title)
	}
	if screen.Description != "" {
		fmt.Fprintf(os.Stdout, "%s\n", screen.Description)
	}

	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return lua.Event{}, errors.New("user interrupted")
		}
		return lua.Event{}, err
	}

	data := make(map[string]any)
	for name, fv := range fieldValues {
		data[name] = t.extractValue(fv)
	}

	return lua.Event{
		Action: selectedAction,
		Data:   data,
	}, nil
}

func (t *TerminalTransport) buildField(f lua.Field) (huh.Field, any, error) {
	switch f.Type {
	case "text":
		return t.buildTextField(f)
	case "select":
		return t.buildSelectField(f)
	case "multi-select":
		return t.buildMultiSelectField(f)
	case "boolean":
		return t.buildBooleanField(f)
	case "number":
		return t.buildNumberField(f)
	case "date":
		return t.buildDateField(f)
	case "markdown":
		return t.buildMarkdownField(f)
	default:
		return nil, nil, fmt.Errorf("unsupported field type: %s", f.Type)
	}
}

func (t *TerminalTransport) buildMarkdownField(f lua.Field) (huh.Field, any, error) {
	rendered, err := glamour.Render(f.Content, "auto")
	if err != nil {
		rendered = f.Content
	}
	rendered = strings.TrimRight(rendered, "\n")

	note := huh.NewNote().Description(rendered)
	if f.Label != "" {
		note = note.Title(f.Label)
	}
	return note, nil, nil
}

func (t *TerminalTransport) buildTextField(f lua.Field) (huh.Field, any, error) {
	var value string
	if f.Default != nil {
		value = fmt.Sprintf("%v", f.Default)
	}

	if f.Lines > 1 {
		field := huh.NewText().
			Title(f.Label).
			Value(&value)
		if f.Placeholder != "" {
			field = field.Placeholder(f.Placeholder)
		}
		if f.Required {
			field = field.Validate(makeRequiredValidator(f.Label))
		}
		return field, &value, nil
	}

	field := huh.NewInput().
		Title(f.Label).
		Value(&value)
	if f.Placeholder != "" {
		field = field.Placeholder(f.Placeholder)
	}
	if f.Required {
		field = field.Validate(makeRequiredValidator(f.Label))
	}
	return field, &value, nil
}

func (t *TerminalTransport) buildSelectField(f lua.Field) (huh.Field, any, error) {
	var value string
	if f.Default != nil {
		value = fmt.Sprintf("%v", f.Default)
	}

	options := make([]huh.Option[string], 0, len(f.Options))
	for _, opt := range f.Options {
		options = append(options, huh.NewOption(opt.Label, opt.Value))
	}

	field := huh.NewSelect[string]().
		Title(f.Label).
		Options(options...).
		Value(&value)
	return field, &value, nil
}

func (t *TerminalTransport) buildMultiSelectField(f lua.Field) (huh.Field, any, error) {
	var values []string
	if f.Default != nil {
		if arr, ok := f.Default.([]any); ok {
			values = make([]string, 0, len(arr))
			for _, v := range arr {
				values = append(values, fmt.Sprintf("%v", v))
			}
		}
	}

	options := make([]huh.Option[string], 0, len(f.Options))
	for _, opt := range f.Options {
		options = append(options, huh.NewOption(opt.Label, opt.Value))
	}

	field := huh.NewMultiSelect[string]().
		Title(f.Label).
		Options(options...).
		Value(&values)
	return field, &values, nil
}

func (t *TerminalTransport) buildBooleanField(f lua.Field) (huh.Field, any, error) {
	var value bool
	if f.Default != nil {
		if b, ok := f.Default.(bool); ok {
			value = b
		}
	}
	field := huh.NewConfirm().
		Title(f.Label).
		Value(&value)
	return field, &value, nil
}

func (t *TerminalTransport) buildNumberField(f lua.Field) (huh.Field, any, error) {
	var value string
	if f.Default != nil {
		value = fmt.Sprintf("%v", f.Default)
	}
	field := huh.NewInput().
		Title(f.Label).
		Value(&value).
		Validate(makeNumberValidator(f))
	return field, &value, nil
}

func (t *TerminalTransport) buildDateField(f lua.Field) (huh.Field, any, error) {
	var value string
	if f.Default != nil {
		value = fmt.Sprintf("%v", f.Default)
	}
	field := huh.NewInput().
		Title(f.Label).
		Placeholder("YYYY-MM-DD").
		Value(&value).
		Validate(makeDateValidator(f))
	return field, &value, nil
}

func makeRequiredValidator(label string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", label)
		}
		return nil
	}
}

func makeNumberValidator(f lua.Field) func(string) error {
	return func(s string) error {
		if s == "" {
			if f.Required {
				return fmt.Errorf("%s is required", f.Label)
			}
			return nil
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return errors.New("must be a number")
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return errors.New("invalid number")
		}
		if f.Min != nil && n < *f.Min {
			return fmt.Errorf("must be at least %v", *f.Min)
		}
		if f.Max != nil && n > *f.Max {
			return fmt.Errorf("must be at most %v", *f.Max)
		}
		if f.Step != nil {
			base := 0.0
			if f.Min != nil {
				base = *f.Min
			}
			remainder := math.Mod(n-base, *f.Step)
			if math.Abs(remainder) > 1e-9 && math.Abs(remainder-*f.Step) > 1e-9 {
				return fmt.Errorf("must be a multiple of %v", *f.Step)
			}
		}
		return nil
	}
}

func makeDateValidator(f lua.Field) func(string) error {
	return func(s string) error {
		if s == "" {
			if f.Required {
				return fmt.Errorf("%s is required", f.Label)
			}
			return nil
		}
		date, err := time.Parse("2006-01-02", s)
		if err != nil {
			return errors.New("must be YYYY-MM-DD format")
		}
		if f.MinDate != "" {
			minDate, _ := time.Parse("2006-01-02", f.MinDate)
			if date.Before(minDate) {
				return fmt.Errorf("must be on or after %s", f.MinDate)
			}
		}
		if f.MaxDate != "" {
			maxDate, _ := time.Parse("2006-01-02", f.MaxDate)
			if date.After(maxDate) {
				return fmt.Errorf("must be on or before %s", f.MaxDate)
			}
		}
		return nil
	}
}

func (t *TerminalTransport) extractValue(fv fieldValue) any {
	switch v := fv.ptr.(type) {
	case *string:
		if fv.fieldType == "number" {
			if n, err := strconv.ParseFloat(*v, 64); err == nil {
				return n
			}
		}
		return *v
	case *[]string:
		result := make([]any, len(*v))
		for i, s := range *v {
			result[i] = s
		}
		return result
	case *bool:
		return *v
	default:
		return nil
	}
}
