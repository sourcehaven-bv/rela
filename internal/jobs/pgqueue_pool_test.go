//go:build postgres

package jobs

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestEnsurePoolFloor pins that the queue guarantees enough connections for its
// own workers.
//
// pgx sizes a pool from GOMAXPROCS (min 4), which is unrelated to how many
// workers this queue runs: on a 2-core CI runner that is 4 connections against
// pgConcurrency=10. Blocked handlers then hold every connection and any further
// Enqueue fails with "exceeded timeout acquiring a connection from the pool" —
// a sizing bug that reads like a database outage. Caught exactly that way, by
// the postgres conformance suite on a 2-core runner.
func TestEnsurePoolFloor(t *testing.T) {
	t.Parallel()

	const floor = 14

	tests := []struct {
		name string
		dsn  string
		want string // expected pool_max_conns, "" means absent/unchanged
	}{
		{
			name: "absent is raised to the floor",
			dsn:  "postgres://u@h:5432/db?sslmode=disable",
			want: strconv.Itoa(floor),
		},
		{
			name: "too small is raised",
			dsn:  "postgres://u@h:5432/db?pool_max_conns=4",
			want: strconv.Itoa(floor),
		},
		{
			name: "an operator's larger value is respected",
			dsn:  "postgres://u@h:5432/db?pool_max_conns=50",
			want: "50",
		},
		{
			name: "exactly the floor is left alone",
			dsn:  "postgres://u@h:5432/db?pool_max_conns=" + strconv.Itoa(floor),
			want: strconv.Itoa(floor),
		},
		{
			name: "postgresql scheme is handled too",
			dsn:  "postgresql://u@h:5432/db",
			want: strconv.Itoa(floor),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ensurePoolFloor(tc.dsn, floor)
			u, err := url.Parse(got)
			require.NoError(t, err)
			require.Equal(t, tc.want, u.Query().Get("pool_max_conns"))
		})
	}
}

// TestEnsurePoolFloor_PreservesTheRestOfTheDSN pins that raising the pool size
// does not disturb credentials, host, database or other options — the DSN is
// rewritten, so anything dropped here becomes a connection failure at startup.
func TestEnsurePoolFloor_PreservesTheRestOfTheDSN(t *testing.T) {
	t.Parallel()

	got := ensurePoolFloor("postgres://user:secret@host:5432/mydb?sslmode=verify-ca", 14)

	u, err := url.Parse(got)
	require.NoError(t, err)
	require.Equal(t, "user", u.User.Username())
	pw, _ := u.User.Password()
	require.Equal(t, "secret", pw)
	require.Equal(t, "host:5432", u.Host)
	require.Equal(t, "/mydb", u.Path)
	require.Equal(t, "verify-ca", u.Query().Get("sslmode"))
	require.Equal(t, "14", u.Query().Get("pool_max_conns"))
}

// TestEnsurePoolFloor_NonURLDSNUntouched pins that a key/value DSN
// ("host=... dbname=...") — which pgx also accepts — is returned as-is rather
// than corrupted into a URL.
func TestEnsurePoolFloor_NonURLDSNUntouched(t *testing.T) {
	t.Parallel()

	const kv = "host=localhost dbname=rela sslmode=disable"
	require.Equal(t, kv, ensurePoolFloor(kv, 14))
}

// TestEnsurePoolFloor_PreservesOptionsEncoding pins that sizing the pool does
// not corrupt an operator's `options` parameter.
//
// The assertion goes through pgx rather than url.Query() on purpose: Query()
// decodes "+" back to a space, so it agrees with itself no matter what was
// written, and the whole defect is invisible to it. The server sees what pgx
// parses.
//
// The bug this pins: setting pool_max_conns through url.Values and re-encoding
// rewrote every other parameter into Go's encoding, where a space becomes "+".
// A URI query means a literal plus there, so libpq — and pgx from v5.11.0 on —
// read `-c+search_path=tenant` and the server rejected the parameter name
// "+search_path" with SQLSTATE 42704. A correct operator DSN became an
// unconnectable one, and only because a helper meant to size a pool.
func TestEnsurePoolFloor_PreservesOptionsEncoding(t *testing.T) {
	t.Parallel()

	const dsn = "postgres://u:p@h:5432/db?options=-c%20search_path%3Dtenant_a,public&sslmode=disable"

	before, err := pgconn.ParseConfig(dsn)
	require.NoError(t, err)
	require.Equal(t, "-c search_path=tenant_a,public", before.RuntimeParams["options"],
		"precondition: the fixture is a DSN pgx already reads correctly")

	after, err := pgconn.ParseConfig(ensurePoolFloor(dsn, 14))
	require.NoError(t, err)
	require.Equal(t, before.RuntimeParams["options"], after.RuntimeParams["options"],
		"raising pool_max_conns must not re-encode the options parameter")
}

// TestEnsurePoolFloor_RaisesWithoutReEncoding pins the same invariant on the
// branch that REPLACES an existing too-small value, not just the one that
// appends a missing one.
func TestEnsurePoolFloor_RaisesWithoutReEncoding(t *testing.T) {
	t.Parallel()

	const dsn = "postgres://u:p@h:5432/db?options=-c%20search_path%3Dtenant_a&pool_max_conns=2"

	cfg, err := pgxpool.ParseConfig(ensurePoolFloor(dsn, 14))
	require.NoError(t, err)
	require.EqualValues(t, 14, cfg.MaxConns)
	require.Equal(t, "-c search_path=tenant_a", cfg.ConnConfig.RuntimeParams["options"])
}
