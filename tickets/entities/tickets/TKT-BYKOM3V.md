---
id: TKT-BYKOM3V
type: ticket
title: Attachments in channel messages
kind: enhancement
priority: medium
effort: xl
started: "2026-11-10"
completed: "2026-11-11"
status: done
---

## Description

Channel messages cannot carry files. A message type's properties allow `string`,
`markdown`, `integer`, `boolean`, `date`, `datetime`, `ref` and enums, but no
`file`. People in an incident room or a ticket run channel want to drop a
screenshot, a log excerpt or a PDF into the conversation, and agents want to
post an artifact (a report, a diff) with a message.

Entity attachments already exist: a `file` property, bytes in
`store.AttachmentManager`, the `attachment.Service` write path with the MIME
allowlist, scan and transform pipeline (`docs/attachment-security.md`). Channels
live outside the graph (`internal/channels`), so those bytes cannot simply hang
off the anchor entity.

## Questions to settle in planning

- Where do the bytes live? Per-channel blob storage in each channel backend
(filechannels, sqlitechannels, pgchannels), or a shared blob seam keyed by
channel and message.
- The channels contract says removing a message erases its content. The
bytes must go with it, and with retention and `detached_retention`.
- Which policy applies: the global attachment floor, a per-property
`allow`/`max_size`/`scan`/`transform`, or both.
- Read access follows the channel's `read` roles; download must be gated the
same way as message reads, with the hardened download headers.
- Can `promote_to.map` copy a message file into an entity `file` property?
- Caps: files per message, bytes per file, bytes per channel.
- Web composer upload and inline preview; MCP and CLI post with a file.
