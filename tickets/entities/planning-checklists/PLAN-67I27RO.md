---
id: PLAN-67I27RO
type: planning-checklist
title: 'Planning: Compact session header by configuration'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Full plan: `.ignored/plan-TKT-56TPOTE.md` (working copy). Summary below.

**Problem.** The session header lists every state as a pill in one wrapping row;
the agent runner PoC's 13 states take two rows. The folded mode (summary line
plus **Show all states** toggle) only switches on below 32rem of container
width, through a CSS container query. No configuration can turn it on.

**Scope:**

In:

- `data-entry.yaml`: `channels.<type>.session.display: compact`, checked at
load, on reload, by `rela validate` and by Configure; served in `GET
/api/v1/_config`.
- Configure: `channels.*.session.display` editable.
- Server: `channels.Session.Previous`, the state before the last move to
another state, stored by every channel backend (file meta, pg and sqlite columns
with migrations), served as `session.previous` on `SessionView`.
- SPA: `RlChannelSession` `compact` prop; in compact at 32rem and wider the
folded mode shows a context strip (previous, current, next states) instead of
the summary; the full list stays behind the existing toggle.
- Docs: data-entry guide, realtime guide, OpenAPI.

Out: a `realtime.yaml` key; a `full` value; the inbox row (it shows only the
state label, so it is already compact; `InboxSession` gets no `previous`); CLI
text output; remembering the toggle; backfill of `previous`.

**Acceptance Criteria:**

1. The key loads; absent, empty, null, `session: {}` mean auto; other values
and unknown keys are refused with the path; a session-less channel type is a
warning. Test: `TestChannelsPresentation_Validate`, `TestValidateChannels`.
2. Configure may edit it. Test: `TestAllowlist_ClassifiesEveryKey`,
`TestCheckEdit_DataEntry`.
3. The SPA gets the key. Test: `TestConfig_ServesChannels`, `channelView.test.ts`.
4. `previous` is stored and served on every backend; a re-entry and any
non-moving write keep it; old rows read empty. Test: `channelstest` conformance
including `SessionPreviousKept`, migration tests, wire golden.
5. Compact, normal status, 32rem and wider: label, strip, toggle; unfolding
hides the strip and shows the list on its own row. Test:
`ChannelsPanel.test.ts`, Playwright.
6. Compact, other statuses: final, paused, loading, stranded as specified;
previous left out when absent, equal to current or undeclared. Test:
`ChannelsPanel.test.ts`.
7. Below 32rem compact equals today's folded mode. Test: Playwright at 24rem.
8. Auto unchanged. Test: Playwright at 60rem and 24rem, existing tests.
9. Accessibility: labelled `<ol>`, hidden text per item, one `aria-current`.
Test: `ChannelsPanel.test.ts`.
10. Docs updated, including the corrected "keeps only the current state".

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach follows two direct precedents, `display` on message types and `session_cause`; alternatives are recorded in the plan)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: an in-repo presentation option; the in-repo precedents decide the shape)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- No library needed: a config key, a stored field and CSS.
- `channels.<t>.messages.<mt>.display` (PLAN-0RKHWQ, TKT-03DMXH):
presentation of channels lives in `data-entry.yaml`, not `realtime.yaml`;
allowlisted values; Configure editable.
- `session_cause` (TKT-V05706X, commit 4ab4678f0): the pattern for a new
session snapshot field across `channels.Session`, filechannels meta, sqlchannels
columns, pg and sqlite migrations, wire and OpenAPI.
- The existing folded mode in `RlChannelSession.vue:340-392` is extended, not
duplicated.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- Config: `ChannelPresentation.Session *SessionPresentation{Display}`;
allowlist value `compact`; unknown-key check; warning on a session-less channel
type via `ChannelSchema.BusyFacts().HasSession`.
- Store: `ApplySession` sets `Previous: move.From` unless it is a re-entry;
file meta `session_previous`; sqlchannels column through `chanCols`,
`scanChannelWith`, `bumpSession`; pg migration 0040, sqlite v33.
- Wire: `SessionView.Previous`, apiwire, OpenAPI.
- SPA: `compact` prop; root classes `--compact` and `--no-context`; strip as
a labelled `<ol>` rendered with `v-if="compact && current && !expanded"`; CSS
selectors with matched specificity so the container query still folds compact
below 32rem; toggle look moved to its base rule.

**Alternatives rejected:** a `realtime.yaml` key (structural file); previous by
declaration order (a session branches); previous from the loaded notice (page
may not hold it); notice lookup by cause (timer moves have no cause); no
previous (brief asks for it; kept as fallback); a second component (brief); JS
ResizeObserver folding (container query works).

**Files to modify:** listed in the plan; main ones:
`internal/dataentryconfig/channels.go`, `internal/configedit/allowlist.go`,
`internal/channels/session.go`, `filechannels/file.go`, `sqlchannels/sql.go`,
`pgstore/migrations/0040_channel_session_previous.sql`, `sqlitedb/migrate.go`,
`sqlitedb/sqlitedb.go`, `realtime/session.go`, `dataentry/channels_wire.go`,
`apiwire/v1/channels.go`, `openapi/channels.go`, `RlChannelSession.vue`,
`types.ts`, `useMessages.ts`, `messages.nl.ts`, `channelView.ts`,
`OpenChannel.vue`, `types/channels.ts`, `types/config.ts`, docs and E2E files.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `data-entry.yaml` is operator-authored and not
secret. The value is checked against an allowlist (`compact`); unknown keys are
refused with the path.

**Security-Sensitive Operations:** none new. `previous` is a state name that
every channel reader already sees in the session notices (`from`); the head is
served only to readers of the channel. `Previous` is written only inside the
store's existing session move, under its lock.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** per AC above; the plan holds the full table.

**Edge Cases:** session never moved; re-entry; previous no longer declared;
final; paused; loading; stranded; self-transition (not shown as next); many next
states and long labels (wrap, never truncated); old store rows (no previous).

**Negative Tests:** bad display value, `Compact`, unknown key under `session`,
unknown channel type (refused); session-less channel type (warning); unknown
value in the SPA (dropped by `presentationView`).

**Integration:** store conformance on file, pg and sqlite; migration tests from
the previous schema version; Playwright for all four mode and width
combinations, because jsdom does not evaluate container queries.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A storage change for a display feature. Mitigation: mirrors
`session_cause`, conformance tests including a kept-value case. Fallback: ship
without previous first.
- CSS specificity between the compact class and the container query.
Mitigation: selectors fixed in the plan; Playwright checks.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no command change)
- [x] docs/data-entry.md - UI changes (new "Compact session header" subsection and GUIDE mirror; docs/realtime.md and mirror too)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing docs are needed)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 16 findings (0 critical, 6 significant, 8 minor, 2
nit), all addressed in the plan. Significant: RR-V6K0FZV, RR-B0HTXQS,
RR-KEN3QD5, RR-C91FORX, RR-9R4JZTG, RR-0O7MKT7. Minor: RR-9JZXTS0, RR-Y5BOKY7,
RR-Z764CB2, RR-MZFMBTL, RR-ZIEB9F1, RR-89EBY15, RR-SLZIT4C, RR-F28XSVE. Nit:
RR-LNTYETK, RR-GU26UVA.
