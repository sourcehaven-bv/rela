package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"
)

// This file holds the tenant API's lifecycle vocabulary (TKT-59HWQ76): the
// access states a control plane sets, the plan it stores, and the usage it
// reads. The writes are on [Provisioner]; the reads on [ClusterResolver].

// AccessState is how the tenant API asks a tenant to be served.
type AccessState string

// The states the tenant API sets. Active serves the tenant normally;
// read_only serves it but refuses writes; suspended refuses every request
// and keeps the data.
const (
	StateActive    AccessState = "active"
	StateReadOnly  AccessState = "read_only"
	StateSuspended AccessState = "suspended"
)

// ParseAccessState returns the state s names, or an error for any other
// value.
func ParseAccessState(s string) (AccessState, error) {
	switch st := AccessState(s); st {
	case StateActive, StateReadOnly, StateSuspended:
		return st, nil
	}
	return "", fmt.Errorf("tenant: unknown state %q (want active, read_only or suspended)", s)
}

// Change is one tenant API update. A nil field is left as it is.
// Entitlements replaces the stored entitlements as a whole; an empty map
// clears them.
type Change struct {
	State        *AccessState
	Plan         *string
	Entitlements *Entitlements
}

// Validate refuses a change the control table must not store.
func (c Change) Validate() error {
	if c.State != nil {
		if _, err := ParseAccessState(string(*c.State)); err != nil {
			return err
		}
	}
	if c.Plan != nil {
		if len(*c.Plan) > maxPlanBytes {
			return fmt.Errorf("tenant: plan is longer than %d bytes", maxPlanBytes)
		}
		if !printable(*c.Plan) {
			return errors.New("tenant: plan holds a control character or is not UTF-8")
		}
	}
	if c.Entitlements != nil {
		return c.Entitlements.Validate()
	}
	return nil
}

// Entitlements are the limits and feature flags of a tenant's plan, keyed
// by name. A value is a number, a boolean or a string. rela stores and
// exposes them; it enforces none of them.
type Entitlements map[string]any

// The bounds of what the control table stores for a plan. They keep one
// tenant's row small; a plan needs a handful of entries.
const (
	maxPlanBytes             = 128
	maxEntitlements          = 64
	maxEntitlementValueBytes = 256
	// maxEntitlementNumberBytes keeps a number inside what the jsonb
	// column stores; a count or a limit needs a few digits.
	maxEntitlementNumberBytes = 32
)

// printable reports whether s is UTF-8 without control characters, which
// PostgreSQL text refuses (NUL) or a log line should not carry.
func printable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// entitlementName constrains an entitlement key: the snake_case names a
// plan catalog uses, such as `max_users`.
var entitlementName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Validate refuses entitlements with a bad name, a value that is not a
// number, boolean or short string, or too many entries.
func (e Entitlements) Validate() error {
	if len(e) > maxEntitlements {
		return fmt.Errorf("tenant: more than %d entitlements", maxEntitlements)
	}
	for k, v := range e {
		if !entitlementName.MatchString(k) {
			return fmt.Errorf("tenant: entitlement name %q must match %s", k, entitlementName)
		}
		switch val := v.(type) {
		case bool:
		case float64:
			if math.IsNaN(val) || math.IsInf(val, 0) {
				return fmt.Errorf("tenant: entitlement %q is not a finite number", k)
			}
		case json.Number:
			if len(val) > maxEntitlementNumberBytes {
				return fmt.Errorf("tenant: entitlement %q is a number longer than %d digits", k, maxEntitlementNumberBytes)
			}
		case string:
			if len(val) > maxEntitlementValueBytes {
				return fmt.Errorf("tenant: entitlement %q is longer than %d bytes", k, maxEntitlementValueBytes)
			}
			if !printable(val) {
				return fmt.Errorf("tenant: entitlement %q holds a control character or is not UTF-8", k)
			}
		default:
			return fmt.Errorf("tenant: entitlement %q must be a number, a boolean or a string", k)
		}
	}
	return nil
}

// ErrArchived reports a change to an archived tenant. Only an operator
// resumes it (`rela-server tenant resume`) or erases it.
var ErrArchived = errors.New("tenant: archived")

// ErrHeldSuspended reports that the tenant API asked to serve a tenant
// that an operator or the IdP suspended. Only they resume it.
var ErrHeldSuspended = errors.New("tenant: suspended by an operator or the IdP")

// Usage is what a tenant uses, for invoicing.
type Usage struct {
	// Users counts the entities of the user type: everyone who has used
	// the tenant at least once, as lazy provisioning creates them.
	Users int
	// ActiveUsers counts the distinct people who made a change in the
	// last [ActiveWindow], from the tenant's audit log. Reads are not
	// audited, so a reader who changes nothing is not counted.
	ActiveUsers int
	// StorageBytes is the on-disk size of the tenant's schema: tables,
	// indexes and TOAST, which hold entities, history, search and
	// attachment bytes.
	StorageBytes int64
	// MeasuredAt is when the numbers were read.
	MeasuredAt time.Time
}

// ActiveWindow is the period [Usage.ActiveUsers] counts.
const ActiveWindow = 30 * 24 * time.Hour

// usageReader reads a tenant's usage. The postgres control reader
// implements it.
type usageReader interface {
	usage(ctx context.Context, schema, userType string, since time.Time) (Usage, error)
}

// Usage reads orgID's usage. userType is the project's user entity type;
// empty counts no users. It refuses as [ClusterResolver.Lookup] does, and
// an org whose schema does not exist (being created, or erased) is an
// error that wraps [ErrUnknownTenant].
func (r *ClusterResolver) Usage(ctx context.Context, orgID, userType string, now time.Time) (Usage, error) {
	u, ok := r.control.(usageReader)
	if !ok {
		return Usage{}, ErrUnsupportedBuild
	}
	rec, err := r.Lookup(ctx, orgID)
	if err != nil {
		return Usage{}, err
	}
	if rec.Status == StatusProvisioning {
		return Usage{}, fmt.Errorf("%w: %w: org_id %q is still being created",
			ErrWrongStatus, ErrTenantProvisioning, orgID)
	}
	if !rec.SchemaExists {
		return Usage{}, fmt.Errorf("%w: org_id %q has no schema (%s)", ErrUnknownTenant, orgID, rec.Status)
	}
	got, err := u.usage(ctx, rec.Schema, userType, now.Add(-ActiveWindow))
	if err != nil {
		return Usage{}, fmt.Errorf("tenant: usage of org_id %q: %w", orgID, err)
	}
	got.MeasuredAt = now.UTC()
	return got, nil
}
