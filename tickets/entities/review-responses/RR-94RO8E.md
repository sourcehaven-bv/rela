---
id: RR-94RO8E
type: review-response
title: Store.SetSession godoc says the due time is not shown to readers
finding: 'internal/channels/store.go:53-54: ''a reschedule does not [raise rev], because the due time is not shown to readers.'' due_at is now on every read. The reason it is still correct is that the wire derives due_at from the config and the row, and never reads the stored hint NextDueAt. Reword it to say exactly that, or the next person will ''fix'' the missing rev bump.'
severity: nit
resolution: SetSession godoc now says due_at is derived from config and row and NextDueAt is only an index hint.
status: addressed
---
