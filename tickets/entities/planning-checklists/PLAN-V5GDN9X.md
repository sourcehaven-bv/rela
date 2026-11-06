---
id: PLAN-V5GDN9X
type: planning-checklist
title: 'Planning: Fork the ygo relay and stamp awareness identity on the server'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: fork ygo `provider/websocket` v1.55.0 and its internal packages
into `third_party/ygows`; a `ConnectionConfig.StampAwareness` hook; refusing an
awareness entry for a live client another connection owns; rela stamps the
principal as the cursor name and keeps only a `#rrggbb` color. Out: a Postgres
`cluster.Relay` for multi-node fan-out (follow-up); display names from person
entities; any frontend change.

**Acceptance Criteria:**
1. A client that sends another user's name is stored under its own principal:
`TestCollab_PresenceCarriesThePrincipal`,
`TestStampAwareness_RoomStoresTheServerName`.
2. A client cannot update or remove another connection's awareness client:
`TestAwarenessOwnership_PeerCannotTouchAnotherPeersClient`.
3. The fork keeps the upstream MIT licence and passes its upstream tests.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: options compared with Jeroen in the session: colour-only cursors, proxy, upstream hook, fork)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** ygo's relay has no per-connection awareness hook and
owns the socket upgrade (`ServeHTTP` with its own gorilla `Upgrader`). Its
`applyAwareness` already tracks clientIDs per peer for the per-connection cap,
so ownership extends that map. Upstream `crdt`, `sync` and `awareness` stay as
library imports.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Copy the relay into `third_party/ygows` (excluded from
golangci-lint, own arch-lint components, Server size pinned for plimsoll,
excluded from the coverage floor). `presence.go` decodes an awareness frame,
passes each live state through the hook and re-encodes it, using `json.Number`
so cursor positions round-trip exactly. Alternatives: colour-only cursors (loses
names), an in-process proxy (double framing, close-code passthrough), an
upstream hook (blocks on another project's release).

**Files to modify:** `third_party/ygows/**`, `internal/dataentry/collab.go`,
`internal/dataentry/collab_test.go`, `.go-arch-lint.yml`, `.testcoverage.yml`,
`go.mod`, `GUIDE-data-entry`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Awareness frames from any writer connection. The
name comes only from the authenticated principal; the color is kept only when it
matches `^#[0-9a-fA-F]{6}$`; a non-object state or more than 1024 entries drops
the frame.

**Security-Sensitive Operations:** Identity shown to other editors. Read-only
connections' awareness stays dropped. Errors are logged at debug level and never
sent to the client.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** Null states pass through; missing `user` is added; integers
above 2^53 keep their value; trailing data after the JSON object is refused.

**Negative Tests:** Non-object state, truncated frame, too many entries, unsafe
color, another connection's client id.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** The fork drifts from upstream fixes; the README lists every change
and each is marked `rela fork`, so rebasing is mechanical. A client that
reconnects before its old connection is removed has its cursor refused until
then; its next heartbeat restores it. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes
- [x] ~~CLAUDE.md~~ (N/A: no new convention)
- [x] ~~README.md~~ (N/A: fork README is in third_party/ygows)
- [x] ~~N/A~~ (N/A: the guide changes)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design chosen with Jeroen in the session)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** none
