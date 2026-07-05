package cli

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclaudit"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// ACLCmd groups access-control commands.
type ACLCmd struct {
	Audit ACLAuditCmd `cmd:"" help:"Audit the ACL policy (acl.yaml) for misconfigurations."`
}

// ACLAuditCmd runs the on-demand authorization-misconfiguration linter over the
// project's acl.yaml. It is advisory by default: it prints findings and exits 0.
// For CI, --fail-on=<severity> (or the --exit-code alias) makes the command exit
// non-zero when a finding at or above that severity is present. See
// internal/aclaudit (TKT-TS0J5K).
type ACLAuditCmd struct {
	// FailOn gates the exit code. Empty = never fail (advisory). A severity
	// label (critical|high|medium|low|nit) or "any" fails when a finding at or
	// above that level is present. "any" == "nit" (every finding).
	FailOn   string `help:"Exit non-zero when a finding at or above this severity is present: critical|high|medium|low|any. Empty = advisory (always exit 0)." enum:",critical,high,medium,low,any" default:""`
	ExitCode bool   `help:"Alias for --fail-on=high (fail CI on critical or high findings)."`
}

// Run executes `rela acl audit`.
func (c *ACLAuditCmd) Run(svc *cliServices) error {
	threshold, gate, err := c.resolveFailOn()
	if err != nil {
		return err
	}

	policyPath := filepath.Join(svc.Paths().Root, "acl.yaml")
	policy, err := acl.LoadPolicy(policyPath)
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
			out.WriteSuccess("No acl.yaml found; nothing to audit (the project has no access-control policy).")
			return nil
		}
		return fmt.Errorf("load acl.yaml: %w", err)
	}

	findings := aclaudit.Audit(policy, &metamodelReader{m: svc.Meta()})

	if out.Format == "json" {
		writeAuditJSON(findings)
	} else {
		writeAuditText(findings)
	}

	if gate && aclaudit.HasAtLeast(findings, threshold) {
		return errors.NewExitError(1)
	}
	return nil
}

// resolveFailOn computes the exit-code gate from --fail-on / --exit-code.
// Returns (threshold, gateEnabled, error). When gateEnabled is false the
// command is advisory and always exits 0. --exit-code is sugar for
// --fail-on=high; if both are set, --fail-on wins (the explicit threshold).
func (c *ACLAuditCmd) resolveFailOn() (aclaudit.Severity, bool, error) {
	if c.FailOn != "" {
		threshold, ok := aclaudit.ParseSeverity(c.FailOn)
		if !ok {
			return 0, false, fmt.Errorf("invalid --fail-on %q: want one of critical, high, medium, low, any", c.FailOn)
		}
		return threshold, true, nil
	}
	if c.ExitCode {
		return aclaudit.High, true, nil
	}
	return 0, false, nil
}

// writeAuditText renders findings as human-readable lines grouped by severity.
func writeAuditText(findings []aclaudit.Finding) {
	if len(findings) == 0 {
		out.WriteSuccess("ACL audit: no findings.")
		return
	}
	out.WriteWarning("ACL audit: %d finding(s)", len(findings))
	for _, f := range findings {
		out.WriteMessage("  [%s] %s (%s: %s)", f.Severity, f.Detail, f.Rule, f.Subject)
		if f.Fix != "" {
			out.WriteMessage("      fix: %s", f.Fix)
		}
	}
}

// auditFindingJSON is the per-finding JSON shape (severity as a label, not the
// internal int).
type auditFindingJSON struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Subject  string `json:"subject"`
	Detail   string `json:"detail"`
	Fix      string `json:"fix"`
}

// writeAuditJSON emits findings via the shared AnalysisResult envelope.
func writeAuditJSON(findings []aclaudit.Finding) {
	details := make([]auditFindingJSON, 0, len(findings))
	for _, f := range findings {
		details = append(details, auditFindingJSON{
			Rule: f.Rule, Severity: f.Severity.String(), Subject: f.Subject,
			Detail: f.Detail, Fix: f.Fix,
		})
	}
	status, message := "success", "ACL audit: no findings"
	if len(findings) > 0 {
		status = "warning"
		message = fmt.Sprintf("ACL audit: %d finding(s)", len(findings))
	}
	_ = out.WriteAnalysisResult(output.AnalysisResult{
		Status: status, Message: message, Count: len(findings), Details: details,
	})
}

// metamodelReader adapts *metamodel.Metamodel to aclaudit.MetamodelReader. It
// lives here (the consumer side) rather than in internal/aclaudit, so aclaudit
// stays free of a metamodel dependency and bounded to the narrow lookups the
// audit actually uses.
type metamodelReader struct {
	m *metamodel.Metamodel
}

func (r *metamodelReader) HasEntityType(t string) bool {
	if r.m == nil {
		return false
	}
	return r.m.HasEntityType(t)
}

func (r *metamodelReader) GetRelation(name string) (aclaudit.RelationView, bool) {
	if r.m == nil {
		return aclaudit.RelationView{}, false
	}
	def, ok := r.m.GetRelationDef(name)
	if !ok {
		return aclaudit.RelationView{}, false
	}
	return aclaudit.RelationView{From: def.From}, true
}

func (r *metamodelReader) HasField(t, field string) bool {
	if r.m == nil {
		return false
	}
	def, ok := r.m.GetEntityDef(t)
	if !ok {
		return false
	}
	_, ok = def.Properties[field]
	return ok
}

// EnumOptions returns the allowed values for an enum field on type t. A field
// is an enum either via inline `values:` or by referencing a custom enum type
// declared under the metamodel's top-level `types:`.
func (r *metamodelReader) EnumOptions(t, field string) ([]string, bool) {
	if r.m == nil {
		return nil, false
	}
	def, ok := r.m.GetEntityDef(t)
	if !ok {
		return nil, false
	}
	prop, ok := def.Properties[field]
	if !ok {
		return nil, false
	}
	if len(prop.Values) > 0 {
		return sortedClone(prop.Values), true
	}
	if ct, ok := r.m.Types[prop.Type]; ok && len(ct.Values) > 0 {
		return sortedClone(ct.Values), true
	}
	return nil, false
}

// sortedClone returns a sorted copy so the audit's "outside the enum" message
// is deterministic regardless of metamodel iteration.
func sortedClone(xs []string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}
