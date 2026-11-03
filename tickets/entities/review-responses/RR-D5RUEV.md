---
id: RR-D5RUEV
type: review-response
title: ChannelService is an 8-method interface; the house rule asks for minimal consumer-side interfaces split by read and write
finding: CLAUDE.md asks for 1-3 methods and read/write capability splits (ReadDeps/WriteDeps). Split it into a reader (Enabled, ListAnchor, Read, Subscribe) and a writer (Post, Edit, Delete, Create), so the watch path holds only the reader.
severity: minor
resolution: Split into ChannelReader (Enabled, ListAnchor, Read, Subscribe) and ChannelWriter (Post, Edit, Delete, Create). The watch path holds only the reader.
status: addressed
---
