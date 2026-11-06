---
id: RR-QEQ6711
type: review-response
title: Joiner's fresh token lets room content overwrite an external edit
finding: 'Plan: each client''s autosave base is its own GET. If the room outlives a store write by another path, or a client reconnects to an evicted room with its old document, the saver''s precondition is the fresh token and the room''s older content silently overwrites the external edit, or old and reseeded content duplicate. Plan change: the room carries its save base (content and token) in a shared Y.Map; savers precondition on it so an external edit 412s into the merge path. RoomIdleTimeout 2 minutes so a short reconnect finds its room; a client disconnected longer than that flushes and rebuilds its document from a fresh room.'
severity: significant
resolution: 'Addressed in plan: room-held save base (body and token), 2 minute room idle timeout, rebuild after a longer disconnect.'
status: addressed
---
