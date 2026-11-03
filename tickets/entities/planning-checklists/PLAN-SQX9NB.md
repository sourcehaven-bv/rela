---
id: PLAN-SQX9NB
type: planning-checklist
title: 'Planning: Realtime 2/1: advisory lease store with claim, renew, release'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope (1a):** internal/lease value types (Lease, Holder, errors), the Store
interface (Claim, Renew, Release, Get(ids), ListByHolder, Rekey, Drop, Version,
Close), lease/kvlease, lease/pglease with migration 0024_leases.sql,
lease/leasetest.RunAll, internal/filelock (FileLocker moved out of
piles/kvpiles, unix + windows), arch-lint and coverage entries. **Out:** the
service, access checks, wiring, MCP/HTTP, SPA (TKT-EIC19E, TKT-27XVYS,
TKT-MHXH87).

**Acceptance criteria:**
1. leasetest.RunAll passes on kvlease over MemFS, kvlease over a disk dir, and pglease through a schema-pinned DSN: claim; conflict returns *HeldError with the current holder; renew by lease hash + identity; renew by another identity or after expiry is ErrLost; release (idempotent answer); lazy expiry in Get/ListByHolder; Claim prunes expired entries; Rekey; Drop; per-identity and store caps count live leases only; token strictly increasing across claim, release, reopen and a deleted document; Version changes on every write.
2. 100 goroutines claiming one id: exactly one winner (kv and pg). Two kvlease stores with separate FileLockers over one directory: exactly one winner.
3. The store never sees a plaintext lease_id (API takes the sha256 hash); lock file opened O_NOFOLLOW 0600; a corrupt document is reset under the lock with a warning that omits its contents.

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA)
- [x] Searched for existing libraries that solve this problem (none fit: needs state.KV/pg tenant schema and the repo's conformance pattern)
- [x] Checked codebase for similar patterns or reusable code (internal/piles, piles/kvpiles FileLocker, pgpiles, pilestest; schedulerstate)
- [x] Looked for reference implementations in other projects (lease/fencing-token pattern: Chubby, etcd leases, Kleppmann's fencing tokens)
- [x] Reviewed relevant rela concepts for prior art (store-backends; storage.md rule)

## Approach
- [x] Technical approach chosen and documented (.ignored/phase2-plan.md "Ticket 1 REVISED")
- [x] Approach builds on existing patterns (not reinventing) (piles store/kv/pg/test layout and locker selection)
- [x] Alternatives considered (document why rejected) (per-key KV files: KV cannot list, so no pruning; pushing lease changes into changefeed.Bus: a polled change does not know its types; LISTEN/NOTIFY: held connections, storage.md; per-entity token with tombstones: unbounded growth)
- [x] Dependencies identified (packages, APIs, types) (state.KV, storage.MemFS, pgx, entity ids)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (holder label/status and TTL come from callers via the 1b service; ids from the gated reader)
- [x] Input validation approach defined (allowlist preferred over blocklist) (label allowlist, status rune limit and character rejection, TTL bounds; validated in 1b, store asserts invariants)
- [x] Security-sensitive operations identified (file access, auth, crypto) (lock file, state document, lease_id hashing, pg advisory locks)
- [x] Error handling doesn't leak sensitive information (no lease ids or document contents in errors or logs)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (expiry exactly at now, renew after expiry with no competitor, deleted document, corrupt document, caps at the boundary)
- [x] Negative test cases defined (invalid input, error conditions) (wrong identity, unknown hash, expired lease, over-cap)
- [x] Integration test approach defined (not just unit tests) (conformance suite over real disk and real postgres)

## Risk Assessment
- [x] Technical risks assessed with mitigations (file-lock portability: reuse the tested kvpiles FileLocker; pg races: conditional upsert + advisory lock; moving FileLocker could break piles: piles tests run unchanged)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (m)

## Documentation Planning
- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: storage only; user docs land with 1c)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing change in 1a)
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] N/A - Internal change, no user-facing docs needed

## Design Review
- [x] Run `/design-review` before starting implementation (security and architecture reviews; 23 review responses on TKT-6QC5EL, all folded into the revised plan)
