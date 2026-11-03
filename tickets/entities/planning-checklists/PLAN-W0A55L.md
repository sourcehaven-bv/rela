---
id: PLAN-W0A55L
type: planning-checklist
title: 'Planning: Realtime 2/2: presence over live views, avatars and claim badge'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope (backend):** internal/presence tracker; GET
/api/v1/{plural}/{id}/_presence; the live presence shape with join and leave
tied to the subscription; filtered presence rounds; `_lease` on the `_views`
entry; docs. **Out:** SPA avatars and badge (TKT-IMBF7U), cross-instance
presence, MCP agent presence, invisible mode.

**Acceptance criteria:**
1. `_presence` on a hidden, absent, wrong-plural or malformed address answers the entity GET's 404 body; a live subscription to it records no viewer.
2. A join reaches other subscribers within two ticks; a leave after close lands at 15 s plus or minus one tick on a fake clock; a rejoin within grace keeps `since`.
3. Revoking read removes the viewer when that subscription's replay next runs, and both sides get a frame.
4. A presence change on one face replays no subscription outside it; a subscriber of a hidden entity gets no frame.
5. Two tabs of one identity are one row; the caller never sees themselves, including from another tool; 25 viewers give 20 rows and more 5.
6. principal.Unknown never joins; per-identity cap 64 including grace entries is enforced before the global 4096 cap.
7. Retire discards presence; a reconnect rejoins.
8. The view entry carries `_lease` when held.
9. Router walk and OpenAPI tests cover `_presence`.
10. The security design review items S1, S2, M1, M2, M4 each have their named test.

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-process tracker over the existing live hub)
- [x] Checked codebase for similar patterns or reusable code (live hub rounds, readAddressedOr404, lease wake source)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: Phoenix Presence and Liveblocks surveyed in RES-1UCUBA)
- [x] Reviewed relevant rela concepts for prior art (authorization, data-entry-ui)

## Approach
- [x] Technical approach chosen and documented (.ignored/phase2-plan.md "Presence design")
- [x] Approach builds on existing patterns (not reinventing) (live subscriptions are the join; fixed-clock rounds)
- [x] Alternatives considered (document why rejected) (join on plain GET, tracker goroutine with timers, separate presence SSE event, family keying)
- [x] Dependencies identified (packages, APIs, types) (live hub, visibility reader, principal identity fields)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (route plural and id, world parameter, live subscription set)
- [x] Input validation approach defined (allowlist preferred over blocklist) (parseLivePath allowlists keys; shared read gate)
- [x] Security-sensitive operations identified (file access, auth, crypto) (existence oracles, viewer disclosure, memory caps)
- [x] Error handling doesn't leak sensitive information (uniform 404; face_required only after the read gate)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (stale replays, churn, overflowed rounds, face moves)
- [x] Negative test cases defined (invalid input, error conditions) (hidden, malformed, non-presence probe, Unknown)
- [x] Integration test approach defined (not just unit tests) (live SSE tests through the router with a fake clock)

## Risk Assessment
- [x] Technical risks assessed with mitigations (lock order, global lock hold time, deterministic tests)
- [x] Security risks assessed (security design review: 2 significant and 4 minor folded into the design)
- [x] Effort estimated (xs/s/m/l/xl) (m)

## Documentation Planning
- [x] User-facing docs identified (skip if internal refactor) (api-reference, GUIDE-acl-security)
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: SPA in TKT-IMBF7U)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing API)

## Design Review
- [x] Run `/design-review` before starting implementation (security design review recorded as addressed RRs on this ticket)
