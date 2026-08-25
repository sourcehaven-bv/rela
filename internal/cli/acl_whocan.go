package cli

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// ACLWhoCanCmd implements `rela acl who-can <verb> <entity>`. It lists
// every principal permitted to perform the verb on the entity, each
// annotated with the route(s) — global assignment, group, or a
// role-conferring / inherited graph edge — by which the grant was
// acquired. It is the first slice of the effective-access map
// (FEAT-RCQ6SJ / TKT-9089I6): the confidentiality-attestation question
// "who can read this, and by what path?".
//
// Read is decided by the same read path the server enforces, so a
// reader is never omitted. Access granted to the built-in `everyone`
// role is reported once, globally, rather than against each principal.
//
// The listing reports every entity the resolver would grant the verb —
// including a group/role entity that itself holds an assigned role (it is
// not hidden by graph topology, since that would risk dropping a real
// actor who happens to also be a membership target). Each row's routes
// name why the grant applies, so a group entity is easy to recognize.
//
// Scope caveat: when the policy sets `principal_property`, that
// raw→entity resolution is wired only into the data-entry (HTTP)
// transport. This command resolves the same way for reporting, but the
// answer reflects data-entry-transport access — CLI/MCP/scheduler writes
// authorize against the raw principal. See internal/acl/declarative.go.
type ACLWhoCanCmd struct {
	Verb   string `arg:"" help:"Access verb to check: read|create|update|delete." enum:"read,create,update,delete"`
	Entity string `arg:"" help:"Entity ID to report access for (e.g. INC-042)."`
}

// Run executes `rela acl who-can`.
func (c *ACLWhoCanCmd) Run(ctx context.Context, svc *readServices) error {
	engine, err := buildACLEngine(svc)
	if err != nil {
		if stderrors.Is(err, errNoACLPolicy) {
			out.WriteSuccess("No acl.yaml found; every principal has full access (no policy).")
			return nil
		}
		return err
	}

	result, err := engine.WhoCan(ctx, acl.Verb(c.Verb), c.Entity)
	if err != nil {
		if stderrors.Is(err, aclmap.ErrEntityNotFound) {
			return fmt.Errorf("entity %q not found", c.Entity)
		}
		return err
	}

	if out.Format == output.FormatJSON {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	writeWhoCanText(result)
	return nil
}

// writeWhoCanText renders a WhoCanResult as human-readable lines: the
// global everyone grant first (when present), then any conditional
// (asserted-claim) grants, then one block per principal listing every
// route by which the verb is granted.
func writeWhoCanText(r *aclmap.WhoCanResult) {
	if r.Everyone.Granted {
		note := ""
		if r.Everyone.Wildcard {
			note = " (via \"*\" — every type)"
		}
		out.WriteMessage("everyone — all principals, incl. unauthenticated%s", note)
	}

	// Rendered in their own block, and never counted as principals. Whoever
	// presents the claim gets the role, and that population lives in the IdP —
	// so the honest report names the claim and says we cannot enumerate it.
	if len(r.Conditional) > 0 {
		out.WriteMessage("%d conditional grant(s) via verified assertion claims "+
			"(holders NOT enumerable from the graph — check your identity provider):",
			len(r.Conditional))
		for _, c := range r.Conditional {
			note := ""
			if c.Wildcard {
				note = " (via \"*\" — every type)"
			}
			out.WriteMessage("  claim %q → role %s%s", c.Claim, c.Role, note)
		}
	}

	if len(r.Principals) == 0 {
		// "No principal can ..." would be a false all-clear when a claim or the
		// everyone role grants the verb.
		if !r.Everyone.Granted && len(r.Conditional) == 0 {
			out.WriteMessage("No principal can %s %s (%s).", r.Verb, r.Entity, r.EntityType)
		}
		return
	}

	out.WriteMessage("%d principal(s) can %s %s (%s):", len(r.Principals), r.Verb, r.Entity, r.EntityType)
	for _, p := range r.Principals {
		who := p.Principal
		if p.Raw != "" {
			who = fmt.Sprintf("%s (%s)", p.Principal, p.Raw)
		}
		out.WriteMessage("  %s", who)
		for _, rt := range p.Routes {
			out.WriteMessage("      via %s", formatRoute(rt))
		}
	}
}

// formatRoute renders one route's terminal facts as a compact,
// action-oriented string naming the role and the key entities/relation.
func formatRoute(rt aclmap.Route) string {
	parts := []string{"role " + rt.Role}
	switch {
	case rt.Claim != "":
		parts = append(parts, fmt.Sprintf("asserted claim %q", rt.Claim))
	case rt.Group != "" && rt.Ancestor != "":
		parts = append(parts, fmt.Sprintf("group %s → %s edge on ancestor %s", rt.Group, rt.Relation, rt.Ancestor))
	case rt.Group != "" && rt.Relation != "":
		parts = append(parts, fmt.Sprintf("group %s → %s edge", rt.Group, rt.Relation))
	case rt.Group != "":
		parts = append(parts, "group "+rt.Group)
	case rt.Ancestor != "":
		parts = append(parts, fmt.Sprintf("%s edge on ancestor %s", rt.Relation, rt.Ancestor))
	case rt.Relation != "":
		parts = append(parts, rt.Relation+" edge")
	}
	return fmt.Sprintf("%s [%s]", strings.Join(parts, ", "), rt.Kind)
}

// errNoACLPolicy signals that the project has no acl.yaml. Callers treat
// it as "no policy → nothing to report" and print a friendly note rather
// than an error, so the shared builder can stay a plain helper.
var errNoACLPolicy = stderrors.New("aclmap: no acl.yaml")

// buildACLEngine loads acl.yaml, validates it against the metamodel, and
// wires an aclmap.Engine over the store — the shared setup for every
// `rela acl` reporting command (who-can, map). Returns errNoACLPolicy
// (via errors.Is) when the project has no policy so the caller can print
// its own friendly message.
func buildACLEngine(svc *readServices) (*aclmap.Engine, error) {
	policyPath := filepath.Join(svc.Paths.Root, "acl.yaml")
	policy, err := acl.LoadPolicy(policyPath)
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
			return nil, errNoACLPolicy
		}
		return nil, fmt.Errorf("load acl.yaml: %w", err)
	}
	if vErr := policy.ValidateAgainstMetamodel(aclMetamodelView{svc.Meta}); vErr != nil {
		return nil, fmt.Errorf("acl.yaml invalid for this project: %w", vErr)
	}
	decl, err := acl.NewDeclarative(policy, acl.NewStoreGraph(svc.Store), svc.Store,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(svc.Store)))
	if err != nil {
		return nil, fmt.Errorf("build ACL resolver: %w", err)
	}
	engine, err := aclmap.New(svc.Store, decl)
	if err != nil {
		return nil, fmt.Errorf("build access engine: %w", err)
	}
	return engine, nil
}

// aclMetamodelView adapts *metamodel.Metamodel to acl.MetamodelView for
// the who-can policy gate. Mirrors appbuild's metamodelView; kept here
// so the CLI doesn't reach into appbuild internals.
type aclMetamodelView struct {
	m *metamodel.Metamodel
}

func (v aclMetamodelView) HasEntityType(entityType string) bool {
	if v.m == nil {
		return false
	}
	return v.m.HasEntityType(entityType)
}

func (v aclMetamodelView) HasRelationType(relationType string) bool {
	if v.m == nil {
		return false
	}
	_, ok := v.m.Relations[relationType]
	return ok
}

func (v aclMetamodelView) PropertyInfo(entityType, property string) acl.PropertyInfo {
	if v.m == nil {
		return acl.PropertyInfo{}
	}
	def, ok := v.m.GetEntityDef(entityType)
	if !ok {
		return acl.PropertyInfo{}
	}
	pd, ok := def.Properties[property]
	if !ok {
		return acl.PropertyInfo{}
	}
	return acl.PropertyInfo{Exists: true, Unique: pd.Unique, List: pd.List}
}
