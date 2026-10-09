//go:build postgres

package appbuild_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

// TestTokens_PostgresOneRefresherAcrossPools pins TKT-01KZSO on postgres:
// two rela processes serving one schema are two Services with their own
// connection pools. When both need an access token at once, exactly one of
// them refreshes; the other waits on the shared lock and reads the result.
// A second refresh would spend a rotated refresh token and lock everyone out.
//
// backendtest pins the schema in the DSN, so this also proves the sealed
// store and the lock both live in the tenant's schema, not in public.
func TestTokens_PostgresOneRefresherAcrossPools(t *testing.T) {
	var refreshes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("refresh_token") != "R0" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		n := refreshes.Add(1)
		// Slow enough that the other process is waiting on the lock.
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"A%d","refresh_token":"R%d","expires_in":3600}`, n, n)
	}))
	t.Cleanup(srv.Close)

	root := writeMinimalProject(t)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".rela", "secrets.yaml"),
		fmt.Appendf(nil, "token_key: %s\ncid: id\ncsecret: secret\n", key), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "connections.yaml"), fmt.Appendf(nil,
		"connections:\n  svc:\n    token_url: %s/token\n    client_id_secret: cid\n"+
			"    client_secret_secret: csecret\n    style: rfc6749\n    user_agent: test (ops@example.com)\n", srv.URL), 0o600))

	open := discoverer(t)
	brokers := make([]*tokenstore.Broker, 2)
	for i := range brokers {
		svc, err := open(root)
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })
		brokers[i], err = appbuild.Tokens(svc)
		require.NoError(t, err)
	}
	ctx := context.Background()
	require.NoError(t, brokers[0].Set(ctx, "svc", tokenstore.Token{Refresh: "R0"}))

	got := make([]string, len(brokers))
	var wg sync.WaitGroup
	for i, b := range brokers {
		wg.Go(func() {
			tok, err := b.AccessToken(ctx, "svc")
			assert.NoError(t, err)
			got[i] = tok
		})
	}
	wg.Wait()

	require.Equal(t, int32(1), refreshes.Load(), "both processes refreshed")
	require.Equal(t, "A1", got[0])
	require.Equal(t, got[0], got[1], "the waiting process did not read the refreshed token")
	st, err := brokers[1].Status(ctx, "svc")
	require.NoError(t, err)
	require.True(t, st.AccessFresh)
}
