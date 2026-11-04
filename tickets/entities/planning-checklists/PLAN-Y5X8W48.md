---
id: PLAN-Y5X8W48
type: planning-checklist
title: 'Planning: Group quiet channel messages across session notices'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Plan file: `.ignored/plan-TKT-LKIVLR2.md` (full plan; this checklist summarises
it).

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `foldMessages` (`rela-components/.../channel/channelFold.ts`) ends
a run of quiet messages at every session notice, so an agent run that alternates
progress and state moves shows many small fold lines (FINDINGS.md gap 6).

**Rule.** A quiet notice (target state status `working`) between two members of
a run joins the run, the same way a removed message does today (`pending`). A
quiet notice with no open run, a notice into a waiting, final or unknown state,
and a kept row end the run and stay rows. A run without a collapsed message
renders as today. In a `working` state with a quiet cause, the decisions move to
the session header so the cause stays folded.

**Scope.** In: `foldMessages`, fold count and text, `noticeView.toStatus`,
announce of folded notices, focus hand-off when a row folds away, decision
placement for a working state with a quiet cause, strings (en, nl), stories,
tests, `docs/data-entry.md`. Out: hidden-only runs, channel types without quiet
types, any server/API/MCP/CLI change, a config key, the inbox row.

**Acceptance Criteria:** AC 1 to 10 with 4a and 7a in the plan file, each mapped
to a test in the Test Plan section.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small frontend change inside an existing fold function)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: extends rela's own fold; no library involved)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: behaviour is specific to rela sessions)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Reuses `foldMessages` and its `pending` mechanism for
removed messages (`channelFold.ts`), `RlChannelFold`, `rowFor`/`shownAs`,
`channelAnchor.findAnchor`, `placeDecisions` fallback (TKT-A0RTC52). Unread rule
`channels.Counts` already excludes notices. Prior tickets TKT-03DMXH,
TKT-A0RTC52, TKT-V05706X, TKT-VXBE16H.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See plan "Rule" and "Approach (files)". The library gets
`ChannelNoticeView.toStatus`; `foldMessages` treats quiet notices like pending
removed messages; `foldRow` returns `count` and `moves`; `RlChannel` announces
notices also when folded and hands focus to the fold button;
`useSessionDecisions` leaves `decisionSeq` undefined for a working state with a
quiet cause.

**Files to modify:** `rela-components/src/components/channel/{types.ts,
channelFold.ts, RlChannelFold.vue, RlChannel.vue, *.stories.ts}`,
`rela-components/src/composables/{useMessages.ts, messages.nl.ts}`,
`src/components/entity/channels/{channelView.ts, useSessionDecisions.ts,
OpenChannel.vue}`, tests (`channelFold.test.ts`, `channelView.test.ts`,
`useSessionDecisions.test.ts`, `ChannelsPanel.test.ts`), `docs/data-entry.md`.

**Alternatives rejected:** fold every notice (hides waits), judge by head (rows
re-fold on every move), join notices at run edges (hides the result of the
reader's own decision), server-side flag (presentation), second grouping pass
(ticket forbids), config key (not needed for the default).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The session descriptor (config, not secret) and
messages the reader may already read. Only `toStatus === 'working'` makes a
notice quiet; any other or missing value shows it (allowlist).

**Security-Sensitive Operations:** None. Presentation only; no new fetch or
write. Show all messages, API, MCP and CLI still show every message.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Table tests in `channelFold.test.ts` (AC 1, 2, 3, 5, 6, 8,
9), `channelView.test.ts` (toStatus), `useSessionDecisions.test.ts` (AC 4a),
`ChannelsPanel.test.ts` integration (AC 4, 4a, 7, 7a, 9), stories with play
tests (AC 6, 10), manual check with `just dev` and VoiceOver.

**Edge Cases:** hidden members around a notice; removed plus notice in one run;
two notices in a row; leading and trailing notices; older page prepended before
a leading notice; descriptor missing, late or changed; focused notice row
folding away; scrolled back while a notice folds.

**Negative Tests:** notices into input_required, final and unknown states stay
rows; no session descriptor leaves every notice a row; `showAll` and no quiet
types render unchanged; batch announce does not double-announce.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A reader reading back may miss a move: fold text counts state
changes, header shows the current state, waiting and final moves stay visible.
Focus and scroll on live merges: hand-off and tests (AC 7a). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `docs/data-entry.md` section "Quiet message types":
run rule, announcement exception, decision placement in a working state.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-KT4LFHB, RR-NLALX2S, RR-5DWURFY, RR-BVI8KCY,
RR-UD94CIR, RR-UT8ZDEK, RR-DZ5PIQQ, RR-XJX65R3, RR-TMPC2DD, RR-H72E2JU,
RR-6ICLAYD (3 significant, 5 minor, 3 nit; all addressed in the plan).
