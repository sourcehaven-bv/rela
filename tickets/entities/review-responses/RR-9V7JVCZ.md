---
id: RR-9V7JVCZ
type: review-response
title: Follower resynced before the source was positioned
finding: The follower resynced at start while the source goroutine was still taking its position (file tail start offset, pg registration and first LISTEN). A write in between was missed by the resync and its hint was never delivered. The relay also signalled its swap gap before the new endpoint was positioned.
severity: significant
resolution: HintSource now calls gap once it is positioned. The file feed calls it after start; pgstore.Listen calls it at once when LISTEN runs and otherwise on the first LISTEN (setListening gaps on every LISTEN). The follower makes its start resync on that first gap; the relay no longer sends its own gap. TestHints_StartResyncWaitsForSource and the notify tests (one send after the first gap, no retry) pin it.
status: addressed
---
