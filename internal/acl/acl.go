// Package acl provides principal-aware authorization for entity and
// relation writes. It is consulted by [entitymanager.Manager] before
// every write and (in future PRs) by read paths in the data-entry
// server and MCP transport.
//
// The contract is one method on the [ACL] interface. Three sibling
// implementations live in this package:
//
//   - [NopACL] allows every write. Default when no acl.yaml is present.
//   - [ReadOnlyACL] denies every write with a fixed Decision. Wired by
//     `rela-server --read-only` for demos, maintenance, and observe-only
//     deployments.
//   - Declarative (PR 2): policy-driven implementation reading roles
//     and assignments from acl.yaml.
//
// Manager always calls [ACL.AuthorizeWrite] before any store mutation;
// on deny it returns [*ForbiddenError] and records a `denied-write`
// audit row. The data-entry HTTP handler maps that error to a
// structured 403 response.
package acl

import (
	"context"
	"errors"
	"fmt"
)

// ACL gates writes. Implementations decide whether a [WriteRequest]
// from the principal carried on ctx is allowed, and explain *why* in
// the returned [Decision] so the deny path is debuggable end-to-end
// (audit log, HTTP 403 body, server logs).
type ACL interface {
	AuthorizeWrite(ctx context.Context, req WriteRequest) Decision
}

// WriteRequest describes the operation an ACL is being asked to
// authorize. The caller (typically [entitymanager.Manager]) populates
// either EntityType (entity ops) or both EntityType and RelationType
// (relation ops, where EntityType is the *source* entity's type).
//
// The split lets policy authors gate writes by the entity types a
// principal can mutate while a separate `role_relations` map (PR 2)
// gates role-conferring relation writes via the delegate-X
// permission check.
type WriteRequest struct {
	Op           Op
	EntityType   string
	RelationType string
}

// Op identifies the verb being requested.
type Op string

// Op constants — stable wire contract; surfaces in audit summaries and
// HTTP 403 bodies.
const (
	OpCreate Op = "create"
	OpUpdate Op = "update"
	OpDelete Op = "delete"
	OpRename Op = "rename"
)

// Decision is the ACL's answer plus enough context to debug a deny.
// Every deny names the rule that fired (RuleKind + RuleID) — opaque
// denials are unsupportable at scale (the AWS IAM lesson).
type Decision struct {
	Allow bool

	// RuleKind classifies the gate that fired. Stable values:
	// "role-grant" (a role's write list either matched or didn't),
	// "delegate-permission" (a role-relation requires a permission
	// the principal doesn't hold), "read-only" (ReadOnlyACL).
	RuleKind string

	// RuleID identifies the specific rule within RuleKind. Role name
	// for role-grant, permission name for delegate-permission,
	// "read-only-acl" for read-only. "-" when no rule applied
	// (deny by default).
	RuleID string

	// Reason is the human-readable explanation. Constructed by the
	// ACL; never contains raw policy data so 403 bodies don't leak
	// the full effective-role set.
	Reason string
}

// ErrForbidden is the sentinel that [ForbiddenError] reports via
// [errors.Is]. Use it in `errors.Is(err, acl.ErrForbidden)` checks at
// the HTTP boundary.
var ErrForbidden = errors.New("forbidden")

// ForbiddenError wraps a deny [Decision] so callers can surface
// RuleKind / RuleID / Reason. Returned by [entitymanager.Manager]
// from every write entry point on deny.
type ForbiddenError struct {
	Decision Decision
}

func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("forbidden: %s (rule_kind=%s rule_id=%s)",
		e.Decision.Reason, e.Decision.RuleKind, e.Decision.RuleID)
}

// Is reports whether target is [ErrForbidden]. Lets callers write
// `errors.Is(err, acl.ErrForbidden)` without knowing the concrete
// error type.
func (e *ForbiddenError) Is(target error) bool {
	return target == ErrForbidden
}
