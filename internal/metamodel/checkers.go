package metamodel

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Checker output formats. Kept as local constants (rather than importing
// internal/check) so the dependency stays one-way: check imports metamodel.
const (
	CheckerFormatVale = "vale"
	CheckerFormatRela = "rela"
)

// Checker timeout bounds. A checker runs while an editor types, so the default
// is short; the cap keeps a misconfigured entry from holding a pool slot.
const (
	DefaultCheckerTimeout = 10 * time.Second
	MaxCheckerTimeout     = 60 * time.Second
)

// CheckableTypeName is the allowlist an entity type name must match when any
// checker is declared: the name becomes the checker's input file name
// directory (`<type>/<id>.md`) and a glob in the checker's own config, so it must be a single
// path segment with no glob characters. The general name check
// ([ValidateSchemaName]) is a blocklist and allows `/`, spaces and `[*?{`.
var CheckableTypeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// CheckerDef is one entry in the top-level `checkers:` registry: an external
// prose checker (spelling, grammar, style) run over an entity's body. Command
// is an argv array; `{in}` is the body file at `<type>/<id>.md`, and `{type}`
// is the entity type. Commands come from project config (this file), never
// from a request.
type CheckerDef struct {
	Command []string `yaml:"command"`
	// Format names the output parser: "vale" or "rela" (native JSON).
	Format string `yaml:"format"`
	// ExitCodes lists the exit codes that still mean "findings on stdout".
	// Empty means [0]; 0 is always accepted.
	ExitCodes []int `yaml:"exit_codes,omitempty"`
	// Timeout is a Go duration ("10s"). Empty means [DefaultCheckerTimeout].
	Timeout string `yaml:"timeout,omitempty"`
	// Analyze reports this checker's findings from analyze runs that ask for
	// prose checks. Editor checks run regardless.
	Analyze bool `yaml:"analyze,omitempty"`
}

// TimeoutDuration returns the checker's timeout, defaulted. Load validation
// guarantees the value parses and lies within the cap.
func (d CheckerDef) TimeoutDuration() time.Duration {
	if d.Timeout == "" {
		return DefaultCheckerTimeout
	}
	t, err := time.ParseDuration(d.Timeout)
	if err != nil {
		return DefaultCheckerTimeout
	}
	return t
}

// AcceptedExitCodes returns the exit codes that count as success, defaulted.
func (d CheckerDef) AcceptedExitCodes() []int {
	if len(d.ExitCodes) == 0 {
		return []int{0}
	}
	return d.ExitCodes
}

// validateCheckers checks the top-level `checkers:` registry. Like
// validateTransforms, a bad entry fails the whole load. When any checker is
// declared, every entity type name must match [CheckableTypeName].
func validateCheckers(m *Metamodel) []string {
	if len(m.Checkers) == 0 {
		return nil
	}
	var errs []string

	names := make([]string, 0, len(m.Checkers))
	for name := range m.Checkers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		def := m.Checkers[name]
		if strings.TrimSpace(name) == "" {
			errs = append(errs, "checker has an empty name")
			continue
		}
		if len(def.Command) == 0 {
			errs = append(errs, fmt.Sprintf("checker %q: command is required", name))
		} else if strings.TrimSpace(def.Command[0]) == "" {
			errs = append(errs, fmt.Sprintf("checker %q: command[0] (the binary) is empty", name))
		}
		switch def.Format {
		case CheckerFormatVale, CheckerFormatRela:
		case "":
			errs = append(errs, fmt.Sprintf("checker %q: format is required (%q or %q)",
				name, CheckerFormatVale, CheckerFormatRela))
		default:
			errs = append(errs, fmt.Sprintf("checker %q: unsupported format %q (want %q or %q)",
				name, def.Format, CheckerFormatVale, CheckerFormatRela))
		}
		for _, c := range def.ExitCodes {
			if c < 0 || c > 255 {
				errs = append(errs, fmt.Sprintf("checker %q: exit code %d is out of range 0-255", name, c))
			}
		}
		if def.Timeout != "" {
			t, err := time.ParseDuration(def.Timeout)
			switch {
			case err != nil:
				errs = append(errs, fmt.Sprintf("checker %q: invalid timeout %q: %v", name, def.Timeout, err))
			case t <= 0 || t > MaxCheckerTimeout:
				errs = append(errs, fmt.Sprintf("checker %q: timeout %s must be above 0 and at most %s",
					name, t, MaxCheckerTimeout))
			}
		}
	}

	types := make([]string, 0, len(m.Entities))
	for name := range m.Entities {
		types = append(types, name)
	}
	sort.Strings(types)
	for _, name := range types {
		if !CheckableTypeName.MatchString(name) {
			errs = append(errs, fmt.Sprintf(
				"entity type %q: with checkers declared, type names must match %s (the name becomes the checker's input file name)",
				name, CheckableTypeName))
		}
	}
	return errs
}
