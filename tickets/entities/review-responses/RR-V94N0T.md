---
id: RR-V94N0T
type: review-response
title: Late-kept rows (choices, decision cause) are never announced; AC 8 test swapped to dodge it
finding: 'RlChannel.vue:306-318 filters added messages by shownAs(...)===''full'' at the moment the messages watch fires, but choices and decisionSeq come from the channel head, which arrives after the frame (the notes say so). A hidden question or hidden decision cause that must be acted on is therefore filtered out, and when keptNow (RlChannel.vue:203) later adds it, nothing announces it. Only the type-action reason works, and the panel test ''kept quiet row is announced'' (ChannelsPanel.test.ts:1965) was changed to use exactly that reason. The docs (''A screen reader announces a new message only when it is shown in full'') are false for these rows. AC 8 lists kept rows without restriction. Fix: in announce, record seqs that were withheld only for being quiet in a per-mount ''silenced'' set; in the keptNow watcher, announce members of keptNow that are in ''silenced'' (with channelNewMessages) and drop them from the set. Restore the planned case: a live hidden ask frame followed by a head with choices must be announced.'
severity: significant
resolution: 'Fixed. RlChannel keeps a per-mount set of new messages that were withheld only for being quiet (pruned to held seqs and cleared when Show all is pressed). The keptNow watcher announces members that become kept with channelNewMessages and drops them from the set. Restored the planned case as panel test "hidden question announced once its choices arrive": a live hidden ask frame, then the head with choices, announces "Question from robot". It fails without the fix. The header comment and the data-entry guide now say a row kept later is announced then.'
status: addressed
---
