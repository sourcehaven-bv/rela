---
id: PLAN-IVEAY3
type: planning-checklist
title: 'Planning: Realtime 3/1: realtime.yaml loader, validation and hot reload'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope:** internal/realtimecfg: strict parse (unknown keys fail), version 1,
channel_types (attach_to, cardinality, anchor family|face, read, roles,
message_types with properties/persist/ttl/rate/when/post/promote_to/actions,
lifecycle, message_ops, on_entity_delete), triggers, limits (lower only).
Cross-file validation against metamodel, acl permissions and predicate compile.
Load through config.Loader (sqlite projectfiles), hot reload via
config.Subscriber with keep-previous on error and a config-error frame,
published via atomic.Pointer. Configure locked. rela acl audit counts
realtime.yaml permissions. Absent file means realtime off. **Out:** any runtime
use of the config (T2 to T7), session: (refused).

**Acceptance criteria:**
1. Golden valid fixture compiles; each invalid fixture fails naming file, key path and reason.
2. Unknown keys and a session: key are refused (session: says not supported yet).
3. Bad references fail: unknown attach_to type, promote_to type, relation type, enum type, ref target, from: entity.<prop>.
4. A trigger whose when: or props depend on a property with a visible: grant fails to load.
5. A broken reload keeps the previous compiled config and emits a config-error frame; a fixed reload replaces it.
6. A schema.yaml or acl.yaml reload re-validates realtime.yaml.
7. Configure Apply refuses realtime.yaml.
8. rela acl audit does not report realtime.yaml permissions as dead.
9. arch-lint forbids metamodel importing realtimecfg.
10. limits may only lower built-in caps; a higher value fails.

**Design review changes:** see .ignored/phase3-plan.md section "Design review
folded in"; acceptance criteria there extend the list above (cross-process lock,
erasure, tombstones, store API, allowlisted trigger programs, ceiling on
identity roles, reload semantics).

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-1UCUBA and DEC-34YERL)
- [x] Searched for existing libraries that solve this problem (yaml.v3 KnownFields, existing predicate compiler)
- [x] Checked codebase for similar patterns or reusable code (data-entry.yaml loader, schedules.yaml, config.Subscriber in App.StartWatching, TKT-IMBOK config-error frame)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: covered by RES-1UCUBA prior art)
- [x] Reviewed relevant rela concepts for prior art

## Approach
- [x] Technical approach chosen and documented (.ignored/phase3-plan.md)
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected) (realtime: in schema.yaml and split by nature, rejected in DEC-34YERL)
- [x] Dependencies identified (packages, APIs, types)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (operator-authored realtime.yaml; config is not secret, so errors may name keys)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

## Risk Assessment
- [x] Technical risks assessed with mitigations (predicate dependency analysis for the visible: rule may drift; pin with tests per predicate form)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (m)

## Documentation Planning
- [x] User-facing docs identified (skip if internal refactor) (docs/realtime.md reference (docs-project guide))
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: realtime.yaml is separate from schema.yaml)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI in this ticket)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: no UI)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: has docs)

## Design Review
- [x] Run `/design-review` before starting implementation (see review responses on the ticket)
