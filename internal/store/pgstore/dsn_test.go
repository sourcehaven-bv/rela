package pgstore_test

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestDSNHelpers_KeepBaseSettings pins that PinSearchPath and WithDatabase
// change only their one setting. Rebuilding a DSN from a few parsed fields
// dropped sslmode=verify-full and pool_max_conns (BUG-BR9CXF).
func TestDSNHelpers_KeepBaseSettings(t *testing.T) {
	t.Parallel()

	bases := []struct{ name, base string }{
		{"url", "postgres://rela:p%40ss@db.example:6432/rela?sslmode=verify-full&pool_max_conns=7&application_name=x"},
		{"keyword", "host=db.example port=6432 user=rela password='p@ss' dbname=rela sslmode=verify-full " +
			"pool_max_conns=7 application_name=x"},
	}
	for _, tc := range bases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pinned, err := pgstore.PinSearchPath(tc.base, "tenant_a")
			if err != nil {
				t.Fatalf("PinSearchPath: %v", err)
			}
			moved, err := pgstore.WithDatabase(pinned, "other")
			if err != nil {
				t.Fatalf("WithDatabase: %v", err)
			}
			cfg, err := pgxpool.ParseConfig(moved)
			if err != nil {
				t.Fatalf("derived DSN does not parse: %v", err)
			}
			cc := cfg.ConnConfig
			if p := cc.RuntimeParams["search_path"]; p != "tenant_a,public" {
				t.Errorf("search_path = %q, want tenant_a,public", p)
			}
			if cc.Database != "other" {
				t.Errorf("database = %q, want other", cc.Database)
			}
			if cc.Host != "db.example" || cc.Port != 6432 || cc.User != "rela" || cc.Password != "p@ss" {
				t.Errorf("connection target changed: %s:%d %s", cc.Host, cc.Port, cc.User)
			}
			if cc.RuntimeParams["application_name"] != "x" {
				t.Error("application_name was dropped")
			}
			if cfg.MaxConns != 7 {
				t.Errorf("pool_max_conns = %d, want 7", cfg.MaxConns)
			}
			if cc.TLSConfig == nil || cc.TLSConfig.InsecureSkipVerify || len(cc.Fallbacks) != 0 {
				t.Error("derived DSN lost verify-full TLS")
			}
		})
	}
}

// TestPinSearchPath_RefusesOtherSpellings pins that a base DSN cannot set
// search_path a second way that could win over the pinned one.
func TestPinSearchPath_RefusesOtherSpellings(t *testing.T) {
	t.Parallel()

	for _, base := range []string{
		"postgres://rela@db.example/rela?SEARCH_PATH=public",
		"host=db.example user=rela dbname=rela Search_Path=public",
		"postgres://rela@db.example/rela?options=-c%20search_path%3Dpublic",
	} {
		if _, err := pgstore.PinSearchPath(base, "tenant_a"); err == nil {
			t.Errorf("PinSearchPath(%q) accepted a second search_path", base)
		}
	}
}

// TestPinSearchPath_OverridesBasePath pins that a search_path already in the
// base is replaced, not appended to.
func TestPinSearchPath_OverridesBasePath(t *testing.T) {
	t.Parallel()

	for _, base := range []string{
		"postgres://rela@db.example/rela?search_path=other",
		"host=db.example user=rela dbname=rela search_path=other",
	} {
		dsn, err := pgstore.PinSearchPath(base, "tenant_a")
		if err != nil {
			t.Fatalf("PinSearchPath(%q): %v", base, err)
		}
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		if p := cfg.RuntimeParams["search_path"]; p != "tenant_a,public" {
			t.Errorf("%q: search_path = %q", base, p)
		}
	}
}
