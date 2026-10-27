---
id: RR-DO014K
type: review-response
title: Inbox watch matches by channel type, so any chat of a common type wakes every inbox watcher
finding: 'WakeTypes holds the types of every channel in the caller''s window, and watchChannel.matches only compares types. Most configurations have one to three channel types, so a post anywhere in the project wakes every list_inbox watch whose window touches that type. Each wake costs a full inbox replay (holder queries, ListReads, the fs tree walk). This also contradicts AC13 ("does not wake for a post in a channel where the caller holds no marker"): the test passes only because the fixture has no marker on another channel of the same type. Fix: return WakeKeys (the window''s channel keys, at most 500) beside the waiting types. Match unread wakes by exact key and waiting wakes by type. Add a test with a marker on channel X of type T and a post in channel Y of type T: no wake.'
severity: significant
resolution: 'Inbox returns WakeKeys (gated window keys, max 500); watch wakes chat by exact key, sessions by waiting type. TestWatch_ListInboxNoWakeOnChat: marker on X of type T, post in Y of type T, no wake.'
status: addressed
---
