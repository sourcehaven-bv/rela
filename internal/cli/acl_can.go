package cli

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ACLCanCmd implements `rela acl can <principal> <verb> <entity>`. It is
// the scriptable spot-check of the effective-access map: a yes/no answer
// for one principal, one verb, one entity, with the deciding route(s) on a
// "yes". It exits non-zero on deny so a CI gate or shell guard can branch
// on it (`rela acl can bob update INC-042 && deploy`).
//
// Read is decided by the same read path the server enforces (shared with
// who-can / map), so a "no" is never a false negative. A grant via the
// built-in everyone role counts as a "yes" — the spot-check answers "can
// this principal?", including when the reason is "everyone can". Create is
// decided with the entity's id, matching the production create path (which
// folds local-edge routes), so an edge-conferred create reads as ALLOW.
//
// Exit codes: 0 = allow, 1 = deny OR an engine/store error (fail-closed).
// A missing entity is a distinct error ("entity not found"), never a plain
// deny, so a typo is never a green attestation.
//
// Scope caveat: when the policy sets `principal_property`, that raw→entity
// resolution is wired only into the data-entry (HTTP) transport. This
// command resolves the same way for reporting, but the answer reflects
// data-entry-transport access — CLI/MCP/scheduler writes authorize against
// the raw principal. See internal/acl/declarative.go.
type ACLCanCmd struct {
	Principal string `arg:"" help:"Principal to check (a user entity ID, or the raw identifier — email/UPN — when principal_property is set)."`
	Verb      string `arg:"" help:"Access verb to check: read|create|update|delete." enum:"read,create,update,delete"`
	Entity    string `arg:"" help:"Entity ID to check access against (e.g. INC-042)."`
}

// Run executes `rela acl can`.
func (c *ACLCanCmd) Run(ctx context.Context, svc *readServices) error {
	engine, err := buildACLEngine(svc)
	if err != nil {
		if stderrors.Is(err, errNoACLPolicy) {
			return c.runNoPolicy(ctx, svc)
		}
		return err
	}

	result, err := engine.Can(ctx, c.Principal, acl.Verb(c.Verb), c.Entity)
	if err != nil {
		if stderrors.Is(err, aclmap.ErrEntityNotFound) {
			return fmt.Errorf("entity %q not found", c.Entity)
		}
		return err
	}

	if out.Format == output.FormatJSON {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(result); encErr != nil {
			return encErr
		}
	} else {
		writeCanText(result)
	}

	// Non-zero exit on deny so the command is scriptable. The JSON/text
	// output is already written, so the exit code carries no message.
	if !result.Allowed {
		return errors.NewExitError(1)
	}
	return nil
}

// runNoPolicy handles `acl can` when the project has no acl.yaml: every
// principal has full access, so the answer is ALLOW — but the entity
// existence gate still applies, so a typo'd id errors rather than attesting
// a green exit on nothing (the same gate engine.Can enforces under a
// policy).
func (c *ACLCanCmd) runNoPolicy(ctx context.Context, svc *readServices) error {
	if _, err := svc.Store.GetEntity(ctx, c.Entity); err != nil {
		if stderrors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("entity %q not found", c.Entity)
		}
		return err
	}
	out.WriteSuccess("No acl.yaml found; every principal has full access (no policy).")
	return nil
}

// writeCanText renders a CanResult as a single allow/deny line plus the
// deciding route(s). A deny names the principal/verb/entity so the "no" is
// self-explanatory; an allow lists every route (and notes an everyone
// grant) so the "yes" is auditable.
func writeCanText(r *aclmap.CanResult) {
	who := r.Principal
	if r.Raw != "" {
		who = fmt.Sprintf("%s (%s)", r.Principal, r.Raw)
	}

	if !r.Allowed {
		out.WriteMessage("DENY: %s cannot %s %s (%s) — no grant.", who, r.Verb, r.Entity, r.EntityType)
		return
	}

	out.WriteMessage("ALLOW: %s can %s %s (%s).", who, r.Verb, r.Entity, r.EntityType)
	if r.Everyone {
		out.WriteMessage("      via the built-in everyone role — all principals.")
	}
	for _, rt := range r.Routes {
		out.WriteMessage("      via %s", formatRoute(rt))
	}
}
