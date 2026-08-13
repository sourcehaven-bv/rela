package cli

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// ACLMapCmd implements `rela acl map`. With `--principal`, it reports one
// principal's effective access across the graph, aggregated by entity type
// with per-entity exceptions — the inverse of who-can (which fixes the
// entity and varies the principal). It answers the onboarding question
// ("did the grant I just made confer exactly what I intended?", UC1) and
// the offboarding question ("what can this account still reach?", UC2).
//
// Without `--principal`, it reports the WHOLE-GRAPH inventory: every
// enumerated principal's effective access, with the built-in everyone
// baseline lifted out and reported once. This is the O(P·E) case — see the
// cost note in internal/aclmap.
//
// Read is decided by the same read path the server enforces, so no
// reachable entity is dropped. Access via the built-in `everyone` role is
// part of every principal's baseline; a principal whose ONLY access is
// that baseline is flagged as fully cut off — the offboarding all-clear.
//
// Scope caveat: when the policy sets `principal_property`, that
// raw→entity resolution is wired only into the data-entry (HTTP)
// transport. This command resolves the same way for reporting, but the
// answer reflects data-entry-transport access. See
// internal/acl/declarative.go.
type ACLMapCmd struct {
	Principal string `help:"Principal to map (a user entity ID, or the raw identifier — email/UPN — when principal_property is set). Omit to map EVERY principal (whole-graph inventory)." default:""`
	Verb      string `help:"Restrict to one verb: read|create|update|delete. Omit for all four." enum:",read,create,update,delete" default:""`
	Type      string `help:"Restrict to one entity type. Omit for all declared types." default:""`
	As        string `help:"Map as a CLIENT of this principal_type (e.g. app, pat, service), applying the matching client_baselines ceiling. Requires --principal." default:""`
	Scope     string `help:"Space- or comma-separated scopes the client presents, re-opening capability the baseline closed. Requires --as." default:""`
}

// clientView builds the attenuation view from --as/--scope. Returns an error
// when the flags are combined in a way that would silently report the wrong
// thing rather than what the operator asked for.
func (c *ACLMapCmd) clientView() (aclmap.ClientView, error) {
	as := strings.TrimSpace(c.As)
	scopes := splitScopes(c.Scope)

	// --as without --principal would be meaningless: the whole-graph inventory
	// enumerates principals from the graph, and a client ceiling is a property
	// of one caller's token. Silently ignoring it would print an unattenuated
	// map under a flag that promised attenuation.
	if strings.TrimSpace(c.Principal) == "" && (as != "" || len(scopes) > 0) {
		return aclmap.ClientView{}, stderrors.New(
			"--as/--scope describe one client's token, so they require --principal " +
				"(the whole-graph map has no single caller to attenuate)")
	}
	// A scope re-opens what a baseline closed; with no baseline selected there
	// is nothing to re-open, so this is almost certainly a mistake.
	if as == "" && len(scopes) > 0 {
		return aclmap.ClientView{}, stderrors.New(
			"--scope requires --as: scopes re-open capability a client baseline closed, " +
				"and without --as no baseline applies")
	}
	return aclmap.ClientView{PrincipalType: as, Scopes: scopes}, nil
}

// splitScopes accepts either the space-delimited form the `scope` claim uses
// (RFC 6749 §3.3) or a comma-separated list, since a shell user typing a flag
// will reach for either.
func splitScopes(v string) []string {
	fields := strings.FieldsFunc(v, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t' || r == '\n'
	})
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// Run executes `rela acl map`. It dispatches to the per-principal view
// when --principal is set, else the whole-graph inventory.
func (c *ACLMapCmd) Run(ctx context.Context, svc *readServices) error {
	engine, err := buildACLEngine(svc)
	if err != nil {
		if stderrors.Is(err, errNoACLPolicy) {
			out.WriteSuccess("No acl.yaml found; every principal has full access (no policy).")
			return nil
		}
		return err
	}

	view, err := c.clientView()
	if err != nil {
		return err
	}

	entityTypes := svc.Meta.EntityTypes()
	if strings.TrimSpace(c.Principal) == "" {
		return c.runAll(ctx, engine, entityTypes)
	}

	result, err := engine.MapPrincipalAs(
		ctx, c.Principal, acl.Verb(c.Verb), c.Type, entityTypes, view)
	if err != nil {
		return err
	}

	if out.Format == output.FormatJSON {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	writeMapText(result)
	return nil
}

// runAll executes the whole-graph inventory (no --principal).
func (c *ACLMapCmd) runAll(ctx context.Context, engine *aclmap.Engine, entityTypes []string) error {
	result, err := engine.MapAll(ctx, acl.Verb(c.Verb), c.Type, entityTypes)
	if err != nil {
		return err
	}
	if out.Format == output.FormatJSON {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	writeMapAllText(result)
	return nil
}

// writeMapAllText renders a MapAllResult: a headline summary, the everyone
// baseline once, then each principal's non-everyone access (reusing the
// per-principal renderer's type/exception layout).
func writeMapAllText(r *aclmap.MapAllResult) {
	out.WriteMessage("Whole-graph access — %d principal(s), %d grant-source(s) — verbs: %s",
		r.PrincipalCount, r.GrantSourceCount, joinVerbs(r.Verbs))

	if len(r.EveryoneBaseline) > 0 {
		out.WriteMessage("  everyone baseline (all principals):")
		for _, et := range r.EveryoneBaseline {
			out.WriteMessage("      %s: %s", et.Type, joinVerbs(et.Verbs))
		}
	}

	for i := range r.Principals {
		pr := &r.Principals[i]
		who := pr.Principal
		if pr.Raw != "" {
			who = fmt.Sprintf("%s (%s)", pr.Principal, pr.Raw)
		}
		if pr.EveryoneOnly {
			out.WriteMessage("  %s — everyone baseline only (no personal grant)", who)
			continue
		}
		out.WriteMessage("  %s", who)
		writeTypeAccess(pr.Types)
	}
}

// writeMapText renders a MapPrincipalResult: a header, then per type the
// baseline verbs and any per-entity exceptions, each grant showing its
// route(s). A fully-cut-off principal (everyone-baseline only) gets a
// single clear line.
func writeMapText(r *aclmap.MapPrincipalResult) {
	who := r.Principal
	if r.Raw != "" {
		who = fmt.Sprintf("%s (%s)", r.Principal, r.Raw)
	}
	out.WriteMessage("Effective access for %s — verbs: %s", who, joinVerbs(r.Verbs))

	// Name the client this map describes. Without it the artifact is
	// ambiguous: the same principal produces different maps through different
	// clients, and a reader who assumed "Alice's access" would draw the wrong
	// conclusion from an attenuated one.
	if r.As != "" {
		line := `  as client type "` + r.As + `"`
		if len(r.Scopes) > 0 {
			line += " with scopes " + strings.Join(r.Scopes, " ")
		}
		out.WriteMessage("%s — the client_baselines ceiling is applied below.", line)
	}

	if r.EveryoneOnly {
		out.WriteMessage("  Only the built-in everyone baseline — no assigned, group, or graph-conferred access.")
	}
	if len(r.Types) == 0 {
		out.WriteMessage("  (no access to any declared type)")
		return
	}

	writeTypeAccess(r.Types)
}

// writeTypeAccess renders a slice of per-type access (baseline verbs +
// per-entity exceptions), shared by the per-principal and whole-graph map
// renderers so their layout can never drift.
func writeTypeAccess(types []aclmap.TypeAccess) {
	for _, ta := range types {
		out.WriteMessage("      %s", ta.Type)
		for _, verb := range sortedKeys(ta.Baseline) {
			out.WriteMessage("          %s (all %s): %s", verb, ta.Type, joinRoutes(ta.Baseline[verb]))
		}
		for _, ex := range ta.Exceptions {
			out.WriteMessage("          exception %s:", ex.Entity)
			for _, verb := range sortedKeys(ex.Extra) {
				out.WriteMessage("              + %s: %s", verb, joinRoutes(ex.Extra[verb]))
			}
		}
	}
}

// joinRoutes renders a verb's routes as a compact list.
func joinRoutes(routes []aclmap.Route) string {
	parts := make([]string, len(routes))
	for i, rt := range routes {
		parts[i] = formatRoute(rt)
	}
	return strings.Join(parts, "; ")
}

func joinVerbs(verbs []string) string { return strings.Join(verbs, ", ") }

// sortedKeys returns a verb-map's keys in stable order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
