package tenant

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mapControl is a control table in memory, keyed by schema.
type mapControl struct {
	rows map[string]Record
	err  error
}

func (m mapControl) lookup(_ context.Context, schema string) (Record, bool, error) {
	if m.err != nil {
		return Record{}, false, m.err
	}
	rec, ok := m.rows[schema]
	return rec, ok, nil
}

func (m mapControl) ready(context.Context) error { return m.err }

const clusterBase = "postgres://rela:hunter2@db.example:5432/rela?sslmode=verify-full"

// controlWith returns a control table with one row per record, keyed by the
// org's derived schema unless the record names one.
func controlWith(t *testing.T, recs ...Record) mapControl {
	t.Helper()
	m := mapControl{rows: map[string]Record{}}
	for _, r := range recs {
		if r.Schema == "" {
			s, err := SchemaFor(r.OrgID)
			if err != nil {
				t.Fatal(err)
			}
			r.Schema = s
		}
		m.rows[r.Schema] = r
	}
	return m
}

func TestSchemaFor(t *testing.T) {
	t.Parallel()

	a, err := SchemaFor("org-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := SchemaFor("org-b")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("two orgs derive the same schema %q", a)
	}
	again, _ := SchemaFor("org-a")
	if again != a {
		t.Errorf("SchemaFor is not stable: %q then %q", a, again)
	}
	for _, s := range []string{a, b} {
		if !strings.HasPrefix(s, "org_") || len(s) != len("org_")+16 || checkSchemaName(s) != nil {
			t.Errorf("derived schema %q is not a valid org_ name", s)
		}
	}
	// Org ids that are hostile as identifiers still derive a plain name.
	hostile := []string{`x"; DROP SCHEMA public; --`, "ORG-A", "ünïcode", strings.Repeat("z", maxOrgIDBytes)}
	for _, org := range hostile {
		s, err := SchemaFor(org)
		if err != nil || checkSchemaName(s) != nil {
			t.Errorf("SchemaFor(%q) = %q, %v", org, s, err)
		}
	}
	// An org id no tenant can have is an unknown tenant.
	for _, org := range invalidOrgIDs {
		if s, err := SchemaFor(org); !errors.Is(err, ErrUnknownTenant) || s != "" {
			t.Errorf("SchemaFor(%q) = %q, %v; want ErrUnknownTenant", org, s, err)
		}
	}
}

// invalidOrgIDs are org ids that checkOrgID refuses.
var invalidOrgIDs = []string{
	"", strings.Repeat("z", maxOrgIDBytes+1), string([]byte{0xff, 0xfe}), "a\x00b", "a\nb", "a\x7fb",
}

func TestCheckOrgID(t *testing.T) {
	t.Parallel()
	for org, ok := range map[string]bool{
		"org-a": true, "": false, strings.Repeat("a", 256): true, strings.Repeat("a", 257): false,
		"a\nb": false, "a\x00b": false, string([]byte{0xff, 0xfe}): false,
	} {
		if err := checkOrgID(org); (err == nil) != ok {
			t.Errorf("checkOrgID(%q) = %v, want ok=%v", org, err, ok)
		}
	}
}

// TestClusterResolver_Resolve pins the derived-tenant rule: only an active
// org whose schema exists is served, every other state fails closed, and a
// colliding derived schema is refused rather than shared.
func TestClusterResolver_Resolve(t *testing.T) {
	t.Parallel()

	ctrl := controlWith(t,
		Record{OrgID: "org-a", Status: StatusActive, SchemaExists: true},
		Record{OrgID: "org-b", Status: StatusActive, SchemaExists: true},
		Record{OrgID: "org-susp", Status: StatusSuspended, SchemaExists: true},
		Record{OrgID: "org-erasing", Status: StatusErasing, SchemaExists: true},
		Record{OrgID: "org-prov", Status: StatusProvisioning, SchemaExists: true},
		Record{OrgID: "org-noschema", Status: StatusActive},
	)
	r := newClusterResolver(clusterBase, ctrl, nil)

	// Pinning a DSN needs the postgres build; the refusals below hold on
	// every build.
	if Supported {
		a, err := r.Resolve(t.Context(), "org-a")
		if err != nil {
			t.Fatalf("Resolve(org-a): %v", err)
		}
		b, err := r.Resolve(t.Context(), "org-b")
		if err != nil {
			t.Fatalf("Resolve(org-b): %v", err)
		}
		if a.Schema == b.Schema || a.DSN == b.DSN {
			t.Fatalf("two orgs resolve to one schema: %+v %+v", a, b)
		}
		if a.OrgID != "org-a" || !strings.Contains(a.DSN, a.Schema) {
			t.Errorf("Resolve(org-a) = %+v; want the DSN pinned to its schema", a)
		}
		if !strings.Contains(a.DSN, "sslmode=verify-full") {
			t.Errorf("the tenant DSN dropped the base TLS setting: %q", a.DSN)
		}
	}

	for _, tc := range []struct {
		org     string
		unknown bool
	}{
		{"org-z", true},
		{"", true},
		{"org-susp", true},
		{"org-erasing", true},
		{"org-prov", true},
		{"org-noschema", false},
	} {
		got, err := r.Resolve(t.Context(), tc.org)
		if err == nil || got != (Tenant{}) {
			t.Errorf("Resolve(%q) = %+v, %v; want a refusal", tc.org, got, err)
			continue
		}
		if errors.Is(err, ErrUnknownTenant) != tc.unknown {
			t.Errorf("Resolve(%q) error %v: unknown=%v, want %v", tc.org, err, !tc.unknown, tc.unknown)
		}
		if strings.Contains(err.Error(), "hunter2") {
			t.Errorf("Resolve(%q) error names the DSN: %v", tc.org, err)
		}
	}
}

// TestClusterResolver_RefusesInvalidOrgID pins that the request path applies
// the same org id check as [Provisioner.Create]: an org id that check refuses
// is an unknown tenant, and the control table is not read for it.
func TestClusterResolver_RefusesInvalidOrgID(t *testing.T) {
	t.Parallel()

	ctrl := &countingControl{}
	r := newClusterResolver(clusterBase, ctrl, nil)
	for _, org := range invalidOrgIDs {
		if got, err := r.Resolve(t.Context(), org); !errors.Is(err, ErrUnknownTenant) || got != (Tenant{}) {
			t.Errorf("Resolve(%q) = %+v, %v; want ErrUnknownTenant", org, got, err)
		}
		if _, err := r.Lookup(t.Context(), org); !errors.Is(err, ErrUnknownTenant) {
			t.Errorf("Lookup(%q) = %v; want ErrUnknownTenant", org, err)
		}
	}
	if ctrl.lookups != 0 {
		t.Errorf("the control table was read %d times for invalid org ids", ctrl.lookups)
	}
}

// countingControl is an empty control table that counts its lookups.
type countingControl struct{ lookups int }

func (c *countingControl) lookup(context.Context, string) (Record, bool, error) {
	c.lookups++
	return Record{}, false, nil
}

func (c *countingControl) ready(context.Context) error { return nil }

// TestClusterResolver_Collision pins that an org whose derived schema is
// recorded for another org is refused, never served the other org's data.
func TestClusterResolver_Collision(t *testing.T) {
	t.Parallel()

	ctrl := controlWith(t, Record{OrgID: "org-a", Schema: "org_0000000000000001", Status: StatusActive,
		SchemaExists: true})
	r := newClusterResolver(clusterBase, ctrl, nil)
	// org-b derives org-a's schema, as a hash collision would.
	r.derive = func(string) (string, error) { return "org_0000000000000001", nil }

	if _, err := r.Lookup(t.Context(), "org-a"); err != nil {
		t.Fatalf("Lookup(org-a): %v", err)
	}
	got, err := r.Resolve(t.Context(), "org-b")
	if !errors.Is(err, ErrSchemaCollision) || got != (Tenant{}) {
		t.Fatalf("Resolve(colliding org) = %+v, %v; want ErrSchemaCollision", got, err)
	}
	if errors.Is(err, ErrUnknownTenant) {
		t.Errorf("a collision reads as an unknown tenant: %v", err)
	}
}

// TestClusterResolver_LookupFailure pins that a broken control lookup is an
// outage, never an unknown tenant.
func TestClusterResolver_LookupFailure(t *testing.T) {
	t.Parallel()

	r := newClusterResolver("postgres://rela@other.example/rela", mapControl{err: errors.New("connection refused")}, nil)
	_, err := r.Resolve(t.Context(), "org-a")
	if err == nil || errors.Is(err, ErrUnknownTenant) {
		t.Fatalf("Resolve over a broken control table = %v; want a non-unknown error", err)
	}
	if r.Check(t.Context()) == nil {
		t.Error("Check passed over a broken control table")
	}
	var nilResolver *ClusterResolver
	if err := nilResolver.Close(); err != nil {
		t.Errorf("Close on nil = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close without a closer = %v", err)
	}
	closed := closerFunc(func() error { return errors.New("pool closed twice") })
	withPool := newClusterResolver(clusterBase, mapControl{}, closed)
	if err := withPool.Close(); err == nil {
		t.Error("Close did not close the control pool")
	}
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// TestClusterResolver_Lookup pins that Lookup reports any status, for the
// operator commands.
func TestClusterResolver_Lookup(t *testing.T) {
	t.Parallel()

	r := newClusterResolver(clusterBase, controlWith(t, Record{OrgID: "org-s", Status: StatusSuspended}), nil)
	rec, err := r.Lookup(t.Context(), "org-s")
	if err != nil || rec.Status != StatusSuspended {
		t.Fatalf("Lookup(suspended) = %+v, %v", rec, err)
	}
	if _, err := r.Lookup(t.Context(), "org-z"); !errors.Is(err, ErrUnknownTenant) {
		t.Errorf("Lookup(unknown) = %v, want ErrUnknownTenant", err)
	}
}

// TestClusterResolver_Inactive pins which refusals are a tenant that is not
// served now, whose inbound events must be retried rather than dropped.
func TestClusterResolver_Inactive(t *testing.T) {
	t.Parallel()

	r := newClusterResolver(clusterBase, controlWith(t,
		Record{OrgID: "org-s", Status: StatusSuspended, SchemaExists: true},
		Record{OrgID: "org-p", Status: StatusProvisioning},
		Record{OrgID: "org-e", Status: StatusErasing, SchemaExists: true},
	), nil)
	for org, inactive := range map[string]bool{"org-s": true, "org-p": true, "org-e": false, "org-z": false} {
		_, err := r.Resolve(t.Context(), org)
		if !errors.Is(err, ErrUnknownTenant) {
			t.Errorf("Resolve(%s) = %v, want ErrUnknownTenant", org, err)
		}
		if errors.Is(err, ErrTenantInactive) != inactive {
			t.Errorf("Resolve(%s) inactive = %v, want %v", org, !inactive, inactive)
		}
	}
}
