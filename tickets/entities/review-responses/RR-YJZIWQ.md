---
id: RR-YJZIWQ
type: review-response
title: Busy guide omits that moves with graph outputs refresh live
finding: 'docs-project/entities/guides/GUIDE-realtime.md:1971-1975 says a session move on its own does not update an open board. The plan''s "Cross-process limits" section says a move with graph outputs (every PoC move) starts a full live round, which replays the board. Readers will think the PoC board never updates live. Fix: add one sentence that a move whose transition has outputs updates the board live, because the output is a graph write. Say the same in the backend table.'
severity: minor
resolution: The busy guide now says a move with graph outputs updates an open board live, a move without outputs does not, and the time moves each minute.
status: addressed
---
