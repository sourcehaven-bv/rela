---
id: RR-9PXHMIA
type: review-response
title: Soft-delete GC purges on the wall clock while memstore stamps on the world clock
finding: startSoftDeleteGC used time.Now while softdelete stamps use m.clock(); a pinned past clock purged at once and a future one never.
severity: significant
resolution: startSoftDeleteGC takes the world clock; softdeletegc.go and scheduled_mail.go are now in the wall-clock guard roots.
status: addressed
---
