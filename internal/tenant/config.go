package tenant

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the operator-authored tenant map, read from the project
// root alongside `acl.yaml` and `metamodel.yaml`.
const ConfigFileName = "tenants.yaml"

// Config is the parsed `tenants.yaml`.
//
// # Why the map is a file
//
// RES-D54281 decided the tenant map is rela-owned — not a JWT claim, not the
// identity provider's — and left the storage open. It is a file because the map
// cannot live inside the thing it shards: a table in a tenant schema is
// circular, and a table in a control schema means the process needs a DSN in
// order to find DSNs, so there is always a bootstrap-config layer underneath.
// A file *is* that layer. Backing the map with a database does not remove the
// file; it adds a database and keeps the file.
//
// It also matches the deployment this design is built around, where the
// operator already authors exactly one config — one metamodel, one `acl.yaml`,
// one `data-entry.yaml` — and every tenant arrives through an operator action
// because self-serve provisioning (TKT-TNPRV8) does not exist yet. A
// control-schema table today would be infrastructure built for a writer that
// has not been designed, which is the wrong order.
//
// What makes this reversible is [Resolver], not this type: when provisioning
// needs to add a tenant without a deploy, a DB-backed resolver replaces this
// one behind the same single-method seam.
type Config struct {
	// BaseDSN connects to the cluster holding the shared-tier tenants. Each
	// tenant's own DSN is derived from it by pinning `search_path`.
	//
	// Left empty when every tenant carries an explicit DSN, which is the
	// fully-sharded arrangement.
	BaseDSN string `yaml:"base_dsn"`

	// Tenants is the map itself.
	Tenants []ConfigTenant `yaml:"tenants"`
}

// ConfigTenant is one entry in the tenant map.
type ConfigTenant struct {
	// OrgID is the verified `org_id` claim.
	OrgID string `yaml:"org_id"`

	// Schema is the PostgreSQL schema holding this org's data.
	Schema string `yaml:"schema"`

	// DSN overrides the derivation from BaseDSN, placing this tenant on another
	// database or another cluster.
	//
	// This field is the sharding story in its entirety. RES-D54281 chose
	// schema-per-tenant partly because `(tenant) -> DSN` composes with it:
	// shard to a cluster, then pick a schema within it. Promoting a tenant that
	// outgrew the shared tier — or one whose contract requires isolation — is
	// then this one line, with no code path that only the promoted tenant
	// takes.
	DSN string `yaml:"dsn"`
}

// LoadConfig reads and parses a tenant map from path.
//
// A parse failure is an error rather than an empty map, for the reason
// `acl.yaml` loading gives: silently degrading to "no tenants" on a typo would
// boot a multi-tenant deployment that serves nobody, and the operator would
// learn about it from users rather than from the process that read the file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tenant: read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("tenant: parse %s: %w", path, err)
	}
	return &cfg, nil
}

// Resolver turns the parsed config into a live lookup, deriving each tenant's
// DSN and validating the whole table.
//
// An empty tenant list is refused. A multi-tenant host with no tenants can
// serve no request, so booting it would only defer the same failure to every
// request — and would do so as a per-request denial, which reads like a
// permissions problem rather than a deployment one.
func (c *Config) Resolver() (*MapResolver, error) {
	if len(c.Tenants) == 0 {
		return nil, errors.New("tenant: no tenants configured")
	}
	tenants := make([]Tenant, 0, len(c.Tenants))
	for i, ct := range c.Tenants {
		dsn := ct.DSN
		if dsn == "" {
			if c.BaseDSN == "" {
				return nil, fmt.Errorf(
					"tenant %d (org_id %q): no dsn and no base_dsn to derive one from",
					i, ct.OrgID)
			}
			// Validate the schema before it reaches DSN construction: the
			// derivation interpolates it into a connection string, so an
			// unchecked name would be building a credential out of unvalidated
			// input. NewMapResolver checks it again — this is the earlier of
			// the two, not a substitute for it.
			if !schemaNamePattern.MatchString(ct.Schema) {
				return nil, fmt.Errorf(
					"tenant %d (org_id %q): schema %q must match %s",
					i, ct.OrgID, ct.Schema, schemaNamePattern)
			}
			var err error
			if dsn, err = dsnForSchema(c.BaseDSN, ct.Schema); err != nil {
				return nil, fmt.Errorf("tenant %d (org_id %q): %w", i, ct.OrgID, err)
			}
		}
		tenants = append(tenants, Tenant{OrgID: ct.OrgID, Schema: ct.Schema, DSN: dsn})
	}
	return NewMapResolver(tenants)
}
