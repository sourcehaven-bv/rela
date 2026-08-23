---
id: TKT-L1A3PH
type: ticket
title: 'Wire the SQLite backend: sqlite build tag, appbuild recipe, release and CI isolation'
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

Make the SQLite backend selectable. TKT-G91TBK built and proved
`internal/store/sqlitestore`; nothing links it yet. This is the wiring half,
split out during implementation so the store could be reviewed as "is the
backend correct" separately from "is the wiring right".

Depends on TKT-G91TBK (the store) and TKT-415WA7 (capability discovery by
interface, needed only for the stage-4 capabilities).

## Scope

IN:

- A `sqlite` build tag mirroring the 17-file `postgres` pattern:
`appbuild_sqlite.go` recipe, plus `sqlite`-tagged variants of the
`derivedschema_*`/`userstate_*`/`versionsweep_*` no-op files where the SQLite
answer differs from the `!postgres` default.
- **`internal/cli/db_nonpostgres.go` is tagged `!postgres`**, so a sqlite build
currently inherits *"the 'db' command requires the PostgreSQL build … this
binary uses the filesystem backend"*. That is wrong for a backend with real
schema management, and the message names the wrong backend. Retag to `!postgres
&& !sqlite` and add `cli/db_sqlite.go`.
- GoReleaser entries: `rela-sqlite` / `rela-server-sqlite`, `CGO_ENABLED=0`
like every other binary (modernc is pure Go — verified cross-compiling to all
six targets in TKT-TWIO11).
- CI backend-isolation assertions in `ci.yml`, alongside the existing
pgx/bleve ones: the **default and postgres builds must not link
`modernc.org/sqlite`** (already true, needs pinning), and the sqlite build must
not link pgx.
- `justfile` `build-check-tags` gains `-tags sqlite`.
- Docs: `docs/sqlite-backend.md` paralleling `postgres-backend.md`, a row in
CLAUDE.md's storage-backend table, README backend guidance.

OUT: making SQLite the `rela-desktop` default — a separate call once the backend
has been exercised. Also out: versioning, `StateKV`, `UserState`, FTS5 search,
SQL pushdown (all stage 4+).

## The `!postgres` no-ops need deciding, not inheriting

RES-03TUXO found three no-op resolvers whose `!postgres` variants encode a
**single-process** assumption. The SQLite backend enforces single-process at
`Open`, so inheriting them is defensible — but it must be a decision, recorded:

| Resolver | `!postgres` behavior | Correct for sqlite? |
|---|---|---|
| `reconcileDerivedSchemaIfSupported` | no-op; `unique:` falls back to the application scan | Yes — the sidecar lock makes the single-writer assumption true |
| `storeUserStateFor` | nil → KV fallback | Yes, same reason |
| `stateKVFor` | nil → node-local FSKV | Yes, same reason |

Each should get a comment saying *why* it is correct here rather than silently
inheriting a default written for fsstore.

## Acceptance criteria

1. `go build -tags sqlite ./...` succeeds; `just build-check-tags` covers it.
2. `rela-sqlite` and `rela-server-sqlite` build at `CGO_ENABLED=0` for all six
release targets.
3. `rela db` under the sqlite tag does something correct — not the PostgreSQL
error message.
4. CI asserts: default and postgres builds do not link `modernc.org/sqlite`;
the sqlite build does not link pgx.
5. The appbuild wiring tests pass under `-tags sqlite`.
6. Each inherited `!postgres` no-op carries a comment justifying it for this
backend.
7. `just arch-lint`, `just lint`, `just coverage-check` pass.
