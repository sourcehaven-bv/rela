---
id: RR-YYH467
type: review-response
title: Mark read on a mention row has side effects the plan does not state
finding: PUT _read with the mention's seq sets marker = max(marker, seq). When Alice had no marker on that channel, this creates one, and the channel enters her read window and later shows in Unread channels. It also marks every earlier message in that channel as read. That may be acceptable, but AC 20 and the data-entry docs should say so, and the test should cover the case of a mention in a channel without a marker.
severity: minor
resolution: AC 24 and the data-entry docs now state that marking a mention read marks earlier messages read and can create a marker, and TestInbox_MentionMarkReadCreatesMarker plus an InboxView case cover a channel without a marker.
status: addressed
---
