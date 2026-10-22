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
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
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
// With `--webhook <id>`, it maps a data-entry.yaml webhook as its deliveries
// run, with the `webhook` client ceiling applied, and checks that the hook's
// mapping is possible. `--expect <file>` turns a single-principal map into a
// CI check: it diffs the access against the file and exits 1 on any
// difference (TKT-J3RDJ4).
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
	Verb      string `help:"Restrict to one verb: read|create|update|delete|convert. Omit for all." enum:",read,create,update,delete,convert" default:""`
	Type      string `help:"Restrict to one entity type. Omit for all declared types." default:""`
	As        string `help:"Map as a CLIENT of this principal_type (e.g. app, pat, service), applying the matching client_baselines ceiling. Requires --principal." default:""`
	Scope     string `help:"Space- or comma-separated scopes the client presents, re-opening capability the baseline closed. Requires --as." default:""`
	Webhook   string `help:"Map a declarative webhook (a key under webhooks: in data-entry.yaml) as its deliveries run: principal system:webhook:<id> with principal_type webhook. Adds permissions, relation writes and a check of what the hook's mapping needs." default:""`
	Expect    string `help:"Compare the access with this expectations YAML file (verbs per type, named permissions, relation writes; not rename or fields). Prints the differences and exits 1 when there are any. Requires --principal or --webhook." default:""`
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

// checkModeFlags rejects flag combinations that would report something other
// than what the operator asked for.
func (c *ACLMapCmd) checkModeFlags() error {
	hasPrincipal := strings.TrimSpace(c.Principal) != ""
	if strings.TrimSpace(c.Webhook) != "" {
		// The hook's identity and principal type are fixed by the runtime;
		// letting a flag override either would map a principal no delivery
		// ever runs as.
		if hasPrincipal || c.As != "" || c.Scope != "" {
			return stderrors.New("--webhook fixes the principal and its principal_type; " +
				"it cannot be combined with --principal, --as or --scope")
		}
	}
	if principal.IsWebhookUser(strings.TrimSpace(c.Principal)) && (c.As != "" || c.Scope != "") {
		// The runtime derives a hook's principal_type from its name, so a
		// --as would be ignored while the report echoed it.
		return stderrors.New("a system:webhook: principal always has principal_type webhook; " +
			"drop --as and --scope, or use --webhook")
	}
	if c.Expect == "" {
		return nil
	}
	if !hasPrincipal && strings.TrimSpace(c.Webhook) == "" {
		return stderrors.New("--expect compares one principal's access; it requires --principal or --webhook")
	}
	if c.Verb != "" || c.Type != "" {
		return stderrors.New("--expect compares every type and verb; it cannot be combined with --verb or --type, " +
			"which would report everything outside the filter as missing")
	}
	return nil
}

// Run executes `rela acl map`. It dispatches to the webhook view when
// --webhook is set, the per-principal view when --principal is set, else
// the whole-graph inventory. With --expect it exits 1 when the access
// differs from the file.
func (c *ACLMapCmd) Run(ctx context.Context, svc *readServices) error {
	if err := c.checkModeFlags(); err != nil {
		return err
	}
	view, err := c.clientView()
	if err != nil {
		return err
	}
	hookID := strings.TrimSpace(c.Webhook)
	mapping, expectation, err := c.loadInputs(ctx, svc, hookID)
	if err != nil {
		return err
	}

	engine, policy, err := buildACLEngineWithPolicy(svc)
	if err != nil {
		if stderrors.Is(err, errNoACLPolicy) {
			if expectation != nil {
				return stderrors.New("--expect needs acl.yaml: without a policy every principal " +
					"has full access, which an expectations file cannot describe")
			}
			out.WriteSuccess("No acl.yaml found; every principal has full access (no policy).")
			return nil
		}
		return err
	}

	entityTypes := svc.Meta.EntityTypes()
	if hookID == "" && strings.TrimSpace(c.Principal) == "" {
		return c.runAll(ctx, engine, entityTypes)
	}

	// Permissions and relation writes complete the picture, so they are
	// computed whenever the answer is a contract (--expect) or a webhook,
	// whose report exists to be checked.
	var extras aclmap.Extras
	if hookID != "" || expectation != nil {
		extras = mapExtras(svc.Meta, policy)
	}
	var result *aclmap.MapPrincipalResult
	if hookID != "" {
		result, err = engine.MapWebhook(ctx, hookID, mapping, acl.Verb(c.Verb), c.Type, entityTypes, extras)
	} else {
		result, err = engine.MapPrincipalFull(ctx, c.Principal, acl.Verb(c.Verb), c.Type, entityTypes, view, extras)
	}
	if err != nil {
		return err
	}

	return c.report(result, expectation)
}

// loadInputs reads the hook's mapping (for --webhook) and the expectations
// file (for --expect), each nil when its flag is unset.
func (c *ACLMapCmd) loadInputs(
	ctx context.Context, svc *readServices, hookID string,
) (*aclmap.WebhookMapping, *accessExpectation, error) {
	var mapping *aclmap.WebhookMapping
	if hookID != "" {
		m, err := loadWebhookMapping(ctx, svc, hookID)
		if err != nil {
			return nil, nil, err
		}
		mapping = m
	}
	var expectation *accessExpectation
	if c.Expect != "" {
		e, err := loadExpectation(c.Expect, svc.Meta)
		if err != nil {
			return nil, nil, err
		}
		expectation = e
	}
	return mapping, expectation, nil
}

// report writes one principal's map and, with --expect, the comparison. It
// returns exit code 1 when the mapping check fails or the access differs.
func (c *ACLMapCmd) report(result *aclmap.MapPrincipalResult, expectation *accessExpectation) error {
	var compared *expectationResult
	if expectation != nil {
		compared = expectation.compare(c.Expect, result)
	}
	if out.Format == output.FormatJSON {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(mapReport{MapPrincipalResult: result, Expectation: compared}); err != nil {
			return err
		}
	} else {
		writeMapText(result)
		if compared != nil {
			writeExpectationText(compared)
		}
	}
	// A failed mapping check fails the command with or without --expect: a
	// hook whose deliveries cannot succeed is a defect on its own.
	if !result.WebhookCheck.OK() {
		if out.Format != output.FormatJSON {
			out.WriteMessage("Webhook %s: mapping check failed.", result.Webhook)
		}
		return errors.NewExitError(1)
	}
	if compared != nil && !compared.Match {
		return errors.NewExitError(1)
	}
	return nil
}

// mapReport is the JSON shape of a per-principal map: the map itself, plus
// the comparison when --expect was given.
type mapReport struct {
	*aclmap.MapPrincipalResult
	Expectation *expectationResult `json:"expectation,omitempty"`
}

// loadWebhookMapping looks the hook up in data-entry.yaml and returns what
// its mapping touches. An unknown id is an error: mapping a hook that does
// not exist would report on a principal no delivery runs as.
func loadWebhookMapping(ctx context.Context, svc *readServices, hookID string) (*aclmap.WebhookMapping, error) {
	loaded, err := loadDataEntryPermissionsCtx(ctx, svc.Config, svc.Meta)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dataentryconfig.ConfigFile, err)
	}
	hook, ok := loaded.cfg.Webhooks[hookID]
	if !ok {
		known := sortedKeys(loaded.cfg.Webhooks)
		if len(known) == 0 {
			return nil, fmt.Errorf("unknown webhook %q: %s declares no webhooks", hookID, dataentryconfig.ConfigFile)
		}
		return nil, fmt.Errorf("unknown webhook %q; %s declares: %s",
			hookID, dataentryconfig.ConfigFile, strings.Join(known, ", "))
	}
	m := hook.Mapping()
	mapping := &aclmap.WebhookMapping{CreateType: m.CreateType(), Updates: len(m.Then) > 0}
	if m.Find != nil {
		mapping.FindType = m.Find.Type
		mapping.FindFace = m.Find.Face
		mapping.Match = m.Find.Match
	}
	return mapping, nil
}

// mapExtras builds the permission and relation universes the map probes.
//
// Permissions are the names the policy's roles grant. A role's "*" matches no
// named permission, so a permission no role lists is held by nobody and need
// not be probed. Relations are every relation type the schema declares, with
// its source types.
func mapExtras(meta *metamodel.Metamodel, policy *acl.Policy) aclmap.Extras {
	var perms []string
	for _, role := range policy.Roles {
		perms = append(perms, role.Permissions...)
	}
	var rels []aclmap.RelationType
	for _, name := range meta.RelationTypes() {
		def, ok := meta.GetRelationDef(name)
		if !ok {
			continue
		}
		rels = append(rels, aclmap.RelationType{Name: name, From: def.From})
	}
	return aclmap.Extras{Permissions: perms, Relations: rels}
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
	if r.Webhook != "" {
		who = fmt.Sprintf("webhook %s (%s)", r.Webhook, r.Principal)
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
		if r.Baseline == "" {
			out.WriteMessage("%s — no client baseline matches, so access is not capped.", line)
		} else {
			out.WriteMessage("%s — client baseline %q is applied below.", line, r.Baseline)
		}
	}

	if r.EveryoneOnly {
		out.WriteMessage("  Only the built-in everyone baseline — no assigned, group, or graph-conferred access.")
	}
	if len(r.Types) == 0 {
		out.WriteMessage("  (no access to any declared type)")
	} else {
		writeTypeAccess(r.Types)
	}
	writeExtrasText(r)
	writeWebhookCheckText(r.WebhookCheck)
}

// writeExtrasText renders the permissions and relation writes, when the map
// computed them.
func writeExtrasText(r *aclmap.MapPrincipalResult) {
	if r.Webhook == "" && r.Permissions == nil && r.Relations == nil {
		return
	}
	if len(r.Permissions) == 0 {
		out.WriteMessage("  permissions: none")
	} else {
		out.WriteMessage("  permissions: %s", strings.Join(r.Permissions, ", "))
	}
	if len(r.Relations) == 0 {
		out.WriteMessage("  relation writes: none")
		return
	}
	out.WriteMessage("  relation writes:")
	for _, ra := range r.Relations {
		parts := make([]string, 0, len(ra.Verbs))
		for _, verb := range sortedKeys(ra.Verbs) {
			parts = append(parts, verb+" from "+strings.Join(ra.Verbs[verb], ", "))
		}
		out.WriteMessage("      %s: %s", ra.Relation, strings.Join(parts, "; "))
	}
}

// writeWebhookCheckText renders what a hook's mapping needs and whether the
// hook holds it.
func writeWebhookCheckText(c *aclmap.WebhookCheck) {
	if c == nil {
		return
	}
	out.WriteMessage("  webhook mapping needs:")
	creates := false
	for _, n := range c.Needs {
		target := n.Type
		if n.Face != "" {
			target += "@" + n.Face
		}
		state := "held"
		if !n.Held {
			state = "NOT HELD: deliveries that need it fail"
		}
		out.WriteMessage("      %s %s: %s", n.Verb, target, state)
		creates = creates || n.Verb == string(acl.VerbCreate)
	}
	if len(c.HiddenMatchFields) == 0 {
		return
	}
	effect := "no delivery ever matches"
	if creates {
		effect = "every delivery creates a new entity"
	}
	out.WriteMessage("      match fields hidden by the client ceiling: %s (the find cannot compare them, so %s)",
		strings.Join(c.HiddenMatchFields, ", "), effect)
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
		for _, face := range sortedKeys(ta.Faces) {
			out.WriteMessage("          %s (face %s only)", joinVerbs(ta.Faces[face]), face)
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
