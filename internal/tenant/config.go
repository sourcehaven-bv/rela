package tenant

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

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
	// tenant's own DSN is derived from it by pinning `search_path`. It may not
	// carry a password (see LoadConfig).
	//
	// Left empty when every tenant carries an explicit DSN, which is the
	// fully-sharded arrangement.
	BaseDSN string `yaml:"base_dsn"`

	// Tenants is the map itself.
	Tenants []ConfigTenant `yaml:"tenants"`

	// MaxResident bounds how many tenants are open at once. Zero or less
	// takes [DefaultMaxResident]. Each open tenant holds its own connection
	// pool, so this is the number to size against the cluster's
	// max_connections.
	MaxResident int `yaml:"max_resident"`

	// MaxOpen is the hard ceiling on open tenants, counting those a
	// long-lived request keeps open past MaxResident. Zero or less takes
	// twice MaxResident. See [WithMaxOpen].
	MaxOpen int `yaml:"max_open"`
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

	// Erase marks the tenant for erasure. A marked tenant is not served: its
	// org fails closed as an unknown org does. It stays in the map so its
	// schema name stays reserved and `rela-server tenant erase` can find
	// it. See [Erase].
	Erase bool `yaml:"erase"`
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
	if err := cfg.refuseFilePasswords(); err != nil {
		return nil, fmt.Errorf("tenant: %s: %w", path, err)
	}
	return &cfg, nil
}

// refuseFilePasswords refuses a password written into the tenant map.
//
// The map lives in the project directory with the rest of the operator
// config, which is not a secret and is routinely committed. A database
// password does not belong there, the same rule that keeps a literal
// password out of `mail.yaml` and the DSN in $RELA_DATABASE_URL. Give the
// base DSN in that variable, or keep the password in PGPASSWORD or a pgpass
// file, which pgx reads for any DSN without one.
func (c *Config) refuseFilePasswords() error {
	if dsnHasPassword(c.BaseDSN) {
		return errors.New("base_dsn carries a password; " + dsnHint)
	}
	for i, ct := range c.Tenants {
		if dsnHasPassword(ct.DSN) {
			return fmt.Errorf("tenant %d (org_id %q): dsn carries a password; %s", i, ct.OrgID, dsnHint)
		}
	}
	return nil
}

const dsnHint = "leave base_dsn out and set $RELA_DATABASE_URL, or use PGPASSWORD or a pgpass file"

// keywordPassword matches a password keyword in a key/value DSN.
var keywordPassword = regexp.MustCompile(`(?:^|\s)password\s*=`)

// dsnHasPassword reports whether dsn spells out a password, in either DSN
// dialect. It reads the string only: a password pgx would find in
// PGPASSWORD or a pgpass file is not in the file, so it is not refused.
func dsnHasPassword(dsn string) bool {
	if dsn == "" {
		return false
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			// Unparseable: refuse rather than guess, since the string may
			// hold a password the parse could not find.
			return true
		}
		if _, ok := u.User.Password(); ok {
			return true
		}
		return u.Query().Has("password")
	}
	return keywordPassword.MatchString(dsn)
}

// Resolver turns the parsed config into a live lookup, deriving each tenant's
// DSN and validating the whole table.
//
// Tenants marked `erase: true` are validated with the rest, so a marked
// tenant still reserves its schema, but the resolver does not serve them.
//
// An empty tenant list is refused, and so is a list in which every tenant is
// marked for erasure. A multi-tenant host with no tenants can serve no
// request, so booting it would only defer the same failure to every request,
// and would do so as a per-request denial, which reads like a permissions
// problem rather than a deployment one.
func (c *Config) Resolver() (*MapResolver, error) {
	all, err := c.tenants()
	if err != nil {
		return nil, err
	}
	served := make([]Tenant, 0, len(all))
	for i, t := range all {
		if !c.Tenants[i].Erase {
			served = append(served, t)
		}
	}
	if len(served) == 0 {
		return nil, errors.New("tenant: no tenants configured that are not marked for erasure")
	}
	return NewMapResolver(served)
}

// ErasureTarget returns the tenant for orgID that [Erase] may erase.
//
// The org must be in the map and marked `erase: true`. The mark is what
// stops a server that loads this file from serving the org, so an unmarked
// org is refused rather than erased under a server that still routes to it.
// The whole table is validated first, as [Config.Resolver] does, so a target
// is never taken from a map the server would refuse to load.
func (c *Config) ErasureTarget(orgID string) (Tenant, error) {
	all, err := c.tenants()
	if err != nil {
		return Tenant{}, err
	}
	for i, t := range all {
		if t.OrgID != orgID {
			continue
		}
		if !c.Tenants[i].Erase {
			return Tenant{}, fmt.Errorf("%w: org_id %q", ErrNotMarkedForErasure, orgID)
		}
		return t, nil
	}
	return Tenant{}, fmt.Errorf("%w: %q", ErrUnknownTenant, orgID)
}

// tenants derives every tenant's DSN, in file order, and validates the whole
// table with [NewMapResolver].
func (c *Config) tenants() ([]Tenant, error) {
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
			if err := checkSchemaName(ct.Schema); err != nil {
				return nil, fmt.Errorf("tenant %d (org_id %q): %w", i, ct.OrgID, err)
			}
			var err error
			if dsn, err = dsnForSchema(c.BaseDSN, ct.Schema); err != nil {
				return nil, fmt.Errorf("tenant %d (org_id %q): %w", i, ct.OrgID, err)
			}
		}
		tenants = append(tenants, Tenant{OrgID: ct.OrgID, Schema: ct.Schema, DSN: dsn})
	}
	if _, err := NewMapResolver(tenants); err != nil {
		return nil, err
	}
	return tenants, nil
}
