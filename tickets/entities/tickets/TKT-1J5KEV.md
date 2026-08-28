---
id: TKT-1J5KEV
type: ticket
title: 'appbuild: make the database DSN an explicit parameter instead of an ambient env read'
kind: refactor
priority: medium
effort: s
status: backlog
---

## Description

`appbuild.Discover` reads `os.Getenv("RELA_DATABASE_URL")` directly
(`internal/appbuild/appbuild.go:701`). Make the DSN an explicit input so that
*where a project's data lives* is decided by the caller, not by process
environment.

`Config.DatabaseURL` is **already** a per-`Config` field
(`appbuild.go:648-654`), and `appbuild.New` is already fine — it takes the DSN
from the caller. **Only `Discover` is ambient.** This is a small, contained
change.

### Why (stands on its own)

An ambient env read is the classic testability problem: a test that wants to
point `Discover` at a specific database has to mutate process environment, which
is global and order-dependent under `t.Parallel()`. It is also
surprise-in-a-second-context — the value silently changes meaning when a second
project is constructed in one process, which `rela-desktop` already does on
every project switch (`cmd/rela-desktop/main.go:125-205`).

### Why it also matters later

Multi-tenant SaaS (RES-D54281) resolves every tenant to a DSN — `search_path` on
one cluster today, a different host once tenants shard across clusters. That
whole range stays reachable **only** while a single lookup decides where data
lives. The ambient read is the one thing keeping tenant count an architecture
question instead of an ops one. RES-S8CH9C named this "the seam that preserves
every storage option at zero cost now"; the sibling refactor R1 (schema-scoped
advisory locks) shipped on the same principle and paid for itself as a plain bug
fix.

**This ticket does not build multi-tenancy.** It removes one ambient read.

## Scope

**In scope**

- Add an option (e.g. `appbuild.WithDatabaseURL(dsn string)`) so callers pass the
DSN explicitly. Follow the existing `Option` pattern used by `WithACL`
(`appbuild.go:518-522`).
- Keep `Discover`'s env read as the **default** when no option is supplied, so
every current caller keeps working unchanged.
- Prefer the explicit option over the env var when both are present, and make
that precedence a test.
- Update the `Discover` and `Config.DatabaseURL` doc comments — they currently
assert env-only sourcing as a security property (see the constraint below).

**Out of scope**

- Any tenant resolution, `org_id` mapping, or per-tenant store handles.
- Splitting `appbuild.Services` into shared-config + per-tenant-store (the main
refactor in RES-D54281; this ticket is a prerequisite, not that work).
- Changing `pgstore` — it already takes a DSN and nothing else
(`pgstore.Open(ctx, dsn)`), which is why this seam works.

## Call sites

Six non-test callers of `appbuild.Discover`, all shallow — each simply gets the
option threaded through or keeps the env default:

| Site | Note |
|---|---|
| `cmd/rela-server/main.go:149` | via `discoverProject`, already passes `discoverOptions(f)...` |
| `cmd/rela-docs/main.go:69` | |
| `internal/docscapture/server.go:81` | |
| `internal/cli/mcp_wiring.go:43` | already passes `WithACL(acl.NopACL{})` |
| `internal/cli/kong.go:170` | |
| `internal/cli/flow.go:43` | |
| `internal/cli/validate.go:83` | |

(`rela-desktop` calls `appbuild.New` directly with an explicit `Config`, so it
is already correct and needs no change.)

## Load-bearing constraint: keep the credential out of argv

**Do not add a `--database-url` flag.** There is deliberately none today
(`cmd/rela-server/main.go:120-122`, and the `Config.DatabaseURL` godoc at
`appbuild.go:648-653`): a DSN carries a password, and a flag would put it in
`ps` output and shell history. The rule is *not a flag*, not *env-only* — an
option passed in Go code by a caller that got the DSN from env, a config file,
or a tenant lookup is fine and is the point of this change.

The godoc currently phrases the guarantee as "sourced from the RELA_DATABASE_URL
environment variable... deliberately env-only". Reword to state the actual
invariant (never on a command line) so the next reader does not think the env
read itself is the security property and re-inline it.

## Acceptance criteria

1. `appbuild.Discover` no longer reads `os.Getenv` when a DSN option is supplied.
2. With no option supplied, `Discover` behaves exactly as today (env read),
and every existing call site compiles and behaves unchanged.
3. An explicit DSN option takes precedence over a set `RELA_DATABASE_URL`, pinned
by a test.
4. A test constructs two `Services` in one process against two different DSNs
without touching process environment. This is the property the whole ticket
exists for.
5. No `--database-url` flag is added to any binary; `rela-server --help` is
unchanged.
6. Godoc on `Discover` and `Config.DatabaseURL` states the invariant as
"never on a command line" rather than "env-only".
7. `just arch-lint` and `just ci` pass.

## Do not consolidate the call sites

Considered and **rejected** (2026-08-13). The four `internal/cli` sites look like
duplication but differ in ways a shared helper would have to re-expose:

- `kong.go:170` is conditional on `requiresProject(ktx.Command())`, owns
  `defer svc.Close()`, and its error path prints `relaerrors.WrapDiscoverError`
  and returns an exit code.
- `flow.go:43` starts from a directory derived from a **script path**, not the
  project flag, and deliberately replaces the error with
  `"no project found for script %s"`.
- `validate.go:83` runs *after* an early return on `result.MetamodelError`, so it
  cannot be hoisted to a common point.
- `mcp_wiring.go:43` passes `WithACL(acl.NopACL{})` behind a documented
  trust-boundary justification.

A helper covering all four would take a start dir, an options slice, and an
error-wrapping strategy — i.e. it would be `appbuild.Discover` with a layer on
top, and it would bury the `NopACL` opt-out that is currently explicit. It would
also change control flow and user-facing error strings in four commands, turning
a mechanically behaviour-preserving refactor into one needing regression review.

**Do** clean up while touching these lines: `flow.go:43` and `validate.go:83`
carry an identical `//nolint:contextcheck` comment, and `kong.go:167-169` has a
comment asserting the env-only rule that this ticket rewords (see the argv
constraint above). Comment fixes only — no structural change.

## Notes

- The postgres recipe already validates a non-empty DSN in its own `openBackend`
(`appbuild_postgres.go:48-51`); `Config.validate()` deliberately only nil-checks
the four build-agnostic fields (`appbuild.go:660-674`). Keep that split — do not
move DSN validation into `validate()`, since FS/memory builds ignore the field.
- `MigrateDSN` / `StatusDSN` (`pgstore/open.go:80-98`) and the `db` commands read
the env directly and are **not** part of this ticket.
