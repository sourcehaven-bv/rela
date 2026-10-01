package cli

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclaudit"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/classification"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// classificationAuditUser is the synthetic principal a role is evaluated as.
// It is never a real user: the policy copy it runs against assigns it exactly
// the role under analysis.
const classificationAuditUser = "classification-audit"

// classificationView is one principal shape the audit evaluates: a role, and
// optionally the client baseline and scopes it acts through.
type classificationView struct {
	role          string
	principalType string
	scopes        []string
}

func (v classificationView) String() string {
	s := "role " + v.role
	if v.principalType != "" {
		s += " as " + v.principalType
		if len(v.scopes) == 0 {
			s += " (no scopes)"
		} else {
			s += " (scopes: " + strings.Join(v.scopes, ", ") + ")"
		}
	}
	return s
}

// classificationViews enumerates the views to evaluate: every role (each
// also holds `everyone`, as every principal does), and every role through
// every client baseline with no scopes, each single scope, and all scopes.
func classificationViews(policy *acl.Policy) []classificationView {
	roles := sortedKeys(policy.Roles)
	scopes := sortedKeys(policy.ScopeGrants)
	var scopeSets [][]string
	scopeSets = append(scopeSets, nil)
	for _, s := range scopes {
		scopeSets = append(scopeSets, []string{s})
	}
	if len(scopes) > 1 {
		scopeSets = append(scopeSets, scopes)
	}

	var views []classificationView
	for _, role := range roles {
		views = append(views, classificationView{role: role})
		for _, name := range sortedKeys(policy.ClientBaselines) {
			b := policy.ClientBaselines[name]
			if len(b.AppliesTo) == 0 {
				continue // matches no principal; aclaudit reports it
			}
			for _, set := range scopeSets {
				views = append(views, classificationView{role: role, principalType: b.AppliesTo[0], scopes: set})
			}
		}
	}
	return views
}

// classificationViewFor computes what v could read, through the same ACL and
// affordance code a request runs. Every `when:` counts as passing
// ([affordances.WithStaticGrants]), so the view is the worst case.
func classificationViewFor(
	ctx context.Context, policy *acl.Policy, meta *metamodel.Metamodel, shape classification.Shape,
	v classificationView,
) (classification.View, error) {
	// A copy whose only assignment is the audit user holding v.role. The roles
	// themselves are shared, unchanged.
	p := *policy
	p.Assignments = map[string]string{}
	p.AssertedRoles = nil
	// The synthetic principal is already a user id; with no lookup to
	// resolve it, a policy using principal_property would be refused.
	p.PrincipalProperty = ""
	if v.role != acl.EveryoneRole {
		p.Assignments[classificationAuditUser] = v.role
	}
	decl, err := acl.NewDeclarative(&p, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		return classification.View{}, err
	}
	resolver, err := affordances.New(meta, noRelations{}, decl)
	if err != nil {
		return classification.View{}, err
	}
	pr := principal.VerifiedFrom(classificationAuditUser, principal.ToolCLI, principal.Claims{
		PrincipalType: v.principalType, Scopes: v.scopes,
	})
	req, err := decl.ForPrincipal(pr)
	if err != nil {
		return classification.View{}, err
	}
	ctx = affordances.WithStaticGrants(acl.WithRequest(principal.With(ctx, pr), req))

	view := classification.View{
		Entities: map[string][]string{}, Relations: map[string][]string{}, Ends: map[string][]classification.Ends{},
	}
	for _, t := range sortedKeys(shape.Entities) {
		if !req.ReadQuery(ctx, t).AllowAll {
			continue
		}
		hidden := resolver.FieldVerdicts(ctx, entity.New("", t)).Visible
		view.Entities[t] = visibleOf(shape.Entities[t].Fields, hidden)
	}
	for _, r := range sortedKeys(shape.Relations) {
		rs := shape.Relations[r]
		// Only properties are relation meta; the body is served with the
		// edge whatever `visible:` says, so it never enters the deny set.
		props := sortedKeys(meta.Relations[r].Properties)
		visible := map[string]bool{}
		for _, e := range rs.Ends {
			_, fromOK := view.Entities[e.From]
			_, toOK := view.Entities[e.To]
			if !fromOK || !toOK {
				continue
			}
			view.Ends[r] = append(view.Ends[r], e)
			// Relation `visible:` grants are keyed by the FROM type, so the
			// union over readable pairs is the worst case.
			hidden := resolver.RelationFieldVerdicts(ctx, entity.New("", e.From), r, props)
			for _, f := range visibleOf(rs.Fields, hidden) {
				visible[f] = true
			}
		}
		if len(view.Ends[r]) > 0 {
			view.Relations[r] = inOrder(rs.Fields, visible)
		}
	}
	return view, nil
}

// visibleOf returns fields minus the ones hidden holds (a deny map: presence
// means hidden). The body is not a property, so it is never in hidden and is
// readable with the row.
func visibleOf(fields []string, hidden map[string]bool) []string {
	var out []string
	for _, f := range fields {
		if _, denied := hidden[f]; !denied {
			out = append(out, f)
		}
	}
	return out
}

func inOrder(fields []string, keep map[string]bool) []string {
	var out []string
	for _, f := range fields {
		if keep[f] {
			out = append(out, f)
		}
	}
	return out
}

// noRelations answers every relation lookup with "no edges". The static
// audit has no entity, and a `when:` that consults edges passes anyway under
// [affordances.WithStaticGrants].
type noRelations struct{}

func (noRelations) OutgoingCounts(context.Context, string) map[string]int { return nil }
func (noRelations) HasEdge(context.Context, string, string, string) bool  { return false }

// Rule names for the audit's own notes about a view, beside the C1-C3
// exposure rules.
const (
	ruleSameAsRole      = "C0-same-as-role"
	ruleClientReadsNone = "C0-reads-none"
	ruleTruncated       = "C0-truncated"
)

// classificationFindings evaluates every view and converts its exposures into
// low-severity audit findings. Views come role first, then that role's
// client views.
//
// Two rules keep the list readable without making silence ambiguous:
//   - An exposure `everyone` has is listed only under role everyone, not
//     again under each role, since every principal holds that role.
//   - A client view that reads exactly what its role reads gets one
//     C0-same-as-role finding, one that reads nothing labeled gets
//     C0-reads-none, and any other lists all its exposures. So an exposure
//     missing under a listed client view is one the ceiling blocks.
func classificationFindings(
	ctx context.Context, policy *acl.Policy, meta *metamodel.Metamodel, f *classification.File,
) ([]aclaudit.Finding, error) {
	shape := classificationShape(meta)
	type evaluated struct {
		exposures []classification.Exposure
		truncated bool
	}
	evaluate := func(v classificationView) (evaluated, error) {
		view, err := classificationViewFor(ctx, policy, meta, shape, v)
		if err != nil {
			return evaluated{}, fmt.Errorf("evaluate %s: %w", v, err)
		}
		es, truncated := f.Exposures(shape, view)
		return evaluated{es, truncated}, nil
	}

	everyone := map[string]bool{}
	if _, ok := policy.Roles[acl.EveryoneRole]; ok {
		ev, err := evaluate(classificationView{role: acl.EveryoneRole})
		if err != nil {
			return nil, err
		}
		everyone = exposureKeys(ev.exposures)
	}

	var out []aclaudit.Finding
	var roleKeys map[string]bool
	for _, v := range classificationViews(policy) {
		ev, err := evaluate(v)
		if err != nil {
			return nil, err
		}
		keys := exposureKeys(ev.exposures)
		if v.principalType == "" {
			roleKeys = keys
		} else if !ev.truncated && maps.Equal(keys, roleKeys) {
			out = append(out, aclaudit.Finding{
				Rule: ruleSameAsRole, Severity: aclaudit.Low, Subject: v.String(),
				Detail: fmt.Sprintf("%s reads the same labeled data as the role alone", v),
				Fix:    "if this client should read less, narrow its client_baselines entry",
			})
			continue
		}
		if v.principalType != "" && len(ev.exposures) == 0 {
			out = append(out, aclaudit.Finding{
				Rule: ruleClientReadsNone, Severity: aclaudit.Low, Subject: v.String(),
				Detail: fmt.Sprintf("%s reads no labeled data", v),
			})
			continue
		}
		for _, e := range ev.exposures {
			if v.principalType == "" && v.role != acl.EveryoneRole && everyone[exposureKey(e)] {
				continue
			}
			out = append(out, exposureFinding(v, e))
		}
		if ev.truncated {
			out = append(out, aclaudit.Finding{
				Rule: ruleTruncated, Severity: aclaudit.Low, Subject: v.String(),
				Detail: fmt.Sprintf("%s has more than %d classification findings; the rest are not listed",
					v, classification.MaxFindingsPerView),
				Fix: "narrow the role, or run `rela classification report` to see the labels involved",
			})
		}
	}
	return out, nil
}

func exposureKey(e classification.Exposure) string {
	return e.Rule + "\x00" + e.Subject + "\x00" + e.Label + "\x00" + strings.Join(e.Fields, "\x00")
}

func exposureKeys(es []classification.Exposure) map[string]bool {
	out := make(map[string]bool, len(es))
	for _, e := range es {
		out[exposureKey(e)] = true
	}
	return out
}

func exposureFinding(v classificationView, e classification.Exposure) aclaudit.Finding {
	var detail, fix string
	switch e.Rule {
	case classification.RuleReadsLabel:
		detail = fmt.Sprintf("%s reads %s data in %s", v, e.Label, strings.Join(e.Fields, ", "))
		fix = "if this role should not see it, hide the fields with visible: or drop the read grant"
	default:
		scope := "one record"
		if e.Rule == classification.RuleDerivedSubject {
			scope = "data about one person"
		}
		detail = fmt.Sprintf("%s can combine %s in %s into %s", v, strings.Join(e.Fields, ", "), scope, e.Label)
		switch {
		case len(e.Breakers) > 0:
			fix = "hiding any one of " + strings.Join(e.Breakers, ", ") + " breaks the combination"
		case e.MinRemovals > 0:
			fix = fmt.Sprintf("no single hidden field breaks the combination; hiding %d of them together does",
				e.MinRemovals)
		default:
			fix = fmt.Sprintf("breaking the combination needs more than %d hidden fields", classification.MaxRemovals)
		}
	}
	return aclaudit.Finding{
		Rule: e.Rule, Severity: aclaudit.Low, Subject: e.Subject, Detail: detail, Fix: fix,
		Label: e.Label, Fields: e.Fields,
	}
}

// classificationWarnings receives the audit's warnings about
// classification.yaml. It is stderr so a warning never corrupts `-o json`.
var classificationWarnings io.Writer = os.Stderr

// auditClassification returns the classification findings for `rela acl
// audit`. No classification.yaml means none. A file that cannot be read or
// does not parse is reported as a warning and skipped: the ACL findings stay
// useful without it.
func auditClassification(
	ctx context.Context, root string, policy *acl.Policy, meta *metamodel.Metamodel,
) ([]aclaudit.Finding, error) {
	data, exists, err := readClassificationFile(filepath.Join(root, classification.FileName))
	if err != nil {
		fmt.Fprintf(classificationWarnings, "WARNING: %v; skipping its findings\n", err)
		return nil, nil
	}
	if !exists {
		return nil, nil
	}
	f, issues := classification.Parse(data)
	if len(issues) > 0 {
		fmt.Fprintf(classificationWarnings,
			"WARNING: %s has %d error(s); skipping its findings (run `rela classification lint`)\n",
			classification.FileName, len(issues))
		return nil, nil
	}
	return classificationFindings(ctx, policy, meta, f)
}
