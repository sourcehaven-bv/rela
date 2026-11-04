---
id: PLAN-3G8X9Z1
type: planning-checklist
title: 'Planning: Place a channel type on the entity page by configuration'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Approved 2026-11-03 with a scope change: only `placement:
above_relations` ships here; `tab` moved to TKT-B75I7LH and is rejected as
unknown until then. In: `channels.<type>.placement: above_relations` with
validation, configedit allowlist and docs; an `entry` flag on view sections;
placement blocks on the entity page (full page and embedded hosts); a `types`
filter on ChannelsPanel/ChannelsSection; Go, Vitest and e2e tests. Out: `tab`
(TKT-B75I7LH), jump-bar entry (TKT-EKCWGQA), `pages:` entity pages,
per-entity-type overrides, other panels, the PoC config change. Full plan:
`.ignored/plan-TKT-VXBE16H.md`.

**Acceptance Criteria:**
1. `above_relations` puts the block before the first non-entry section.
Go (entry flag), Vitest (insert index for generated, custom, entry-only and
empty views; blocks), e2e (block above the first relation section).
2. No `placement`: page unchanged. Existing tests pass unchanged.
3. Unknown value (`tab` included): load error naming the channel type. Go
test on the error text; allowlist test.
4. A `?channel=` link for a type in another block is ignored by this block.
5. Budget: at most two live keys per block (Vitest).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** ~~RES entity~~ (N/A: four config locations compared in the
plan's Decision section; the choice follows the existing `channels:`
presentation convention, so no separate research entity)

**Existing Solutions:**
- `channels.<type>.session.display` (TKT-56TPOTE) and
`messages.<m>.display` (TKT-03DMXH): per-channel-type presentation in
`data-entry.yaml`, validated in `validateChannelsShape`. Followed.
- `entities.<type>.mentioned_by_panel` (TKT-HF0D5W): per-entity-type panel
switch. Considered, rejected (placement is per channel type).
- `pages:` entity pages (TKT-ITQ0HL, `page.go`): tabbed screens on their own
route. Rejected: does not lay out `/entity/<type>/<id>`.
- `RlViewTabs` (rela-components): reused for the tab bar, with a new
optional badge.
- `useEntityChannels`, `ChannelsSection` deep link (TKT-9YRRM5): reused per
block with a `types` filter.
- No external library applies; this is layout of existing components.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See plan sections Decision, Behaviour and Budget.
Channel types with equal placement share one block (a `ChannelsSection` filtered
by `types`). The `above_relations` block renders in the sections loop before the
first section without `entry: true`, or after the last section. Each block holds
at most two live keys; full page 7, worst stream 13 of 16.

**Files to modify:** Go: `internal/dataentryconfig/channels.go` (+test),
`internal/configedit/allowlist.go` (+test), `internal/dataentry/sections.go`,
`internal/dataentry/views_handler.go`, `internal/apiwire/v1/responses.go`, view
goldens in `internal/dataentry/testdata/golden/`. SPA: `types/config.ts`,
`api/views.ts`, new `channels/channelPlacement.ts`, `ChannelsPanel.vue`,
`ChannelsSection.vue`, `EntityDetail.vue`, `useInboxSummary.ts` (budget note).
e2e: `realtime-project.ts`, `channels.spec.ts`, `channels.page.ts`. Docs:
`docs/data-entry.md`, `docs/realtime.md`, GUIDE mirrors.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `placement` (operator config): allowlist of values, load error otherwise.
The SPA treats an unknown value from a newer server as the default.
- `entry` flag: server-derived from config.

**Security-Sensitive Operations:** None new. Channel content and existence stay
behind the ACL-gated channel list and channel routes; tab visibility and counts
come from the reader's own gated list. The `entry` flag discloses config only,
which is not secret.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See plan "Acceptance criteria and tests".

**Edge Cases:** Several types per block; type without a `channels:` entry
(bottom); custom view starting with a relation section; entry-only and empty
views; hide_empty sections omitted by the server; unreadable channel type (block
renders nothing); duplicate heading ids (`useId()`).

**Negative Tests:** Unknown `placement` value (`tab`, `sidebar`) is a load error
with the channel type and valid values; a `?channel=` for a type outside a block
does not switch that block's filter to `all`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- EntityDetail size: logic in a pure helper; one block in the loop.
- Additive wire field on every view response: `omitempty`, goldens checked.
- Live subscriptions: at most two per block; worst stream 13 of 16.

Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - `channels.<type>.placement`
- [x] docs/realtime.md - pointer to the placement setting
- [x] GUIDE-data-entry / GUIDE-realtime mirrors in docs-project
- [x] ~~docs/metamodel.md~~ (N/A: no schema change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~CLAUDE.md~~ (N/A: no new convention)
- [x] ~~README.md~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 0 critical, 3 significant, 9 minor, 3 nit. Deferred
to TKT-B75I7LH (tab only, approved scope change): RR-SY7QUQQ, RR-RP4N6QL
(significant), RR-YSSDE4P, RR-X4FLZV0, RR-FH331WE, RR-2R9PQQY, RR-OFREEMB.
Addressed: RR-T1CKYTL (significant; non-tab budget), RR-GVZEOTE, RR-NYD6CG9,
RR-AFVFT7M (jump bar split off to TKT-EKCWGQA), RR-PQ1APYX, RR-047L45Q,
RR-22F0A8K, RR-HDBBVV4.
