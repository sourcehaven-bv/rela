package aclmap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// CanRelationResult is the answer to "may principal P create/update/delete a
// <relType> edge FROM entity F".
//
// Unlike [CanResult] it carries no Routes. A relation write is decided by the
// policy's global grants — the delegate-X permission gate, the client ceiling,
// the relation_grants block, and the source type's verb grant — none of which
// is a graph route the way an entity grant can be. What it carries instead is
// the deciding rule, verbatim from the evaluator: RuleKind names WHICH gate
// spoke, and Reason is that gate's own words.
//
// That is the point of the command. The motivating outage (TKT-XZEY) had two
// causes: no verb expressed "may add an edge", and no tool could ask. A report
// derived from a second reading of the policy would be free to drift from the
// gate; this one is the gate's own verdict.
type CanRelationResult struct {
	SchemaVersion int    `json:"schema_version"`
	Principal     string `json:"principal"`
	Raw           string `json:"raw,omitempty"`
	Verb          string `json:"verb"`
	Relation      string `json:"relation"`
	From          string `json:"from"`
	FromType      string `json:"from_type"`
	Allowed       bool   `json:"allowed"`
	// RuleKind is the deciding rule class as the evaluator reports it:
	// "delegate-permission", "client-ceiling", "relation-grant" or
	// "role-grant".
	RuleKind string `json:"rule_kind,omitempty"`
	// RuleID names the specific rule: a permission name, a baseline name, or
	// the granting role. "-" when no rule matched.
	RuleID string `json:"rule_id,omitempty"`
	// Reason is the evaluator's own explanation, present on a deny.
	Reason string `json:"reason,omitempty"`
}

// ErrRelationNotFound reports a relation type absent from the metamodel.
var ErrRelationNotFound = errors.New("relation type not found")

// relationVerbs are the write verbs a relation supports. Read is absent by
// design: relation visibility is derived from both endpoints, so there is no
// relation-level read grant to report on (see acl.RelationWriteGrant).
var relationVerbs = map[acl.Verb]acl.Op{
	acl.VerbCreate: acl.OpCreate,
	acl.VerbUpdate: acl.OpUpdate,
	acl.VerbDelete: acl.OpDelete,
}

// RelationVerbValid reports whether verb is checkable against a relation.
func RelationVerbValid(verb acl.Verb) bool {
	_, ok := relationVerbs[verb]
	return ok
}

// CanRelation reports whether rawPrincipal may perform verb on a relType edge
// originating at fromID.
//
// It asks the REAL gate — the same [acl.Request.AuthorizeWrite] the
// entitymanager calls on every relation write — rather than re-deriving a
// verdict from the policy. A re-derivation could disagree with the runtime,
// and a static check that disagrees with the gate is worse than no check: it
// is the failure this command exists to prevent, wearing a green tick.
//
// The source entity is resolved first so its type feeds the decision exactly
// as it does in production (best-effort there, required here — a report that
// silently answered for an empty FromType would attest to a different question
// than the one asked).
func (e *Engine) CanRelation(
	ctx context.Context, rawPrincipal string, verb acl.Verb, relType, fromID string,
) (*CanRelationResult, error) {
	op, ok := relationVerbs[verb]
	if !ok {
		return nil, fmt.Errorf("aclmap: %q is not a relation write verb (want create/update/delete)", verb)
	}
	relType = strings.TrimSpace(relType)
	if relType == "" {
		return nil, errors.New("aclmap: relation type must not be empty")
	}

	from, err := e.src.GetEntity(ctx, fromID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, fromID)
		}
		return nil, fmt.Errorf("aclmap: load entity %q: %w", fromID, err)
	}

	user, rawShown, err := e.resolveEffective(ctx, rawPrincipal)
	if err != nil {
		return nil, err
	}
	req, err := e.resolver.ForPrincipal(
		principal.Principal{User: user, Tool: principal.ToolCLI, RawUser: rawShown})
	if err != nil {
		return nil, fmt.Errorf("aclmap: open resolver for %q: %w", user, err)
	}

	d := req.AuthorizeWrite(ctx, acl.WriteRequest{
		Op: op,
		Subject: acl.RelationSubject{
			Type:     relType,
			FromType: from.Type,
			FromID:   fromID,
		},
	})

	return &CanRelationResult{
		SchemaVersion: schemaVersion,
		Principal:     user,
		Raw:           rawShown,
		Verb:          string(verb),
		Relation:      relType,
		From:          fromID,
		FromType:      from.Type,
		Allowed:       d.Allow,
		RuleKind:      d.RuleKind,
		RuleID:        d.RuleID,
		Reason:        d.Reason,
	}, nil
}
