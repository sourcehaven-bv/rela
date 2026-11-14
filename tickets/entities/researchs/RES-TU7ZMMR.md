---
id: RES-TU7ZMMR
type: research
title: How should reply threads work in realtime channels?
summary: Thread panel with inline exceptions for choice answers and session moves; per-thread read markers; stored followers (author, repliers, mentioned; follow/mute; cap 50) with a new inbox kind thread; thread= and top= page filters; five slices.
status: done
---

## Problem

A realtime channel shows every reply inline, in seq order, with a quote of its
parent. In a busy channel a side discussion interleaves with the main stream,
nobody can read one thread on its own, and only the parent's author hears of a
reply. The user decided that everyone who takes part in a thread follows it.
This research decides how threads look, how they are read, who is told, and what
the API offers, so the work can be split into tickets.

Out of scope: nested threads. A reply still targets only a top-level message.

## Context

### What the store already does

- **One-level threads.** Both stores refuse a parent that is missing, removed or itself a reply (`filechannels/ops.go` `replyParent`, `sqlchannels/sql.go` `checkParent`).
- **Parent counter.** A reply raises the parent's `reply_count` and bumps the parent's `rev`, so a client that reads `messages?after_rev=` gets the new count. Removing a reply never lowers the count; the removed stub keeps `reply_to` and `reply_count`.
- **Removed parent.** Removing a parent leaves its replies alone. New replies to it are refused.
- **Page reads.** HTTP takes `after`, `before`, `after_rev` and `limit`. MCP `read_channel` and `rela channel read` take the same cursors. Nothing filters by `reply_to`.
- **Live frames carry no data.** A write sends one `live:changed` frame per channel. The client refetches `after_rev` and merges by seq.
- **Read state is per channel.** One marker per reader (user plus organization) per channel, a seq that only moves forward. Mention and reply rows count as read once the channel marker passes their seq.
- **Addressed rows.** A reply adds one `reply` row for the parent's author (`channels.ReplyTarget`), except for trigger, timer and reminder posts, rela identities and the poster's own user. `MaxMentions` = 10 counts mentions only. `MaxAddressedPerTarget` = 200 rows per principal per channel, freed by `Evict`.

### Features that depend on `reply_to`

- **Choices.** A question must be top-level. It counts as answered once a non-removed reply of the answer type is held (`choiceViews` in `channelView.ts`).
- **Inbox decision head.** Offers choices only while the message has no reply at all (`offersChoices`).
- **Session decisions.** A decision is posted as a reply to its cause message when the cause is top-level (`decisionReplyTo`).

These answers are replies, but they are part of the main conversation: they move
a session or settle a question.

### Prior art

- **Slack.** A parent shows "N replies" and the last reply time. Replies open in a side panel. "Also send to channel" copies a reply into the stream. The parent's author, every replier and everyone mentioned in the thread follow it; anyone may follow or unfollow. Thread replies do not make the channel unread; followed threads have their own unread view.
- **Matrix (MSC3440, MSC3771).** A thread is a relation on the event. Read receipts carry a thread id, so a client tracks one marker for the main timeline and one per thread. This is the closest model to rela's seq markers.
- **Zulip.** Every message has a topic; a channel is a set of topics. That is a different model (no top-level stream) and does not fit rela's session and decision flows.
- **Discord.** A thread is a child channel with its own membership. In rela that would mean a channel per thread, with its own `read:`, lifecycle and session. Too heavy for a side discussion.
- **rela comments** (`docs/comments.md`) have no threading at all; nothing to reuse.

## Options

### 1. Presentation in the web app

**A. Inline quote (today).** No UI work. Does not solve the problem.

**B. Thread panel.** The main stream shows top-level messages only. A parent
with replies shows "N replies · last reply time". Clicking it opens the thread
in a side panel (a sheet on mobile), with its own composer.
- Pro: solves the interleaving; matches what users know from Slack and Teams.
- Con: an answer to a choice or a session decision would disappear from the stream, although it is the main event.
- Effort: L.

**C. Inline expand.** Replies are folded under their parent in the stream and
expand in place.
- Pro: no second pane.
- Con: a long thread pushes the stream away; a new reply to an old parent is hard to notice; paging by seq no longer matches what is on screen.
- Effort: M to L.

**B with exceptions.** As B, but a reply that answers a choice, or that a
session decision posted, also shows inline in the stream, as today. Those are
the replies the existing features already single out.

### 2. Read state

**R1. Channel marker only (today).** Opening the channel moves the marker past
replies the reader never opened. A follower then loses the thread's unread rows
without having seen them. Does not work with option B.

**R2. Thread markers.** A reader has one marker for the main stream and one per
thread they read, as in Matrix. The channel's unread count covers top-level
messages only (and the inline exceptions, see below). A thread's unread count
covers its replies above the thread marker. Reply and thread rows in the inbox
turn read against the thread marker.
- Pro: correct; one marker shape, reused.
- Con: a new table in all three backends (`thread_reads`: reader, channel, parent seq, seq), a new count split, a mark-read route per thread.
- Effort: L.

**R3. Read flag on each addressed row.** The inbox row itself records that it
was seen.
- Con: breaks the rule that the marker alone decides read state; does not give a thread unread count for non-followers.

Inline exceptions under R2: the web app shows those replies in the stream, so it
marks their thread read up to them when it marks the channel read. The store
needs no special case.

### 3. Who follows a thread

**F1. Derived on each reply.** Address the parent's author plus every distinct
earlier replier, found by scanning the thread.
- Pro: no new state.
- Con: no way to unfollow; the scan grows with the thread; mentioned people are not followers.

**F2. Stored followers.** A follower row per (channel, parent seq, principal
key) with state `follow` or `mute`.
- The parent's author follows when the first reply arrives.
- A replier follows on their first reply.
- A principal mentioned in the parent or a reply follows.
- Anyone who can read the channel may follow or mute by hand. Mute wins over every automatic follow.
- Trigger, timer and reminder posts and rela identities never follow, matching `ReplyTarget`.
- A reply adds one `thread` row per follower except the poster. Those rows count toward the 200 per principal per channel and use the same `Evict`. They do not count toward the 10 mentions, which limit what one author writes. A thread has at most 50 followers; past that, automatic follows stop and a manual follow is refused with 422.
- Effort: M to L.

### 4. Inbox kind

Today `reply` means "a reply to your message". A follower who did not write the
parent gets a new kind, `thread`. A watch that asks for `reply` keeps its
meaning. The parent's author still gets `reply`. One row per message per target,
with the existing precedence extended: mention, then reply, then thread.

### 5. API, MCP and CLI

- `GET .../messages?thread=<seq>`: the parent and its replies, with the usual cursors.
- `GET .../messages?top=1`: top-level messages and the inline exceptions only, so a page is not filled by replies the stream hides. The server must decide the exceptions, so the rule moves from the frontend into `realtime` (a reply of the answer type of a choice on its parent, or a reply that moved the session, which the store already records).
- The parent carries `last_reply_at` next to `reply_count`. Both stores already rewrite the parent on a reply.
- `PUT .../messages/<seq>/_read` marks a thread read. `PUT/DELETE .../messages/<seq>/_follow` follows and mutes.
- The head and each parent carry the caller's `thread_unread` for threads they follow.
- MCP `read_channel` gains `thread`; `mark_read` gains `thread`; a new `follow_thread` tool. CLI `rela channel read --thread`, `mark-read --thread`, `follow`/`mute`.
- Live: no new topic. The channel frame already wakes the panel, which refetches `thread=<seq>&after_rev=`.

### 6. Removal and retention

Keep today's rules: a removed parent becomes a stub, its thread stays readable,
and new replies are refused. Followers and thread markers live and die with the
channel, like read markers and mention rows. `reply_count` keeps counting
removed replies; the panel shows their stubs, so the number matches what it
lists.

## Recommendation

Option **B with exceptions**, read state **R2**, followers **F2**, a new inbox
kind `thread`, and the API in section 5.

The trade-off: R2 and F2 add two per-user tables to every backend and a second
read marker shape. Without them, a thread panel loses notifications the moment a
follower opens the channel, which defeats the user's decision that participants
follow. Deriving followers (F1) would be cheaper, but gives no way to leave a
noisy thread.

"Also send to channel" is left out of the first slices. It needs a broadcast
flag on the message and a rule for how it counts; add it when users ask.

### Slices

1. **Store: thread reads.** `thread=` and `top=` page filters, `last_reply_at`, the inline-exception rule in `realtime`; all backends and `channelstest`. M.
2. **Store: thread markers and counts.** `thread_reads`, split unread counts, mark-read per thread; HTTP, MCP, CLI. L.
3. **Followers and the `thread` inbox kind.** Follower rows, automatic follows, follow and mute on every transport, fan-out under the caps, inbox and watch. L.
4. **Web app thread panel.** "N replies" on the parent, side panel with composer, inline exceptions, follow and mute, unread badges. L.
5. **Docs.** `realtime.md` sections on replies, read markers, inbox and the API. S.
