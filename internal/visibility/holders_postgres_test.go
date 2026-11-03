//go:build postgres

package visibility

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// The postgres twin of TestHolderLookup_GatedLikeList: the holder predicate
// on a list value, the face allowlist and the paging render as postgres
// SQL. It runs in its own schema pinned through the DSN.
func TestHolderLookup_GatedLikeList_Postgres(t *testing.T) {
	base := os.Getenv("RELA_TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("RELA_TEST_DATABASE_REQUIRED") != "" {
			t.Fatal("RELA_TEST_DATABASE_URL not set but RELA_TEST_DATABASE_REQUIRED is")
		}
		t.Skip("RELA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("relaholders_%d", os.Getpid())
	cfg, err := pgx.ParseConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	kv := []string{
		"host=" + cfg.Host, fmt.Sprintf("port=%d", cfg.Port), "user=" + cfg.User, "dbname=" + cfg.Database,
		"search_path=" + schema + ",public",
	}
	if cfg.Password != "" {
		kv = append(kv, "password="+cfg.Password)
	}
	if cfg.TLSConfig == nil {
		kv = append(kv, "sslmode=disable")
	}
	dsn := strings.Join(kv, " ")
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`)
		admin.Close()
	})
	if err = pgstore.Migrate(ctx, admin); err != nil {
		t.Fatal(err)
	}
	pool, closer, err := pgstore.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := pgstore.Open(ctx, pool, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st.Close()
		_ = closer.Close()
	})
	assertHoldersLikeList(t, st)
}
