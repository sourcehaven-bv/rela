package tenant

import (
	"errors"
	"fmt"
)

// Provisioner makes the lifecycle writes to the control table: create,
// suspend, resume and erase. It is a separate type from [ClusterResolver] so
// that the request path, which receives only a [Resolver], cannot reach a
// write. The CLI uses it, and so does the tenant host when
// `-tenant-provision` is on (TKT-TNPRV8).
//
// Every status change is a compare-and-set on the current status, so two
// operators cannot undo each other: a resume cannot turn an erasing tenant
// back into an active one.
//
// The provisioner connects with the same base DSN, and so the same database
// role, as the server. The tables a create migrates then belong to the role
// the server reads and writes them as.
type Provisioner struct {
	base      string
	auditRoot string
	derive    func(string) (string, error)
}

// NewProvisioner builds a provisioner over baseDSN ($RELA_DATABASE_URL).
// auditRoot is [AuditRoot] of the project's cache directory, where [Erase]
// removes a tenant's audit directory.
//
// Nil: rejected — an empty base DSN or audit root is an error.
func NewProvisioner(baseDSN, auditRoot string) (*Provisioner, error) {
	if !Supported {
		return nil, ErrUnsupportedBuild
	}
	if baseDSN == "" {
		return nil, errors.New("tenant: the base DSN is required; set $RELA_DATABASE_URL")
	}
	if auditRoot == "" {
		return nil, errors.New("tenant: audit root is required")
	}
	return &Provisioner{base: baseDSN, auditRoot: auditRoot, derive: SchemaFor}, nil
}

// CreateOptions are the operator's choices for [Provisioner.Create]. The
// zero value is what the tenant host passes: take over nothing.
type CreateOptions struct {
	// Adopt takes over a schema that already exists with the derived name
	// but has no control row.
	Adopt bool
	// Recreate turns an erased tenant's tombstone back into a fresh, empty
	// tenant. Without it a create for an erased org fails with
	// [ErrTenantErased].
	Recreate bool
}

// ErrTenantExists reports a create for an org that already has a control
// row that is not mid-provisioning.
var ErrTenantExists = errors.New("tenant: already exists")

// ExistsError is the [ErrTenantExists] a create returns. Status is the
// existing row's status, so a caller can tell an active tenant from a
// suspended or erasing one.
type ExistsError struct {
	OrgID  string
	Status Status
}

func (e *ExistsError) Error() string {
	return fmt.Sprintf("%v: org_id %q is %s", ErrTenantExists, e.OrgID, e.Status)
}

// Unwrap returns [ErrTenantExists].
func (e *ExistsError) Unwrap() error { return ErrTenantExists }

// ErrSchemaExists reports a create whose derived schema already exists
// without a control row. It may hold another org's data, so it is not taken
// over unless the operator asks to adopt it.
var ErrSchemaExists = errors.New("tenant: the schema already exists without a control row")

// ErrWrongStatus reports a status change the tenant's current status does
// not allow.
var ErrWrongStatus = errors.New("tenant: not allowed in the current status")

// ErrOperatorSuspended reports that the IdP asked to resume a tenant an
// operator suspended. Only an operator resumes it.
var ErrOperatorSuspended = errors.New("tenant: suspended by an operator")
