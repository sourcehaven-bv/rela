package acl

import "context"

// NopACL is the explicit opt-out: allows every write. Wired by
// appbuild when no acl.yaml is present (PR 3) so projects that don't
// care about access control run unchanged.
//
// "Explicit opt-out" matters: [entitymanager.Deps] rejects a nil ACL
// at construction, so callers must wire NopACL by name when they
// actually want allow-all. Defaulting nil to NopACL would silently
// disable the gate if the wiring forgot a step.
type NopACL struct{}

// AuthorizeWrite always returns Allow=true.
func (NopACL) AuthorizeWrite(_ context.Context, _ WriteRequest) Decision {
	return Decision{Allow: true}
}
