---
id: RR-XJDM4M
type: review-response
title: Roles order and validation error kind
finding: InboxEntry.Roles is documented as "config order", but ct.Roles is a map. Use st.WaitingFor() order. CheckDueQuery uses ErrInvalidKey for an empty or invalid type list. CheckWaitingQuery should follow that convention instead of ErrInvalidMessage.
severity: nit
resolution: Roles follow st.WaitingFor() order. CheckWaitingQuery uses ErrInvalidKey, as CheckDueQuery (confirmed in session.go:521).
status: addressed
---
