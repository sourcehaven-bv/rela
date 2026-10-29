---
id: RR-J6W8N4
type: review-response
title: Use the codebase's omitzero idiom for _busy
finding: Busy *[]BusyChannel with omitempty works, but InboxView uses plain slices with omitzero to tell nil (absent) from empty (sent as []) (internal/realtime/inbox.go:166-169). []BusyChannel with json:"_busy,omitzero" says the same thing without a pointer to a slice.
severity: nit
resolution: Activity.Busy is now a plain []BusyChannel with json:"_busy,omitzero", as InboxView does.
status: addressed
---
