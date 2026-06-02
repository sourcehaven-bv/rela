---
id: RR-TKNUJ
type: review-response
title: openBackend + poolCloser + backfill duplicated byte-identically across appbuild and cli/mcp_wiring
finding: 'go-architect (S3): the postgres openBackend (build pool -> Migrate -> NewSearchBackend -> pgstore.New) plus its poolCloser/mcpPoolCloser are duplicated nearly byte-identically across internal/appbuild/appbuild_postgres.go and internal/cli/mcp_wiring_postgres.go. Same true for the FS bleve backfill (appbuild_fs.go backfillBleve vs mcp_wiring_fs.go backfillMCPBackend). A 4th backend multiplies this: 6 recipe files + duplicated closers. Recommendation: push backend-open down into the store package, e.g. pgstore.Open(ctx, dsn) (store.Store, search.Backend, io.Closer, error) owning pool+migrate+wire, so both recipes call one constructor and the pool/closer logic lives once next to pgstore. Same shape for a shared fs-open helper.'
severity: significant
resolution: 'Fixed: added pgstore.Open(ctx, dsn) (store.Store, search.Searcher, io.Closer, error) in internal/store/pgstore/open.go — the single owner of pool-build + Migrate + store/search wiring + pool-closing io.Closer. Both appbuild_postgres.go and cli/mcp_wiring_postgres.go now call pgstore.Open and no longer import pgxpool; the duplicated openBackend bodies and the two near-identical poolCloser types are gone. Bonus: pgx is now confined to the pgstore package (removed the now-dead canUse:pgx allowances from appbuild/cli in .go-arch-lint.yml, tightening the dependency graph). A 4th DB backend now adds one store package with its own Open + two ~6-line recipe shims. The FS bleve-backfill duplication (backfillBleve vs backfillMCPBackend) was left as-is — lower value, and the two roots differ slightly in logging.'
status: addressed
---

## Resolution plan

Defer-or-fix decision needed. Options:
- **Fix now:** add `pgstore.Open(ctx, dsn) (store.Store, search.Backend, io.Closer, error)`
(builds pool, runs Migrate, wires store+search, returns a pool-closing
io.Closer). Both appbuild_postgres.go and mcp_wiring_postgres.go collapse to a
one-line call. Optionally a shared fs-open helper for the bleve backfill
duplication.
- **Defer:** acceptable while there's exactly one DB backend; revisit when a 2nd
(sqlite/bolt) lands. The duplication is ~25 lines × 2, low churn risk.

Leaning fix-now for the pgstore.Open part (small, removes the worst duplication
and the two near-identical poolCloser types); the fs backfill duplication is
lower value.
