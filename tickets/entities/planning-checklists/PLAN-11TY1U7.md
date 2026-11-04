---
id: PLAN-11TY1U7
type: planning-checklist
title: 'Planning: Show each session decision button once'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Plan file: `.ignored/plan-TKT-A0RTC52.md` (full plan; this checklist summarises
it).

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** A waiting session shows each decision up to three times on the
entity page: in the session header (`OpenChannel.vue` decisions slot), on the
waiting message (`RlChannel` row of `decisionSeq`), and as a type option in the
composer picker. The placed block (TKT-VXBE16H) and the inbox drawer mount the
same `OpenChannel`. The inbox row (TKT-V05706X) already shows each decision
once.

**Rule.** Decision buttons sit on the waiting message when the surface shows it,
else in the fallback place (entity page: session header; inbox row: below the
row). Never both. Choices stay on their question. The composer picker leaves out
every type a shown button stands for, unless that would empty it. A form
decision opens the composer in decision mode (the existing compose mode, owned
by the app). With the decisions on the message, the header shows a **Go to
decision** button that focuses them.

**Scope.** In: `channelView.ts` (`placeDecisions`, `buttonTypesOf`,
`rowDecisions` keeps answer-type presets), `useSessionDecisions.ts`,
`OpenChannel.vue`, `InboxWaitingItem.vue` (shared placement, no visible change),
library `RlChannelComposer`, `RlChannel`, `RlSegmentedControl`, texts en/nl,
tests, docs. Out: server, wire, MCP, CLI; configuration key; a note on a
one-click decision (backlog TKT-HG4PO94); POC config; channels without a
session.

**Acceptance Criteria:** (details and tests in the plan file)
1. Once on the message: each decision once, header shows Go to decision, no picker option for a button type.
2. Header fallback when the waiting message is not shown; no jump.
3. Choices: answer type button nowhere, its presets on the row; Other governs whether the answer type leaves the picker.
4. Composer decision mode owned by the app; Send carries the click generation, also after a refusal; cancel ends a reply it started.
5. No unreachable state: empty-picker safeguard; button types held across the actions gap.
6. Inbox row unchanged, placed by the same function.
7. Keyboard and screen reader: jump button, tabbable unchecked picker, scoped focus follow.
8. Compact header shows the same slot content.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: a placement rule over existing parts; alternatives documented in the plan)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** No library applies. Reused: `rowDecisions`,
`RlChannelDecisions`, `RlChannel.reveal`/`focusRow`, compose mode and
`composeOf`, the composer picker focus-restore pattern, the inbox row placement.
Prior art: TKT-T9G1AE (header and message, no recorded reason for both),
TKT-V05706X (message on the inbox row).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Message wins with header fallback (chosen over
header-only, scroll-based placement and a config key). Composer: leave out
button types with an empty-picker safeguard; decision mode reuses compose mode.
Details in plan sections 1 to 8.

**Files to modify:** listed in the plan ("Files to modify"), frontend only, plus
`e2e/tests/channels.spec.ts`, `e2e/tests/kanban-busy.spec.ts`,
`docs/realtime.md`, `docs/data-entry.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** server-computed `_actions` and session, held
messages, the channel descriptor. No new input, request or wire field.

**Security-Sensitive Operations:** none new. Hiding controls is not access
control; the server gates every post. A form opened by a refusal now carries
`expect_generation`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** per AC in the plan's test table: `channelView.test.ts`,
`useSessionDecisions.test.ts`, `ChannelsPanel.test.ts`,
`InboxWaitingItem.test.ts`, library story plays (composer, segmented control,
channel, session), e2e in `channels.spec.ts` and `kanban-busy.spec.ts`.

**Edge Cases:** hidden cause type, live order and the actions gap, held cap
dropping the cause, paged back (`hasNewer`), presets of one type, a chat type
that moves the session, every type a button type, two channel sections on a
page, ended/stranded/closed sessions.

**Negative Tests:** `select()` of a type not offered, or of a button type
without `compose`, returns false; unknown names in `buttonTypes` change nothing.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** loss of the optional fields of one-click types (documented, open
choice 4, backlog TKT-HG4PO94); focus loss (focus-follow rule); picker flicker
(held button types); existing tests assume header buttons (adapted, not
deleted). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes (inbox section)
- [x] `docs/realtime.md` - decision, preset, choice and composer sections
- [x] ~~docs/metamodel.md, docs/cli-reference.md, CLAUDE.md, README.md~~ (N/A: no schema, CLI or convention change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-PL9YTZ9, RR-0CO6A2S, RR-D83PLN6, RR-890AEID,
RR-1XRWH05, RR-EU11IAA, RR-R6T79M9, RR-WVMB9NA (significant); RR-H6UMX4Q,
RR-HH8KZH5, RR-MPJ038Z, RR-KYPQVZD (minor); RR-IFP2WV3, RR-DFENQCU (nit). All
`addressed`.
