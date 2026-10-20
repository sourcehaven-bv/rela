---
id: RR-WJR75J
type: review-response
title: Live refetches can overlap and move the cursor backwards
finding: '§3: each `live:changed` starts a head refetch plus an after_rev loop "while has_more". Frames arrive during a loop (busy channels, the reconnect re-add), so two loops run from the same held rev, and whichever answers last sets `next_after_rev`, which can be lower than the rev already merged. The loop is also unbounded if the server keeps saying has_more. The seq counter in usePresence guards target switches, not this. Fix: single-flight per channel with a dirty flag (a frame during a fetch reruns once after it), held rev = max(held, page.next_after_rev), never set from write answers, and an iteration cap that falls back to a full newest-page reload. AC 8 cases: a frame during an in-flight loop, an out-of-order answer, the cap.'
severity: significant
resolution: useChannel runs one fetch at a time with a dirty flag, keeps the held rev monotonic and never takes it from a write answer, and caps the after_rev loop with a fallback reload (R13, AC 8).
status: addressed
---
