---
id: RR-8QUG6O
type: review-response
title: Own post does not wake the poster's inbox, so the wake set goes stale
finding: 'Service.Post appends with MarkRead: true (service.go:421-424), so the poster''s marker moves or is created. But it calls only markAddressed(addressed, key) (service.go:438) and never markFor. Today markFor is called only from MarkRead (reads.go:79-81). Case: Alice posts first in a non-waiting channel. That creates her marker and puts the channel in her read window, but her inbox subscription is not selected (the key is not in her old WakeKeys, the type is not waiting, she is not addressed), so the wake set never learns the key. Bob then posts there. Alice''s unread_channels and badge dot should move, but no frame is sent. Opening the channel does not repair this: her marker already sits at her own message, so MarkRead reports moved=false. With no safety replay, the badge stays wrong until some graph write starts a full round. The same can happen when a first MarkRead on an empty channel creates a marker at 0 without reporting moved. Fix: Post (and Edit, if it moves the marker) calls s.local.markFor(reader, c.key) when the marker moved, or the hub treats a key change authored by w.user as a wake. Add an AC and a test: Alice posts first in a non-waiting channel, then Bob posts, and Alice''s inbox and summary frame.'
severity: significant
resolution: Post now calls markFor for the caller after Append with MarkRead, MarkRead calls markFor after every successful SetRead, Edit and CreateMany are checked, and AC 10 tests the own-post-then-Bob-posts case and the first mark read on an empty channel.
status: addressed
---
