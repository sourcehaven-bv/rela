---
id: RR-P13ZK
type: review-response
title: pgstore.New accepts an injected pgx handle (DBTX), not a DSN
finding: Original plan had pgstore own its connection (constructed from a DSN inside the store). That couples connection lifecycle/config into the store, makes per-test schema isolation awkward, and violates the project's 'constructors take focused dependencies' rule.
severity: significant
resolution: 'DECIDED: pgstore.New(db DBTX, opts...) accepts an injected pgx handle. DBTX is a small interface (Exec/Query/QueryRow/Begin) that *pgxpool.Pool, pgx.Conn, and pgx.Tx all satisfy. CRITICAL CONSTRAINT: prod AND tests inject a POOL (not a bare pgx.Tx). The store must Begin() its own transactions (rename, cascade-delete) and serve concurrent ops (FuzzConcurrentOps + -race); a bare Tx is single-connection and would serialize everything and force nested savepoints. A pool satisfies DBTX and Begin() returns a real tx, so rename/cascade and concurrency both work. appbuild (postgres build) builds the *pgxpool.Pool from the resolved DSN and passes it to pgstore.New; the store never sees the DSN. The conformance factory passes a pool whose BeforeAcquire/AfterConnect sets `SET search_path TO <test-schema>` (see RR-U9RFH) so isolation is transparent to the store. Migration runs against the same DBTX (per-schema in tests, once in prod).'
status: addressed
---

## Design decision

```go
// pgstore: store accepts a handle, owns no connection config.
type DBTX interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
    Begin(ctx context.Context) (pgx.Tx, error)
}

func New(db DBTX, opts ...Option) (*Store, error) // rejects nil db (constructor-validates required deps)
```

- **Prod:** `appbuild` (postgres build) builds `*pgxpool.Pool` from the DSN
(`RELA_DATABASE_URL` / `--database-url`), runs migrations, calls
`pgstore.New(pool)`.
- **Tests:** shared pool + per-schema `search_path` scoping (RR-U9RFH); inject the
scoped pool.
- **Lifecycle:** the OWNER of the pool closes it. In prod that's the
`buildSearcher`/`openStore` seam returning an `io.Closer` that closes the pool
(the store's `Close()` closes subscriber channels + signals done, but does NOT
own the pool unless we decide it should — pick one and document; leaning:
appbuild owns and closes the pool, store.Close() just tears down the watcher).
- Same handle feeds the pg search backend so it queries the same DB/schema.
