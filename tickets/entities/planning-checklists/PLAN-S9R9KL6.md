---
id: PLAN-S9R9KL6
type: planning-checklist
title: 'Planning: AtomicWrites capability flag and capabilities.atomic_batch on _schema'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: decide once at wiring whether the assembled store rolls a transaction
back, pass it to the data-entry app, and report it as
`capabilities.atomic_batch` on `GET /api/v1/_schema`.

Out of scope: any runtime probing, a config key (the owner decided it is not
configurable), MCP.

**Acceptance Criteria:**

1. `appbuild.AtomicWrites(svc)` is true for a sqlitestore or pgstore and false
for fsstore and memstore, judged from the innermost store, not the build tag.
2. A sqlite build that falls back to markdown files (fsstore) reports false.
3. `GET /api/v1/_schema` has `capabilities.atomic_batch` matching the flag;
the field is always present.
4. rela-server and rela-desktop both get the flag through
`dataentrywire.Services`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-QKREXZZ, DEC-ODU65KR item 1.

**Existing Solutions:**

- `internal/appbuild/realtime_promote_{sqlite,postgres,nodb}.go` already
decides a per-backend capability from `store.Innermost(st)` and
`pgstore.HandleFor(st)` under build tags. The new files mirror them.
- No library needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `internal/appbuild/atomic_writes_{sqlite,postgres,nodb}.go` with a
package-private `atomicWritesFor(store.Store) bool` per build tag, and
`AtomicWrites(svc *Services) bool` in a shared file. A package function, because
`Services` is at its method cap.
- `dataentry.SetAtomicBatch(a *App, on bool)`, a package function for the
same reason, stores the flag on the app before it serves.
- `dataentrywire.Services` calls it.
- `v1.Schema` gets `Capabilities v1.Capabilities` with `AtomicBatch bool
json:"atomic_batch"`, filled in `handleV1Schema`. The OpenAPI schema for
`_schema` gets the property.

Alternatives rejected: a method on `store.Store` (widens every backend for one
bit); a runtime check per request (the answer cannot change after wiring);
exposing the build tag (wrong for the sqlite fs fallback).

**Files to modify:**

- internal/appbuild/atomic_writes*.go (new)
- internal/dataentry/app.go or a small new file for `SetAtomicBatch`
- internal/dataentry/api_v1.go (`handleV1Schema`)
- internal/apiwire/v1 schema types
- internal/dataentrywire/wire.go
- internal/openapi (schema property)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** none; the flag comes from the assembled store.

**Security-Sensitive Operations:** the flag discloses the storage tier. That is
configuration, not data; per CLAUDE.md "the configuration is not a secret", and
`_schema` is already served to every authenticated caller. No DSN or path is
exposed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC 1/2: `atomicWritesFor` table test per build (fsstore and memstore false
in the default build; sqlitestore true under `-tags sqlite`; pg true under
`-tags postgres`).
2. AC 3: dataentry test, `GET /api/v1/_schema` with the flag off and on.
3. AC 4: covered by compile and the wiring call; the endpoint's 501 test
(TKT-BO8CYG9) exercises the flag end to end.

**Edge Cases:** a wrapped store (decorators) resolves through `store.Innermost`.

**Negative Tests:** fs and memory report false.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a future backend forgets the flag and reports false; that fails safe
(501). Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** the `_schema` capability is described in the
api-reference section written for TKT-BO8CYG9.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-2FB5KS6 (minor): the flag is computed from the
store with store.TxRollsBack instead of a setter. Addressed.
