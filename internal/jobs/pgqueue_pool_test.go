//go:build postgres

package jobs

import (
	"net/url"
	"strconv"
	"testing"

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
