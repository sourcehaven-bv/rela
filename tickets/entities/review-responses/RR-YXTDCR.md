---
id: RR-YXTDCR
type: review-response
title: Typing and signal marks replay every inbox in the process
finding: 'Local.mark is the one change channel for durable writes and for ephemeral ones: typing start (typing.go:75), typing and signal expiry sweeps (service.go:588, Local.expire), and signals. The plan''s selection replays every inbox subscription whose wake set holds the changed key''s type. So typing in any waiting-type channel (the session channels, which are the busiest) replays every connected user''s summary and inbox, as often as the 2 s gap allows. The risk section talks about chat, not keystrokes. On the file store each replay walks the channel tree at least twice (ListReads reads.go:307, ListAddressed addressed.go:255) plus the creator-branch walk in ListWaiting. Fix: tag ephemeral marks in Local (for example a separate set, or a flag Take returns) and let inbox selection ignore them, since typing and signals never change an inbox answer. Add a test that typing in a waiting-type channel does not replay an inbox subscription.'
severity: significant
resolution: Local gains markEphemeral and a durable key set (TakeDurable) used by typing, signals and their expiry, inbox selection reads only durable keys, and AC 13 tests that typing replays no inbox while the head still frames.
status: addressed
---
