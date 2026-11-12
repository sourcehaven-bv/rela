---
id: PLAN-DMUEED8
type: planning-checklist
title: 'Planning: Threads 1: read one thread and the top-level stream'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `thread=` and `top=1` page filters on every channel backend,
`last_reply_at` and `inline` on a message, the inline rule in the server,
HTTP/MCP/CLI options. Out: read markers, followers, web app panel (slices 2-4).

**Acceptance Criteria:**
1. A thread page holds the parent and only its replies, paged by seq and by after_rev (channelstest ThreadPages).
2. A top page holds no reply that is not inline, and a full page is never thinned by hidden replies (ThreadPages, filter in the store query).
3. last_reply_at matches the newest reply; a removal keeps it (ThreadPages, ThreadRemoval).
4. A thread on an unreadable anchor answers like any read of it (realtime TestRead_Threads).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-TU7ZMMR

**Existing Solutions:** Slack and Zulip thread models (in RES-TU7ZMMR). The
inline rule mirrors `channelView.ts` in the web app; reply_count bookkeeping in
filechannels/sqlchannels is extended rather than duplicated.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `channels.Page` gains Thread and Stream; `Message` gains
LastReplyAt and Inline, decided at append from `NewMessage.InlineUnder` (set by
realtime from the channel type's choices) and the session move. SQL backends
store `in_stream` and `last_reply_at` columns with partial indexes so the filter
runs in the query; the file store filters in its scan with `Page.Admits`.
Alternative rejected: computing inline on read, which cannot page in SQL.

**Files to modify:** internal/channels (channels.go, filechannels, sqlchannels,
channelstest), internal/sqlitedb (migration v37), pgstore migration 0045,
internal/realtime (read, render, service), internal/dataentry (handler, wire),
internal/apiwire/v1, internal/mcp/tools_channels.go, internal/cli/channel*.go,
frontend/src/types/channels.ts.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `thread` must be a whole number of at least 1,
`top` is 1/0/true/false; anything else is 400 (HTTP), a tool error (MCP) or a
flag error (CLI). Thread with top is refused.

**Security-Sensitive Operations:** Reads go through the existing `Service.Read`
access path; the filters only narrow a page the caller may already read.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** channelstest conformance on file, sqlite and postgres;
realtime TestRead_Threads; dataentry TestChannels_ThreadPages; MCP and CLI round
trips; sqlite and postgres migration tests.

**Edge Cases:** thread naming a reply or a missing seq (empty page); removed
parent and removed inline reply; legacy rows (backfill).

**Negative Tests:** thread=-1, thread=x, top=yes, thread with top.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Backfill cannot tell old SQL choice answers from plain replies, so
they leave the stream (accepted; replies stay in their thread). last_reply_at is
not backfilled. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: docs are slice 5, TKT-771TAL2)

**Documentation Impact:** API reference and channel guides, in slice 5.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design reviewed in RES-TU7ZMMR with the user)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no separate design review)

**Design Review Findings:** None.
