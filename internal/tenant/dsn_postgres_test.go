//go:build postgres

package tenant

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestDSNForSchema_KeepsBaseSettings pins that deriving a tenant DSN changes
// only search_path. Rebuilding it from a few parsed fields dropped TLS
// settings such as sslmode=verify-full, which silently weakened every
// tenant's connection.
func TestDSNForSchema_KeepsBaseSettings(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, base string }{
		{"url", "postgres://rela@db.example:6432/rela?sslmode=verify-full&search_path=other"},
		{"keyword", "host=db.example port=6432 user=rela dbname=rela sslmode=verify-full search_path=other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := dsnForSchema(tc.base, "tenant_a")
			if err != nil {
				t.Fatalf("dsnForSchema: %v", err)
			}
			want, err := pgx.ParseConfig(tc.base)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := pgx.ParseConfig(got)
			if err != nil {
				t.Fatalf("derived DSN does not parse: %v", err)
			}
			if p := cfg.RuntimeParams["search_path"]; p != "tenant_a,public" {
				t.Errorf("search_path = %q, want tenant_a,public", p)
			}
			if cfg.Host != want.Host || cfg.Port != want.Port || cfg.User != want.User || cfg.Database != want.Database {
				t.Errorf("connection target changed: %s:%d %s/%s", cfg.Host, cfg.Port, cfg.User, cfg.Database)
			}
			// verify-full: TLS with certificate checks and no plaintext
			// fallback, which is what the old derivation's default (prefer)
			// would have added.
			if cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify || len(cfg.Fallbacks) != 0 {
				t.Error("derived DSN lost the base's verify-full TLS settings")
			}
		})
	}
}

// TestDSNForSchema_RefusesOtherSearchPathSpellings pins that a base DSN
// cannot set search_path a second way that could win over the pinned one on
// some connections.
func TestDSNForSchema_RefusesOtherSearchPathSpellings(t *testing.T) {
	t.Parallel()

	for _, base := range []string{
		"postgres://rela@db.example/rela?SEARCH_PATH=public",
		"host=db.example user=rela dbname=rela Search_Path=public",
		"postgres://rela@db.example/rela?options=-c%20search_path%3Dpublic",
	} {
		if _, err := dsnForSchema(base, "tenant_a"); err == nil {
			t.Errorf("dsnForSchema(%q) accepted a second search_path", base)
		}
	}
}
