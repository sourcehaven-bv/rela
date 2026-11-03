---
id: PLAN-BRAV93
type: planning-checklist
title: 'Planning: Realtime 2/2b: SPA presence avatars and claim badge'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope:** usePresence composable over the `_presence` live subscription;
PresenceAvatars on EntityDetail; LeaseBadge (text only) on detail, list and
kanban; a kanban live subscription so the badge refreshes; vitest and a
two-context Playwright spec. **Out:** claiming from the SPA, invisible mode, MCP
agent presence.

**Acceptance criteria:**
1. Opening an entity in two browser contexts (alice, bob) shows each other's avatar within 2 s; closing bob's tab removes his avatar within 25 s.
2. The caller never sees their own avatar; more than 5 viewers render 5 avatars plus "+N" (the count includes the server's `more`).
3. A refused presence refetch (404) clears the avatars and does not keep them.
4. A claim made through the API shows "Claimed by user (label)" on detail, list and kanban within 2 s; release removes it.
5. Status and label render as text: a status of `<img src=x onerror=alert(1)>` appears literally (vitest); no v-html.
6. The kanban board refreshes on lease and entity changes through a live subscription.
7. Faced types subscribe to presence with an explicit face (the server refuses bare ids for faced types).

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: reuses rela-components avatars)
- [x] Checked codebase for similar patterns or reusable code (useLiveSubscription, liveListPath, WorldBadge placement, RlAvatarGroup)
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art (data-entry-ui)

## Approach
- [x] Technical approach chosen and documented (.ignored/phase2-plan.md "Presence design" section 3)
- [x] Approach builds on existing patterns (not reinventing) (live subscriptions, shouldDropHeldContent)
- [x] Alternatives considered (document why rejected) (polling presence: polling clients would never leave; fetching entity GET on detail for _lease: extra request and subscription)
- [x] Dependencies identified (packages, APIs, types) (_presence API, _lease on view entries and list rows)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (server-supplied user, tool, label, status)
- [x] Input validation approach defined (allowlist preferred over blocklist) (text interpolation only)
- [x] Security-sensitive operations identified (file access, auth, crypto) (rendering holder-written status)
- [x] Error handling doesn't leak sensitive information (refused refetch drops held presence)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (404 refetch, caps, faced types, kanban beyond the first page)
- [x] Negative test cases defined (invalid input, error conditions) (markup in status, refused refetch)
- [x] Integration test approach defined (not just unit tests) (Playwright with two contexts through -principal-header)

## Risk Assessment
- [x] Technical risks assessed with mitigations (kanban over 100 cards misses lease wakes past the first page: documented)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (m)

## Documentation Planning
- [x] User-facing docs identified (skip if internal refactor) (data-entry guide: who is viewing, claim badge)
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A)
- [x] docs/data-entry.md - UI changes
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing UI)

## Design Review
- [x] Run `/design-review` before starting implementation (presence design and its security review cover the SPA rendering rules)
