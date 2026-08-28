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

// ACLCanRelationCmd implements
// `rela acl can-relation <principal> <verb> <relation> --from <entity>`:
// the relation-shaped sibling of `acl can`.
//
// It exists because relation writes were previously unaskable. `acl can`,
// `acl map` and `acl who-can` are all entity-shaped, so a policy that denied
// every relation write at runtime could pass every static check — which is
// exactly what happened in the outage behind TKT-XZEY, where the gap cost a
// production incident rather than a puzzled operator.
//
// The verdict comes from [aclmap.Engine.CanRelation], which calls the same
// AuthorizeWrite the entitymanager calls. It cannot drift from the runtime
// answer, because it IS the runtime answer.
//
// Exit codes: 0 = allow, 1 = deny OR an engine/store error (fail-closed).
// A missing source entity or an undeclared relation type is a distinct error,
// never a plain deny — a typo must not read as a considered "no".
type ACLCanRelationCmd struct {
	Principal string `arg:"" help:"Principal to check (a user entity ID, or the raw identifier — email/UPN — when principal_property is set)."`
	Verb      string `arg:"" help:"Relation write verb: create|update|delete. (There is no relation read verb — visibility is derived from both endpoints.)" enum:"create,update,delete"`
	Relation  string `arg:"" help:"Relation type to check (e.g. spawnt)."`
	From      string `help:"Source entity ID the edge originates at (e.g. TERUG-1)." required:""`
}

// Run executes `rela acl can-relation`.
func (c *ACLCanRelationCmd) Run(ctx context.Context, svc *readServices) error {
	// Reject an undeclared relation type before consulting the policy: the
	// gate would answer "denied" for a typo, and a typo that reads as a
	// considered denial is the failure mode this command exists to remove.
	if svc.Meta != nil {
		if _, ok := svc.Meta.Relations[c.Relation]; !ok {
			return fmt.Errorf("relation type %q is not declared in the schema", c.Relation)
		}
	}

	engine, err := buildACLEngine(svc)
	if err != nil {
		if stderrors.Is(err, errNoACLPolicy) {
			return c.runNoPolicy(ctx, svc)
		}
		return err
	}

	result, err := engine.CanRelation(ctx, c.Principal, acl.Verb(c.Verb), c.Relation, c.From)
	if err != nil {
		if stderrors.Is(err, aclmap.ErrEntityNotFound) {
			return fmt.Errorf("entity %q not found", c.From)
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
		writeCanRelationText(result)
	}

	if !result.Allowed {
		return errors.NewExitError(1)
	}
	return nil
}

// runNoPolicy mirrors `acl can`: with no acl.yaml every principal has full
// access, but the source entity must still exist so a typo'd id cannot exit
// green on nothing.
func (c *ACLCanRelationCmd) runNoPolicy(ctx context.Context, svc *readServices) error {
	if _, err := svc.Store.GetEntity(ctx, c.From); err != nil {
		if stderrors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("entity %q not found", c.From)
		}
		return err
	}
	out.WriteSuccess("No acl.yaml found; every principal has full access (no policy).")
	return nil
}

// writeCanRelationText renders the verdict as one line plus the deciding rule.
// The rule is the useful part of a deny: "which gate said no" is the question
// an operator debugging a relation grant actually has, and each gate needs a
// different fix.
func writeCanRelationText(r *aclmap.CanRelationResult) {
	who := r.Principal
	if r.Raw != "" {
		who = fmt.Sprintf("%s (%s)", r.Principal, r.Raw)
	}
	edge := fmt.Sprintf("%s edge from %s (%s)", r.Relation, r.From, r.FromType)

	if r.Allowed {
		out.WriteMessage("ALLOW: %s can %s a %s.", who, r.Verb, edge)
		if r.RuleKind != "" {
			out.WriteMessage("      via %s", formatRelationRule(r.RuleKind, r.RuleID))
		}
		return
	}

	out.WriteMessage("DENY: %s cannot %s a %s.", who, r.Verb, edge)
	if r.Reason != "" {
		out.WriteMessage("      %s", r.Reason)
	}
	if r.RuleKind != "" {
		out.WriteMessage("      (rule_kind=%s rule_id=%s)", r.RuleKind, r.RuleID)
	}
}

// formatRelationRule names the granting rule in the operator's vocabulary
// rather than the evaluator's, so an allow explains WHICH configuration
// produced it — the two allow paths need different edits to revoke.
func formatRelationRule(kind, id string) string {
	switch kind {
	case "relation-grant":
		return fmt.Sprintf("relation_grants — permission %q", id)
	case "role-grant":
		return fmt.Sprintf("role %q granting the verb on the source entity type", id)
	default:
		return fmt.Sprintf("%s %q", kind, id)
	}
}
