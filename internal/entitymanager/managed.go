package entitymanager

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ruleKindManaged is the [acl.Decision.RuleKind] of a write refused because
// it touches a rela-managed type or relation ([entity.IsManagedName]).
const ruleKindManaged = "managed-type"

// managedWriteDenial reports why req may not run, or "" when it may. It is
// the write-path integrity rule of flow runs (TKT-BC5G8RP):
//
//   - only [principal.UserFlow] writes a managed entity or relation;
//   - nobody renames or converts a managed entity, or converts an entity
//     into a managed type.
//
// It holds on every Manager handle: an elevated (`bypass_acl`) or cascade
// write, and a project with no acl.yaml ([acl.NopACL]), are refused too.
// The compiled ACL grants the same allowlist ([acl.FlowEngineRole]); this
// rule is what keeps it when the ACL is skipped or absent.
func managedWriteDenial(ctx context.Context, req acl.WriteRequest) string {
	var name string
	switch s := req.Subject.(type) {
	case acl.EntitySubject:
		if entity.IsManagedName(s.ConvertTo()) {
			return "an entity cannot be converted into " + s.ConvertTo()
		}
		if !entity.IsManagedName(s.Type()) {
			return ""
		}
		if req.Op == acl.OpRename || req.Op == acl.OpConvert {
			return fmt.Sprintf("a %s entity cannot be %sd", s.Type(), req.Op)
		}
		name = s.Type()
	case acl.RelationSubject:
		switch {
		case entity.IsManagedName(s.Type):
			name = s.Type
		case entity.IsManagedName(s.FromType):
			name = s.FromType
		default:
			return ""
		}
	default:
		return ""
	}
	if principal.From(ctx).User == principal.UserFlow {
		return ""
	}
	return "only the flow engine writes " + name
}

// refuseManagedWrite applies [managedWriteDenial]: on a denial it records a
// `denied-write` audit row (not for an affordance probe) and returns an
// [*acl.ForbiddenError].
func refuseManagedWrite(ctx context.Context, m *Manager, req acl.WriteRequest) error {
	reason := managedWriteDenial(ctx, req)
	if reason == "" {
		return nil
	}
	decision := acl.Decision{Reason: reason, RuleKind: ruleKindManaged, RuleID: "-"}
	if !isAffordanceProbe(ctx) {
		m.deps.Audit.Record(deniedWriteRecord(ctx, decision, req))
	}
	return &acl.ForbiddenError{Decision: decision}
}
