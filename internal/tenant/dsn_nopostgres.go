//go:build !postgres

package tenant

import (
	"context"
	"errors"
	"io"
)

// dsnForSchema refuses to derive a per-tenant DSN on a build with no PostgreSQL.
//
// Split from the postgres implementation for the reason `pgstore` is:
// deriving a DSN needs `pgx.ParseConfig` to handle both DSN dialects safely,
// and importing pgx here unconditionally would link it into the default and
// memory builds. CI asserts it is absent from those binaries, and the isolation
// is worth keeping even while nothing wires this package yet — the alternative
// is a violation that only appears once a host does.
//
// It errors rather than returning the base DSN unchanged, which is the
// fail-closed choice: an unpinned DSN would put every tenant on the same
// `search_path` and hand them each other's data. Refusing to boot is the only
// safe way to be wrong here.
//
// A deployment on these builds is single-store by construction (fsstore or
// memstore), so tenant mode is meaningless rather than merely unsupported.
func dsnForSchema(_, _ string) (string, error) {
	return "", errors.New(
		"tenant: deriving a per-tenant DSN requires the postgres build; " +
			"schema-per-tenant has no meaning for the filesystem or memory stores")
}

// pinSchema refuses on a build with no PostgreSQL, for the reason
// dsnForSchema does: there is no tenant schema to land in.
func pinSchema(context.Context, string, string) (io.Closer, error) {
	return nil, errors.New("tenant: checking a tenant schema requires the postgres build")
}

// dropSchema refuses on a build with no PostgreSQL: there is no tenant
// schema to drop.
func dropSchema(context.Context, string, string) (bool, error) {
	return false, errors.New("tenant: erasing a tenant requires the postgres build")
}

// DescribeTarget has no DSN to parse on a build with no PostgreSQL.
func DescribeTarget(string) string { return "a database this build cannot reach" }

// Supported reports whether this build can serve tenant mode. The
// filesystem, memory and sqlite stores keep their data in the project
// directory, so every tenant would read the same data.
const Supported = false

// NewClusterResolver refuses on a build with no PostgreSQL: there is no
// control table to read.
func NewClusterResolver(context.Context, string) (*ClusterResolver, error) {
	return nil, ErrUnsupportedBuild
}

// The lifecycle methods exist on every build so the commands compile;
// [NewProvisioner] refuses to build a Provisioner here, so none is reached.

// Create refuses on a build with no PostgreSQL.
func (*Provisioner) Create(context.Context, string, bool) (Tenant, error) {
	return Tenant{}, ErrUnsupportedBuild
}

// Suspend refuses on a build with no PostgreSQL.
func (*Provisioner) Suspend(context.Context, string) error { return ErrUnsupportedBuild }

// Resume refuses on a build with no PostgreSQL.
func (*Provisioner) Resume(context.Context, string) error { return ErrUnsupportedBuild }

// Erase refuses on a build with no PostgreSQL.
func (*Provisioner) Erase(context.Context, string) (Tenant, Erasure, error) {
	return Tenant{}, Erasure{}, ErrUnsupportedBuild
}

// CheckBareTarget has no database to check on a build with no PostgreSQL.
func CheckBareTarget(context.Context, string) error { return nil }

// RefuseTenantDatabase has no database to check on a build with no
// PostgreSQL.
func RefuseTenantDatabase(context.Context, string) error { return nil }
