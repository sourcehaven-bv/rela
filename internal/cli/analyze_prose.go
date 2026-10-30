package cli

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/check"
	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AnalyzeProseCmd runs the prose checkers that opted into analyze runs.
type AnalyzeProseCmd struct {
	Type string `help:"Only check entities of this type."`
}

// Run dispatches `rela analyze prose`.
func (c *AnalyzeProseCmd) Run(ctx context.Context, svc *readServices) error {
	report, err := collectProse(ctx, svc, c.Type)
	if err != nil {
		return err
	}
	if out.Format == "json" {
		if jsonErr := writeProseJSON(report); jsonErr != nil {
			return jsonErr
		}
		return report.err()
	}
	writeProseText(report)
	return report.err()
}

// proseEntity is one entity's prose-check outcome.
type proseEntity struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Face     string          `json:"face,omitempty"`
	Findings []check.Finding `json:"findings,omitempty"`
	Errors   []string        `json:"errors,omitempty"`
}

// proseReport is the outcome of a prose analysis. Configured is false when no
// checker has `analyze: true`, so nothing ran.
type proseReport struct {
	Configured bool
	Entities   []proseEntity // only entities with findings or errors
	Findings   int
	Errors     int
}

// err surfaces checker failures as the command's error after the report is
// written: a checker that could not run is not "no findings".
func (r proseReport) err() error {
	if r.Errors == 0 {
		return nil
	}
	return fmt.Errorf("prose analysis: %d checker run(s) failed", r.Errors)
}

// collectProse runs every analyze-enabled checker over the body of each
// entity the CLI's world selects, optionally of one type.
func collectProse(ctx context.Context, svc *readServices, typeName string) (proseReport, error) {
	checkers := check.AnalyzeCheckers(svc.Meta)
	if len(checkers) == 0 {
		return proseReport{}, nil
	}
	if typeName != "" {
		if _, ok := svc.Meta.Entities[typeName]; !ok {
			return proseReport{}, fmt.Errorf("unknown entity type %q", typeName)
		}
	}
	if err := cmdexec.CheckProjectNotExposed(svc.Paths.Root); err != nil {
		return proseReport{}, err
	}
	engine := check.NewEngine()
	// A missing binary would fail on every entity; say so once instead.
	for _, name := range slices.Sorted(maps.Keys(checkers)) {
		if err := engine.Probe(checkers[name]); err != nil {
			return proseReport{}, fmt.Errorf("checker %q: %w", name, err)
		}
	}
	report := proseReport{Configured: true}
	q := store.EntityQuery{Type: typeName, Faces: store.InWorld(svc.World)}
	for e, err := range svc.Store.ListEntities(ctx, q) {
		if err != nil {
			return report, fmt.Errorf("list entities: %w", err)
		}
		res, err := engine.Check(ctx, checkers, e.Type, e.ID, []byte(e.Content))
		if err != nil {
			return report, fmt.Errorf("check %s: %w", e.ID, err)
		}
		if len(res.Findings) == 0 && len(res.Errors) == 0 {
			continue
		}
		pe := proseEntity{ID: e.ID, Type: e.Type, Face: string(e.Face), Findings: res.Findings}
		for _, ce := range res.Errors {
			pe.Errors = append(pe.Errors, ce.Error())
		}
		report.Findings += len(res.Findings)
		report.Errors += len(res.Errors)
		report.Entities = append(report.Entities, pe)
	}
	sort.Slice(report.Entities, func(i, j int) bool {
		return natsort.Less(report.Entities[i].ID, report.Entities[j].ID)
	})
	return report, nil
}

// proseCoverage states what the prose report covers: the face the CLI's
// world selects for each id, not every face.
const proseCoverage = "the world's face of each entity"

func writeProseJSON(r proseReport) error {
	if !r.Configured {
		return writeNoProseCheckers()
	}
	// A failed checker run makes the command fail, so the status says so too.
	status, message := "success", "No prose findings"
	switch {
	case r.Errors > 0:
		status = "error"
		message = fmt.Sprintf("Found %d prose finding(s); %d checker run(s) failed", r.Findings, r.Errors)
	case r.Findings > 0:
		status = "warning"
		message = fmt.Sprintf("Found %d prose finding(s)", r.Findings)
	}
	return out.WriteAnalysisResult(output.AnalysisResult{
		Status: status, Message: message, Coverage: proseCoverage, Count: r.Findings + r.Errors, Details: r.Entities,
	})
}

func writeNoProseCheckers() error {
	if out.Format == "json" {
		return out.WriteAnalysisResult(output.AnalysisResult{
			Status: "success", Message: "No checkers with analyze: true configured",
		})
	}
	out.WriteInfo("No checkers with analyze: true configured")
	return nil
}

func writeProseText(r proseReport) {
	if !r.Configured {
		_ = writeNoProseCheckers()
		return
	}
	out.WriteMessage("Coverage: %s", proseCoverage)
	for _, pe := range r.Entities {
		addr := pe.ID
		if pe.Face != "" {
			addr += "@" + pe.Face
		}
		out.WriteMessage("%s (%s)", addr, pe.Type)
		for _, f := range pe.Findings {
			out.WriteMessage("  %s", formatProseFinding(f))
		}
		for _, e := range pe.Errors {
			out.WriteError("  %s", e)
		}
	}
	switch {
	case r.Findings == 0 && r.Errors == 0:
		out.WriteSuccess("No prose findings")
	case r.Errors == 0:
		out.WriteWarning("Found %d prose finding(s) in %d entities", r.Findings, len(r.Entities))
	default:
		out.WriteWarning("Found %d prose finding(s); %d checker run(s) failed", r.Findings, r.Errors)
	}
}

func formatProseFinding(f check.Finding) string {
	pos := "-"
	if f.Line > 0 {
		pos = fmt.Sprintf("%d:%d", f.Line, f.Column)
	}
	s := fmt.Sprintf("%-7s %-10s %s  %s", pos, f.Severity, f.Rule, f.Message)
	if len(f.Replacements) > 0 {
		s += " (use: " + strings.Join(f.Replacements, ", ") + ")"
	}
	return s
}
