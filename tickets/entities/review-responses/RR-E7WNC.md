---
id: RR-E7WNC
type: review-response
title: DSN must be threaded via appbuild Option + openStore seam, not just cmd/ flags
finding: 'The plan said ''build-tag-guarded DSN wiring in cmd/''. But the openStore seam signature is `openStore(fs, paths, meta, obs)` with NO DSN param, and it''s called from FOUR places: appbuild.New (line 421), cli/mcp_wiring.go newMCPServices, and indirectly via appbuild.Discover (used by rela-server main.go:101 and cli/kong.go runKong). cmd/rela is a thin wrapper over kong; cmd/rela CANNOT add a flag without going through internal/cli. So a per-cmd flag alone is insufficient — the DSN must reach openStore. The appbuild.Option type already exists (currently only WithACL) and is the clean threading path.'
severity: significant
status: open
---

## Resolution (plan update)

Thread the DSN through the existing `Option` mechanism, build-tag-aware:
- Add `appbuild.WithDatabaseURL(dsn string) Option` (stored in `options`). In the **postgres** build, `appbuild.New` passes it into `openStore` (the postgres `openStore` reads it; the FS `openStore` ignores it). Since `openStore`'s signature is fixed across builds, either (a) read the DSN from `options` inside `New` and add a postgres-only overload path, or (b) resolve the DSN from env inside the postgres `openStore` directly and treat the flag/Option as an override passed via a package-level/contextual value. Prefer extending the seam: add the dsn to a small `openStoreParams` struct so all three variants share one signature.
- Resolution order: `--database-url` flag > `RELA_DATABASE_URL` env. The CLI flag lives in `internal/cli/kong.go` global flags (so `cmd/rela` gets it for free); `rela-server` adds its own `-database-url`; MCP (`newMCPServices`) and desktop read `RELA_DATABASE_URL` env (no flag surface).
- **Redact** the password from any logged/echoed DSN.
- Decide and document: in the postgres build, is `--project` (filesystem) still required? See RR for metamodel-on-disk.
