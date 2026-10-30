---
id: RR-PGI9NL
type: review-response
title: safetyDue reads poorly and hashes before checking the tick
finding: 'live_inbox.go:183 relies on && binding tighter than ||, and it computes inboxSlice(fp) (an FNV hash, under h.mu in collect) for every subscription with a wake on every topic round, even when now.slices == 0. Fix: if now.slices == 0 { return false }; eligible := (s.inbox && s.wake != nil) || s.busyWake != nil; return eligible && now.slices&(1<<h.inboxSlice(fp)) != 0.'
severity: nit
resolution: 'Fixed: safetyDue returns early when now.slices is 0 and spells out the eligibility with explicit parentheses before it hashes.'
status: addressed
---
