//go:build postgres

package docscapture

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// scratchDropTimeout bounds the scratch-schema DROP at cleanup.
const scratchDropTimeout = 30 * time.Second

// scratchBackend creates a PRIVATE, empty PostgreSQL schema for one docs build
// and returns the appbuild options that pin the stood-up temp project to it,
// plus a cleanup that drops it.
//
// # Why the postgres docs build needs this at all
//
// A manual's create()/face()/link() seed is a FIXTURE. On the fs build that is
// self-evidently harmless: appbuild.Discover opens an fsstore under a temp dir
// standUp just made. On the postgres build the same call binds pgstore to
// $RELA_DATABASE_URL, so without this the seed would write fixture entities
// into whatever database the operator happens to have configured — which is
// why screenshot{} and api{} used to REFUSE to run on this build at all.
//
// Refusing was the right call while nothing scoped the writes. This scopes
// them: a schema named for this process, created empty, migrated from scratch
// by pgstore's own auto-migrate on first open, and dropped CASCADE at
// teardown. The operator's tables are never on the search_path ahead of it,
// and nothing outside the scratch schema is written or read.
//
// It is deliberately the same mechanism internal/appbuild/backendtest uses to
// isolate the wiring tests, for the same reason and with the same failure
// modes. Both derive the DSN with pgstore.PinSearchPath.
//
// Nil: the returned cleanup is never nil — callers may defer it unconditionally.
func scratchBackend(ctx context.Context, _ string) ([]appbuild.Option, func(), error) {
	base := os.Getenv("RELA_DATABASE_URL")
	if base == "" {
		return nil, func() {}, errors.New(
			"the postgres docs build needs a database to build a scratch schema in: " +
				"set RELA_DATABASE_URL (env-only, never a flag)")
	}

	admin, err := pgstore.ConnectDirect(ctx, base)
	if err != nil {
		return nil, func() {}, fmt.Errorf("connect to RELA_DATABASE_URL: %w", err)
	}

	schema, err := scratchSchemaName()
	if err != nil {
		_ = admin.Close(ctx)
		return nil, func() {}, err
	}
	// DDL takes no bind parameters; schema is minted hex and quoted.
	// nosemgrep: go.lang.security.audit.sqli.pgx-sqli.pgx-sqli
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema)); err != nil {
		_ = admin.Close(ctx)
		return nil, func() {}, fmt.Errorf("create scratch schema %s: %w", schema, err)
	}

	cleanup := func() {
		// Detached: cleanup runs at close, after the caller's context may be
		// done. Bounded so a hung DROP cannot block Close forever.
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), scratchDropTimeout)
		defer cancel()
		// nosemgrep: go.lang.security.audit.sqli.pgx-sqli.pgx-sqli
		if _, derr := admin.Exec(bg, "DROP SCHEMA "+quoteIdent(schema)+" CASCADE"); derr != nil {
			// Worth reporting rather than swallowing: a surviving scratch
			// schema is clutter in the operator's database that nothing else
			// will ever clean up, and the name is what they need to drop it.
			slog.Warn("docscapture: could not drop scratch schema",
				"schema", schema, "error", derr)
		}
		_ = admin.Close(bg)
	}

	dsn, err := dsnWithSearchPath(base, schema)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return []appbuild.Option{appbuild.WithDatabaseURL(dsn)}, cleanup, nil
}

// scratchSchemaName mints a collision-free schema name. The random suffix (not
// just a PID) is what makes two concurrent docs builds against one database
// safe — a PID is reused, and a stale schema left by a killed build would
// otherwise be adopted by a later one and serve its fixtures.
func scratchSchemaName() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("mint scratch schema name: %w", err)
	}
	return "reladocs_" + hex.EncodeToString(b[:]), nil
}

// dsnWithSearchPath pins search_path to the scratch schema, keeping public on
// the path for pg_trgm, and keeps every other setting of base, TLS included
// (BUG-BR9CXF).
func dsnWithSearchPath(base, schema string) (string, error) {
	dsn, err := pgstore.PinSearchPath(base, schema)
	if err != nil {
		return "", fmt.Errorf("RELA_DATABASE_URL: %w", err)
	}
	return dsn, nil
}

// quoteIdent renders the generated schema name as a quoted identifier. The
// name is built from a hex-encoded random suffix so it cannot contain a quote;
// the doubling is belt-and-braces.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
