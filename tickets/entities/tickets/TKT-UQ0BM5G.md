---
id: TKT-UQ0BM5G
type: ticket
title: Research reply threads in realtime channels
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Decide how replies and threads should work in realtime channels, and split the
outcome into implementation tickets.

### What exists today

- The store already keeps one-level threads. `reply_to` may only name a top-level message that is not removed (`checkParent` in `internal/channels/sqlchannels/sql.go`; the file store has the same rule). The parent keeps a `reply_count`.
- The wire, MCP `read_channel` and the CLI (`N replies`) carry `reply_to` and `reply_count`.
- The web app ignores the thread structure. Every reply shows inline in seq order, with a quote of its parent (`replyQuote` in `channelView.ts`). `reply_count` is used only to decide whether an inbox decision is still open.
- A reply creates an inbox row for the parent's author only. Other people in the thread hear of a reply only through a mention.

### The gap

In a busy channel, a side discussion interleaves with the main stream. Nobody
can read one thread on its own, and people who took part in a thread are not
told about later replies.

### Questions the research must answer

1. Web app presentation: inline quote (today), Slack-style thread panel with "N replies" on the parent, or a hybrid (thread panel plus an optional "also send to channel").
2. Notification: the user chose that thread participants follow a thread. Define who counts as a participant (parent author, repliers, people mentioned in the thread), how to stop following, and how this fits the 200-row cap and the 10-principal limit per message.
3. Read state: does the per-channel read marker suffice, or does a thread need its own marker so that a hidden reply does not count as unread in the main stream?
4. API, MCP and CLI: reading one thread (`GET .../messages?thread=n` or similar), paging, and live frames for a thread.
5. Interaction with choices, decisions and sessions, which already rely on `reply_to` and top-level questions.
6. Retention and removal: what happens to a thread when its parent is removed.

### Out of scope

Nested threads (replies to replies). The one-level rule stays.

### Acceptance criteria

- A research entity records the options, trade-offs and a recommendation for each question above.
- The user approves the recommendation.
- Implementation tickets exist for the approved slices, linked to this ticket.
