---
id: RR-ATZDJ33
type: review-response
title: Names and package boundaries
finding: Feed and FeedSource collide with changefeed and pgstore/feed.go; NotifyChannels puts a channel concept into pgstore.
severity: nit
resolution: Renamed to HintSender and HintSource; pgstore exposes a generic Notify and Listen limited to a fixed list of channel names.
status: addressed
---
