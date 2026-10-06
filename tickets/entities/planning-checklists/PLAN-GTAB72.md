---
id: PLAN-GTAB72
type: planning-checklist
title: 'Planning: Investigate Milkdown concurrent (multi-user) editing for entity bodies'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Investigation only: survey Milkdown's multi-user editing support,
propose options for rela, and confirm the chosen option's unknowns with a
throwaway spike. Out of scope: any committed code; implementation is TKT-CJL604,
TKT-RROOP1 and TKT-WVSTXC.

**Acceptance Criteria:**
1. Options with trade-offs, effort and a recommendation are recorded
(RES-L4FVT0). Verified: the research entity has Problem, Context, Options,
Recommendation.
2. A direction is chosen with the user and recorded (DEC-OHJEKG, Option A).
3. The three unknowns of Option A are answered by a spike (RES-L4FVT0,
"Spike results").

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-L4FVT0

**Existing Solutions:** `@milkdown/plugin-collab`, Yjs, y-prosemirror,
y-websocket, `reearth/ygo`, Hocuspocus, y-sweet, prosemirror-collab, Automerge,
Loro. Prior art in rela: TKT-2VDVHF, TKT-34XS2R, the pgstore change feed
(TKT-WZYWM9). Details in RES-L4FVT0.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** RES-L4FVT0 Option A (DEC-OHJEKG).

**Files to modify:** ~~N/A~~ (N/A: investigation ticket; files are listed in
TKT-CJL604)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** WebSocket frames from browsers; read-only peers'
writes dropped server-side; `disableBc` required. Recorded in RES-L4FVT0 and
TKT-CJL604.

**Security-Sensitive Operations:** WebSocket upgrade behind the existing
Host/Origin/JWT/ACL middleware; per-connection ACL re-check. Recorded in
RES-L4FVT0.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Spike: y-websocket interop script against `ygo`; corpus test
of 1,312 bodies in collab mode; upgrade through a `statsResponseWriter` replica.

**Edge Cases:** Read-only peer, late joiner, concurrent edits, BroadcastChannel
bypass, version skew, null attributes, marks on inline nodes.

**Negative Tests:** Read-only edits must not relay (verified); upgrade through a
wrapper without `Hijack` fails (reproduced).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** `ygo` is young (kept behind a narrow seam); y-prosemirror patch must
be maintained until upstream; saving depends on a connected browser. Effort: m
(investigation); implementation L + M + M.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: investigation; docs belong to TKT-CJL604)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no implementation in this ticket)

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: no implementation; design review runs on TKT-CJL604)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** N/A
