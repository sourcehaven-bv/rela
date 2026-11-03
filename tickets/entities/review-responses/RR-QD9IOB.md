---
id: RR-QD9IOB
type: review-response
title: The replacement for TestLive_BusyListIgnoresChannelRound should keep a move that does not change _busy
finding: 'The old test posts `go` (new to queued, which is not a working state) and then `claim` (live_test.go:827-831). Under the new design, `go` is moved: it is the first post (Seq 1) and it moves the session. So it selects the board, the replay runs and the hash stays put. Keep that step in TestLive_BusyListWakesOnSessionMove as a "move that changes nothing" subtest: one replay, no frame. It proves that the frame comes from the hash and not from the selection, which the Security section relies on. Keep the run channel''s requireChanged as the barrier, and delete the old test in the same commit, since it asserts the opposite.'
severity: minor
resolution: 'Adopted: AC 4 keeps the go post as a "move that changes nothing" subtest with one replay and no frame, uses the run channel frame as the barrier, and deletes the old test in the same commit.'
status: addressed
---
