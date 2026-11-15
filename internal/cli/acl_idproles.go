package cli

import (
	"context"
	"errors"
	"fmt"
)

// idpRoleForgetter erases the IdP roles stored for one user entity.
// appbuild.IDPRoleWriter satisfies it.
type idpRoleForgetter interface {
	Forget(ctx context.Context, entityID string) (int, error)
}

// ACLForgetIDPRolesCmd erases the IdP roles the webhook receiver stored for
// one person, for an erasure request (GH-2118). It takes the entity id, not
// the IdP subject, so it also reaches a record whose person was already
// deleted.
type ACLForgetIDPRolesCmd struct {
	ID string `arg:"" help:"User entity id the roles were stored for."`
}

// Run executes `rela acl forget-idp-roles`.
func (c *ACLForgetIDPRolesCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.IDPRoles == nil {
		return errors.New("this project has no store of IdP roles")
	}
	n, err := svc.IDPRoles.Forget(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("forget IdP roles of %s: %w", c.ID, err)
	}
	if n == 0 {
		out.WriteSuccess("No IdP roles were stored for %s.", c.ID)
		return nil
	}
	out.WriteSuccess("Erased the IdP roles stored for %s.", c.ID)
	return nil
}
