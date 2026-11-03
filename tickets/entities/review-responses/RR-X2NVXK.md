---
id: RR-X2NVXK
type: review-response
title: reminders.go presence test is not in the files list
finding: internal/realtimecfg/reminders.go:236 does `c.states[sn].On[raw.Type]` as a presence test, like message_in.go:82. It still compiles and behaves correctly after the type change, but the plan lists only message_in.go. Add it so the reviewer knows it was considered.
severity: nit
resolution: Added reminders.go to the raw-shape step and the files list as a type-only change.
status: addressed
---
