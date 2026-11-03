---
id: RR-H4E5X9H
type: review-response
title: Value attribution cannot judge body writes
finding: The body is sent whole, so a blind write-ours that restores a base value into a slot the other client changed has no owner and criterion 2 checks nothing. Attribute each request to the client that sent it (AsyncLocalStorage in runWrite).
severity: critical
resolution: Each tab's runWrite runs its save inside AsyncLocalStorage.run(tab); the adapter fault hook reads the sender of every request, including the conflict refetch. Writes are judged per sender; the mergeText write-ours fault is caught (test 'catches a body merge that always writes ours').
status: addressed
---
