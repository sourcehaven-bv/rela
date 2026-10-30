// Package check runs operator-registered prose checkers (the metamodel's
// `checkers:` registry) over entity bodies and returns their findings in one
// shape, whatever tool produced them.
//
// A checker is an external command run through internal/cmdexec, so it gets
// the same confinement as transforms: argv only, no network, a timeout, an
// output cap and a bounded pool. The body is written to `<type>/<id>.md` and
// the entity type is available as `{type}`, so the checker's own config
// selects per-type rules; rela keeps no per-type rule lists. The type
// directory keeps the layout the same when a run checks several bodies.
//
// Findings are advisory. Nothing here is on the write path.
package check

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TemplateType is the argv placeholder replaced by the entity type name.
const TemplateType = "{type}"

// fallbackName names the input file when the entity id is not a safe file
// name (or there is none, as for a body not yet saved).
const fallbackName = "body"

// Execution bounds. A check runs while an editor types, so the pool is small
// and separate from export: a burst of checks must not queue exports, and the
// reverse. Per-checker timeouts come from the registry and only shorten
// maxTimeout.
const (
	maxTimeout       = metamodel.MaxCheckerTimeout
	defaultMaxBytes  = 4 << 20
	defaultMaxActive = 2
)

// Severity of a finding.
const (
	SeverityError      = "error"
	SeverityWarning    = "warning"
	SeveritySuggestion = "suggestion"
)

// Finding is one checker result. Line and Column are 1-based; Column and
// Length count code points. A finding whose position does not fit the body
// keeps its message with Line, Column and Length 0, so a caller can still show
// it without anchoring it.
type Finding struct {
	Checker      string   `json:"checker"`
	Rule         string   `json:"rule"`
	Severity     string   `json:"severity"`
	Message      string   `json:"message"`
	Line         int      `json:"line"`
	Column       int      `json:"column"`
	Length       int      `json:"length"`
	Match        string   `json:"match,omitempty"`
	Replacements []string `json:"replacements,omitempty"`
	Link         string   `json:"link,omitempty"`
}

// CheckerError reports a checker that failed to run or produced output that
// could not be parsed. The other checkers' findings are still returned.
type CheckerError struct {
	Checker string
	Err     error
}

func (e CheckerError) Error() string { return fmt.Sprintf("checker %q: %v", e.Checker, e.Err) }

func (e CheckerError) Unwrap() error { return e.Err }

// Result is the outcome of checking one body with every given checker.
type Result struct {
	Findings []Finding
	Errors   []CheckerError
}

// Engine runs checkers. It owns the bounded pool, so it MUST be built once and
// shared (see .claude/rules/transforms.md for why). It holds no registry: each
// call passes the checkers, so a metamodel reload needs no rebuild.
type Engine struct {
	runner *cmdexec.Runner
}

// NewEngine builds an engine with the package's execution bounds.
func NewEngine() *Engine {
	runner, err := cmdexec.New(maxTimeout, defaultMaxBytes,
		cmdexec.WithMaxConcurrent(defaultMaxActive), cmdexec.WithPurpose(cmdexec.PurposeCheck))
	if err != nil {
		// Unreachable: cmdexec.New fails only on non-positive bounds, and the
		// bounds are package constants.
		panic("check: " + err.Error())
	}
	return &Engine{runner: runner}
}

// Describe reports the confinement posture for the startup log.
func (e *Engine) Describe() string { return e.runner.Describe() }

// Probe reports a checker whose binary is not on PATH, so a missing tool is a
// startup warning rather than a surprise on the first check.
func (e *Engine) Probe(def metamodel.CheckerDef) error { return e.runner.Probe(def.Command) }

// Check runs every checker in checkers over body, in name order, and returns
// the findings sorted by position. The input file is `<typeName>/<id>.md`;
// an id that is not a safe file name becomes "body". An empty or
// whitespace-only body runs nothing. typeName must match
// [metamodel.CheckableTypeName]; the metamodel enforces that at load whenever
// checkers are declared.
func (e *Engine) Check(
	ctx context.Context, checkers map[string]metamodel.CheckerDef, typeName, id string, body []byte,
) (Result, error) {
	if !metamodel.CheckableTypeName.MatchString(typeName) {
		return Result{}, fmt.Errorf("check: entity type %q is not a safe input name", typeName)
	}
	name := id
	if !metamodel.CheckableTypeName.MatchString(name) {
		name = fallbackName
	}
	inputName := typeName + "/" + name + ".md"
	var res Result
	if strings.TrimSpace(string(body)) == "" || len(checkers) == 0 {
		return res, nil
	}
	names := make([]string, 0, len(checkers))
	for checker := range checkers {
		names = append(names, checker)
	}
	sort.Strings(names)

	for _, checker := range names {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		findings, err := e.run(ctx, checker, checkers[checker], typeName, inputName, body)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
				return res, ctxErr
			}
			res.Errors = append(res.Errors, CheckerError{Checker: checker, Err: err})
			continue
		}
		res.Findings = append(res.Findings, findings...)
	}
	sortFindings(res.Findings)
	return res, nil
}

func (e *Engine) run(
	ctx context.Context, name string, def metamodel.CheckerDef, typeName, inputName string, body []byte,
) ([]Finding, error) {
	argv := make([]string, len(def.Command))
	for i, a := range def.Command {
		argv[i] = strings.ReplaceAll(a, TemplateType, typeName)
	}
	out, err := e.runner.RunWith(ctx, argv, body, cmdexec.RunSpec{
		InputName:         inputName,
		AcceptedExitCodes: def.AcceptedExitCodes(),
		Timeout:           min(def.TimeoutDuration(), maxTimeout),
	})
	if err != nil {
		return nil, err
	}
	var raw []rawFinding
	switch def.Format {
	case metamodel.CheckerFormatVale:
		raw, err = parseVale(out.Output)
	case metamodel.CheckerFormatRela:
		raw, err = parseRela(out.Output)
	default:
		// Unreachable: the metamodel rejects unknown formats at load.
		err = fmt.Errorf("unsupported format %q", def.Format)
	}
	if err != nil {
		return nil, err
	}
	return resolve(name, raw, body), nil
}

// AnalyzeCheckers returns the checkers that opted into analyze runs.
func AnalyzeCheckers(m *metamodel.Metamodel) map[string]metamodel.CheckerDef {
	if m == nil {
		return nil
	}
	out := make(map[string]metamodel.CheckerDef)
	for name, def := range m.Checkers {
		if def.Analyze {
			out[name] = def
		}
	}
	return out
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return a.Checker < b.Checker
	})
}
