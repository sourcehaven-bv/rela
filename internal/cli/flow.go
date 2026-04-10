package cli

import (
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
	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var flowOutputDir string

var flowCmd = &cobra.Command{
	Use:         "flow <script.lua> [args...]",
	Short:       "Run an interactive Lua flow",
	Annotations: map[string]string{skipProjectDiscovery: "true"},
	Long: `Run a Lua script that can present interactive forms to the user.

Flow scripts use rela.flow.emit() to present forms and receive user input.
The script suspends at each emit() call until the user responds.

Field types: text, select, multi-select, boolean, number, date, markdown

Example:
  local event = rela.flow.emit({
    type = "form",
    title = "Create Ticket",
    fields = {
      {name = "title", type = "text", required = true},
      {name = "priority", type = "select",
       options = {{"high", "High"}, {"low", "Low"}}},
    },
    actions = {{"submit", "Create"}, {"cancel", "Cancel"}},
  })
  if event.action == "cancel" then return end
  rela.create_entity("ticket", event.data)`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptPath := args[0]
		scriptArgs := args[1:]

		// Resolve script path to absolute
		if !filepath.IsAbs(scriptPath) {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			scriptPath = filepath.Join(cwd, scriptPath)
		}

		// Discover project from script location (walk up from script's directory)
		// This allows shebang scripts to work from any directory
		scriptDir := filepath.Dir(scriptPath)
		flowWs, err := workspace.Discover(scriptDir, script.NewEngine())
		if err != nil {
			return fmt.Errorf("no project found for script %s", scriptPath)
		}

		opts := []lua.Option{lua.WithContext(cmd.Context())}
		if flowOutputDir != "" {
			opts = append(opts, lua.WithOutputDir(flowOutputDir))
		}
		// AI is often the whole point of running a flow script, so a
		// misconfigured ai.yaml should surface immediately rather
		// than silently disable AI. ErrConfigNotFound is the normal
		// "no AI" state and is not propagated.
		provider, providerErr := ai.LoadProvider(flowWs.Paths().CacheDir)
		switch {
		case errors.Is(providerErr, ai.ErrConfigNotFound):
			// no AI configured
		case providerErr != nil:
			return fmt.Errorf("ai: %w", providerErr)
		default:
			opts = append(opts, lua.WithAIProvider(provider))
		}

		runtime := lua.New(flowWs, flowWs.Meta(), flowWs.Paths().Root, os.Stdout, opts...)
		defer runtime.Close()

		transport := &TerminalTransport{}
		flow := lua.NewFlowRuntime(runtime, transport)

		return flow.RunFile(scriptPath, scriptArgs)
	},
}

func init() {
	flowCmd.Flags().StringVar(&flowOutputDir, "output-dir", "",
		"Directory for write_file output (default: {project}/output)")
	rootCmd.AddCommand(flowCmd)
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

// fieldValue tracks a form field's value pointer and its type for correct extraction.
type fieldValue struct {
	ptr       any
	fieldType string
}

func (t *TerminalTransport) presentForm(screen lua.Screen) (lua.Event, error) {
	// Build huh form fields
	var groups []*huh.Group
	fieldValues := make(map[string]fieldValue)

	fields := make([]huh.Field, 0, len(screen.Fields)+1) // +1 for action select
	for _, f := range screen.Fields {
		field, valuePtr, err := t.buildField(f)
		if err != nil {
			return lua.Event{}, err
		}
		fields = append(fields, field)
		// Only track data-collecting fields (markdown fields return nil valuePtr)
		if valuePtr != nil {
			fieldValues[f.Name] = fieldValue{ptr: valuePtr, fieldType: f.Type}
		}
	}

	// Add action buttons as a select
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

	// Build and run form
	form := huh.NewForm(groups...)

	// Run with accessible mode for better compatibility
	form = form.WithAccessible(false)

	// Print title and description before the form
	if screen.Title != "" {
		fmt.Fprintf(os.Stdout, "\n%s\n", screen.Title)
	}
	if screen.Description != "" {
		fmt.Fprintf(os.Stdout, "%s\n", screen.Description)
	}

	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return lua.Event{}, fmt.Errorf("user interrupted")
		}
		return lua.Event{}, err
	}

	// Build result data
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
	// Render markdown content using glamour
	rendered, err := glamour.Render(f.Content, "auto")
	if err != nil {
		// Fall back to raw content if rendering fails
		rendered = f.Content
	}
	// Trim trailing newlines that glamour adds
	rendered = strings.TrimRight(rendered, "\n")

	note := huh.NewNote().Description(rendered)
	if f.Label != "" {
		note = note.Title(f.Label)
	}
	// Return nil for valuePtr since markdown fields don't collect data
	return note, nil, nil
}

func (t *TerminalTransport) buildTextField(f lua.Field) (huh.Field, any, error) {
	var value string
	if f.Default != nil {
		value = fmt.Sprintf("%v", f.Default)
	}

	if f.Lines > 1 {
		// Textarea for multiline
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

	// Single line input
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
			return fmt.Errorf("must be a number")
		}

		if math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("invalid number")
		}

		if f.Min != nil && n < *f.Min {
			return fmt.Errorf("must be at least %v", *f.Min)
		}
		if f.Max != nil && n > *f.Max {
			return fmt.Errorf("must be at most %v", *f.Max)
		}
		if f.Step != nil {
			// Check that (n - base) is a multiple of step, where base is min or 0
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
			return fmt.Errorf("must be YYYY-MM-DD format")
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
		// Only convert to number for number-typed fields
		if fv.fieldType == "number" {
			if n, err := strconv.ParseFloat(*v, 64); err == nil {
				return n
			}
		}
		return *v
	case *[]string:
		// Convert to []any for Lua
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
