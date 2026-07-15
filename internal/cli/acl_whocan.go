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
func (c *ACLWhoCanCmd) Run(ctx context.Context, svc *cliServices) error {
	policyPath := filepath.Join(svc.Paths().Root, "acl.yaml")
	policy, err := acl.LoadPolicy(policyPath)
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
			out.WriteSuccess("No acl.yaml found; every principal has full access (no policy).")
			return nil
		}
		return fmt.Errorf("load acl.yaml: %w", err)
	}

	// Fail loud on a policy that references schema the metamodel doesn't
	// declare (e.g. a non-unique principal_property) — the same gate the
	// server applies at wiring time, so who-can can't report against a
	// policy the server would reject.
	if vErr := policy.ValidateAgainstMetamodel(aclMetamodelView{svc.Meta()}); vErr != nil {
		return fmt.Errorf("acl.yaml invalid for this project: %w", vErr)
	}

	decl, err := acl.NewDeclarative(policy, acl.NewStoreGraph(svc.Store()), svc.Store(),
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(svc.Store())))
	if err != nil {
		return fmt.Errorf("build ACL resolver: %w", err)
	}

	engine, err := aclmap.New(svc.Store(), decl)
	if err != nil {
		return fmt.Errorf("build access engine: %w", err)
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
// global everyone grant first (when present), then one block per
// principal listing every route by which the verb is granted.
func writeWhoCanText(r *aclmap.WhoCanResult) {
	if r.Everyone.Granted {
		note := ""
		if r.Everyone.Wildcard {
			note = " (via \"*\" — every type)"
		}
		out.WriteMessage("everyone — all principals, incl. unauthenticated%s", note)
	}

	if len(r.Principals) == 0 {
		if !r.Everyone.Granted {
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
