---
id: TKT-S51R1IH
type: ticket
title: 'Threads 3: thread followers and the thread inbox kind'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Slice 3 of RES-TU7ZMMR (from TKT-UQ0BM5G). Everyone who takes part in a thread
follows it and hears of new replies.

### Scope

- Follower rows per channel, parent seq and principal key, with state `follow` or `mute`.
- Automatic follows: the parent's author on the first reply, a replier on their first reply, a principal mentioned in the parent or a reply. Trigger, timer and reminder posts and rela identities never follow, as in `channels.ReplyTarget`.
- Mute wins over every automatic follow. Anyone who can read the channel may follow or mute.
- At most 50 followers per thread. Past that, automatic follows stop and a manual follow gets 422.
- A reply adds one `thread` inbox row per follower except the poster. These rows count toward the 200 rows per principal per channel and use `Evict`; they do not count toward the 10 mentions.
- New inbox kind `thread`. Precedence per message and target: mention, then reply, then thread.
- `PUT/DELETE .../messages/<seq>/_follow`, MCP `follow_thread`, `rela channel follow` and `mute`. MCP `watch` on `list_inbox` wakes for `thread`.
- The head and each parent carry the caller's `thread_unread` for followed threads.

### Acceptance criteria

- A replier gets a `thread` row for each later reply by someone else and none after muting.
- The parent's author still gets `reply`, not `thread`.
- A follower who cannot read the channel gets nothing shown until they can, as for mentions.
- Fan-out stays within the caps; a full target never loses another poster's unread row.
