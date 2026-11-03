---
id: PLAN-1NVG2X
type: planning-checklist
title: 'Planning: Realtime 3/2: channel store and backends'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope:** internal/channels Store interface, channelstest.RunAll
conformance, filechannels (.rela/channels/<anchor>/<type>[~name]/ meta.yaml,
log-NNNN.jsonl segments of 1000, audit.jsonl; edits and removals as patch
records folded on read), sqlitechannels and pgchannels (channels,
channel_messages, channel_audit tables in tenant schema; migrations), appbuild
selection per build tag, re-key on rename and on_entity_delete fan-out hooks
(delete, detach). Anchor key is family id or id plus face per channel type
(decision 3). **Out:** authorization, service logic, HTTP (T3).

**Acceptance criteria:**
1. All backends pass RunAll.
2. seq is gapless and strictly increasing per channel under 50 concurrent appends.
3. Pages by after/before/limit return ordered messages; limit is capped at 200.
4. Edit and remove patches fold on read; a removed message keeps its seq with a removal record.
5. Re-key moves all channels of an anchor atomically; reads under the old key find nothing.
6. Delete removes messages and writes a deletion record to channel audit.
7. Postgres tests use a schema-pinned DSN; sqlite single-process lock respected.
8. arch-lint: channels does not import internal/store.
9. Message JSON over 16 KiB or more than 32 properties is rejected at the store boundary.

**Design review changes:** see .ignored/phase3-plan.md section "Design review
folded in"; acceptance criteria there extend the list above (cross-process lock,
erasure, tombstones, store API, allowlisted trigger programs, ceiling on
identity roles, reload semantics).

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-1UCUBA and DEC-34YERL)
- [x] Searched for existing libraries that solve this problem (none fit: comments file backend caps threads at 500 in one document)
- [x] Checked codebase for similar patterns or reusable code (internal/comments Store and conformance, piles kvpiles/pgpiles, lease kvlease/pglease)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: covered by RES-1UCUBA prior art)
- [x] Reviewed relevant rela concepts for prior art

## Approach
- [x] Technical approach chosen and documented (.ignored/phase3-plan.md)
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected) (reuse comments: rejected for its single-document thread cap; state.KV blobs: rejected, no ordered paging)
- [x] Dependencies identified (packages, APIs, types)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (message payloads are untrusted; anchor keys derive from entity ids and type names; path segments must be validated against traversal)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

## Risk Assessment
- [x] Technical risks assessed with mitigations (fs segment rollover and crash safety; mitigate with append-then-fsync and a rebuild-on-open test)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (l)

## Documentation Planning
- [x] User-facing docs identified (skip if internal refactor) (docs: storage note in the realtime guide (shared with T1))
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: realtime.yaml is separate from schema.yaml)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI in this ticket)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: no UI)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: has docs)

## Design Review
- [x] Run `/design-review` before starting implementation (see review responses on the ticket)
