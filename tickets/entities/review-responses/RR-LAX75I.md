---
id: RR-LAX75I
type: review-response
title: Baseline could be taken after the client refetch
finding: 'Code review: the PUT answered 204 before the baseline existed. A write between the client refetch and a late baseline was absorbed into the baseline and never reported.'
severity: significant
resolution: The PUT takes the baseline of each added subscription before answering 204 (in a context detached from the PUT's ACL state). Rounds skip a subscription while it syncs. A baseline the PUT could not take is taken by a round with a frame. A feed-position check was tried first and dropped because it framed on hidden writes. Pinned by TestLiveHub_BaselineBeforeAnswer and TestLiveHub_RoundSkipsSyncing.
status: addressed
---
