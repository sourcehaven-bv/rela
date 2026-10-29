---
id: RR-2UYUYS
type: review-response
title: Syncing list subscriptions miss channel changes that land during their baseline PUT
finding: 'collect() sets recheck on a syncing subscription during a topic round only for presence, channel and inbox subscriptions: s.recheck = s.recheck || changed && (only == nil || s.presenceID != "" || s.channel || s.inbox) (internal/dataentry/live.go:850). The comment says that "a change of a channel moves no other response", which stops being true once a list carries _busy. A session move that lands while a board''s PUT takes its baseline is then lost until the next unrelated change. The plan names live.go only for keeping the probe''s busy type. It should extend the recheck condition to list subscriptions with a busy type (or with recorded anchors), update that comment, and add a test (TestLive_BusyListRecheckWhileSyncing).'
severity: significant
resolution: 'Resolved by the same move: list subscriptions are not woken by channel changes in this ticket, so the `collect` recheck condition and its comment stay correct. The syncing recheck is part of the follow-up''s scope.'
status: addressed
---
