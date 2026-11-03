---
id: PLAN-VD856N
type: planning-checklist
title: 'Planning: Realtime 3/3: channel service, HTTP API and live delivery'
started: "2026-10-12"
status: done
completed: "2026-10-12"
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
**In:** internal/realtime service (post, read, message_ops, ephemeral signals, rate limits, refs), authorization from compiled realtime.yaml roles with client ceilings, the entitymanager anchor guard (delete, rename, soft delete, purge) with Tombstone handles and Untombstone, a lazy anchor and cardinality record, v1 HTTP routes under _channels, OpenAPI, live delivery through topic rounds. **Out:** lifecycle ops, actions, promote-to-graph, retention (T4); triggers and auto.when (T5); MCP and CLI (T6); SPA (T7); cross-node wakes (phase 5). Ships as three PRs: guard, service, HTTP and live.

**Acceptance Criteria:**
Full list with tests: .ignored/t3-plan.md sections 1 and 2, as amended by section 6.
1. Post matrix per role source and ceiling (TestAuthz_PostMatrix).
2. Anchor miss gives the entity 404 body; unknown channel type gives 404 unknown_channel_type before the anchor read.
3. Read denied gives 403 naming the rule; no name lookup before it (TestAuthz_Order_NameAfterRead).
4. Unreadable ref renders {restricted: true}; posting one gives the uniform 422.
5. One live frame per post to that channel's subscribers, none to others; no frame for a hidden channel on overflow.
6. Removal placeholder keeps thread fields; removed.by and reason only for remove-gate holders.
7. Edit runs the full post gate; within: and from: honoured.
8. Ephemeral: 202, ring 64, TTL <= 60s, 1 KiB props, expiry frame without posts, never stored.
9. Rate limits on every write op plus a per-identity aggregate; 429 Retry-After; caps refuse rather than evict.
10. Caps: 16 KiB, 32 props, 200 channels, page 200, title and reason runes.
11. Anchor guard: block refuses before any write and never depends on a hidden face; detach before the Tx and Untombstone on failure; rename keeps channels reachable under newID; renaming onto a soft-deleted id is refused and keeps its channels.
12. A changed anchor mode or cardinality of a recorded type fails boot and is refused on reload.
13. Degraded mode: entity-property roles grant nobody.
14. Budget flat at 10 and 50 messages and channels.
15. No realtime.yaml: routes 404, no OpenAPI paths.
16. ReadOnlyACL refuses writes; NopACL permits.
17. plimsoll and arch-lint pass.

## Research

- [x] For larger features: run `/research` to create a structured research doc (RES-1UCUBA)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: covered by RES-1UCUBA for the phase)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-1UCUBA (phase research); plan in .ignored/phase3-plan.md and .ignored/t3-plan.md

**Existing Solutions:**
- comments/authz.go: anchor-gated reads with the uniform 404.
- leases_handler.go and lease service: v1 handler, apiwire and openapi shape, 422 face_required, identity keying.
- live.go presenceRound and withPresenceProbe: topic rounds set from a 200 replay.
- changefeed.Bus Subscribe {Wake, Take}: per-consumer change sets.
- No external library: token buckets and the ring are small and need custom keys and caps.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
See .ignored/t3-plan.md sections A and B, with section 5 decisions and section 6 design review changes, which take precedence.

**Files to modify:**
internal/realtime/*, internal/channels (Tombstone handle, Untombstone, record), internal/entitymanager (anchor guard, AnchorBlockedError), internal/appbuild (guard, service wiring, boot check), internal/acl/request.go (CeilingPermitsVerb), internal/principal (Identity), internal/dataentry (channels handler, livepath, live topics), internal/apiwire/v1/channels.go, internal/openapi/channels.go, MCP and CLI delete/rename 409 mapping.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
HTTP bodies: DisallowUnknownFields, MaxBytesReader, props validated against the message type (allowlist), refs resolved through visibility. URL: structural parse; channel type and name by regex; ids through the entity resolver. Config: compiled by T1. Invalid input gives 400/413/422 with no entity content.

**Security-Sensitive Operations:**
Channel reads (anchor 404, read role 403, then name); posts and ops (role sources plus client ceiling; one acl.Request per op); entity delete and rename (pre-commit guard after full authorization pre-pass); live frames (topic only from a 200 replay); ephemeral memory (byte budget, per-identity cap, refuse not evict).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
.ignored/t3-plan.md section 2 test table plus the section 6 tests (rename end to end, soft-deleted target, hidden sibling face, Untombstone batch, post-commit sweep, overflow without frame, expiry frame, post during baseline).

**Edge Cases:**
Empty props and missing required props (400); limit 0 and over 200 (clamped); names with uppercase or traversal (404 by regex); concurrent first post during delete (post-commit sweep); delete and recreate of an id (Untombstone by handle only); bucket and ring caps reached (429).

**Negative Tests:**
Hidden anchor (404 identical to entity GET); read denied (403); post without role or ceiling (403); oversize (413); unreadable ref (422); rate exceeded (429 Retry-After); block on delete (409 naming the type); anchor or cardinality change (boot fails, reload refused).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Size (three PRs). Stranded detached channels on double failure (documented, Untombstone by handle). External fs edits (reconciler or documented residual). Per-process ephemeral and buckets (documented until phase 5). Replay cost (round cap, head-only live shape). Effort xl.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
docs/realtime.md and GUIDE-realtime (channels API, limits, guard behaviour); docs/data-entry/api-reference.md (_channels routes); docs/acl-security.md (channel 404/403 order, block 409 one-bit signal).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-QEVDCA, RR-9P495X, RR-1UG54A, RR-2OCZS1, RR-5VJN2O, RR-X9N6WX, RR-MY6A09, RR-5A64TZ, RR-PH3RHT, RR-VKKIBT, RR-9O3C9M, RR-GDLFO2, RR-OTFZKR, RR-G6K2EZ, RR-7P5JOX, RR-KFLOY8, RR-OBPUX5, RR-Y218R8, RR-72EARV, RR-MZ18OI, RR-AOUEHB, RR-IQDHGX, RR-393439, RR-KKHWVZ, RR-8XFGLA, RR-UW0KET, RR-O8YJPD, RR-F361VR, RR-32SB1P, RR-IMMQ5O, RR-NK9HHN, RR-57IWUQ, RR-ZZNV2J, RR-D1WNDJ, RR-6QYPX1, RR-IAC4T3 (all addressed by the plan update).
