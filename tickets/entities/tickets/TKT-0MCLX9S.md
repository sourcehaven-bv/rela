---
id: TKT-0MCLX9S
type: ticket
title: 'rela test v1: .test.lua runner with fixtures, principals, fake clock and mail capture'
kind: enhancement
priority: medium
effort: xl
started: "2026-11-07"
status: ready
---

## Description

Operators have no way to check that their app config (schema, `acl.yaml`,
automations, scripts) behaves as intended, or that it still does on a newer rela
build. Static checks exist (`rela validate`, `analyze`, `acl map --expect`);
behaviour checks do not.

This ticket adds the first slice of `rela test` (FEAT-IODR9ST): a runner for
`.test.lua` files in the project's `tests/` folder.

- **Isolation:** each case runs on a fresh in-memory world: memstore, LinearSearch and a MemFS for every write. Config (schema, ACL, scripts) is read from disk through a read-only rooted loader. The command does not open the live store, locks or database.
- **Fixtures:** `t.fixtures("fixtures/basic")` loads entity and relation markdown into the case's memstore with `fsimport` Copy, as given state (no ACL, automations or validation).
- **Principals:** `t.as(user)` acts through the real write path, so automations, validation and ACL all run. Roles come from the real policy. `t.principal{roles = {...}}` creates a user and a per-case policy clone that assigns those roles.
- **Writes and reads:** create, update, link, unlink, transition, get, list.
- **Assertions:** `t.expect(...)`, `t.expect_denied(...)`, `t.expect_mail{...}`, `t.expect_access(principal, {...})`, reusing the `acl map --expect` format.
- **Determinism:**
  - A shared fake clock (`t.clock`, `t.advance`) reaches automations, predicates, computed properties, Lua, memstore timestamps and leases. Audit and version stamps stay on the wall clock.
  - `background: true` automation actions run in the foreground (the existing default), so their effects are visible in the next step.
  - Mail goes to a shared memory transport; `mail.yaml`'s recipient policy still applies.
  - Real HTTP and AI calls fail the test.
  - `t.random` is seeded.
- **Generated data by default:** the easy path in the API is the faker path.
  - `create("incident")` fills every required property from the schema (enum values, dates around the fake clock, names, titles, emails). A test names only the properties it is about.
  - `t.fake` offers schema-aware and plain fakers (`t.fake.value("incident", "priority")`, `t.fake.email()`, `t.fake.date{within = "30d"}`).
  - `t.principal{roles = {...}}` creates a principal with a generated name.
  - The clock starts at a seeded random instant; tests use `t.now()` and durations, not literal timestamps.
  - Every run uses a fresh seed, printed in the summary and on failure; `--seed N` replays it exactly. Each case's seed is derived from the run seed and the case name, so adding a case does not change the others' data.
- **Output:** a TAP-like summary and a non-zero exit code on failure, so it works in CI.

Out of scope here (separate tickets): letting the operator choose the test
backend (sqlite/postgres, needs per-case isolation and parallelism design),
scheduler ticks, HTTP/AI stubs, execution trace and coverage, mutation testing,
property-based tests, record mode, version differential.
