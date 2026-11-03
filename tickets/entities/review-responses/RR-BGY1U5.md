---
id: RR-BGY1U5
type: review-response
title: A list_inbox replay that never finishes still reads as "unchanged"
finding: 'internal/mcp/tools_watch.go:498 (`incomplete = incomplete || !inbox`) and paceInbox: an inbox replay that keeps timing out never marks the pass incomplete. The watch then ends at its deadline with the same answer as "nothing moved". An agent cannot tell that its inbox could not be computed, and keeps watching a broken target with backoff 15 s, 30 s, 60 s and so on. Fix: when inboxHeld is still set at return, report the call incomplete, or add a per-target note, so the agent learns that the inbox failed. Add a case to TestWatch_ListInboxSlowReplayDoesNotSpin that asserts this outcome.'
severity: minor
resolution: The watch result has an incomplete field listing targets whose latest replay did not finish; tested for slow and recovered inbox
status: addressed
---
