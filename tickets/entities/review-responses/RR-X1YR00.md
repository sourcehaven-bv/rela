---
id: RR-X1YR00
type: review-response
title: ErrClosed from dispatch makes neoq log and requeue after Close
finding: 'A dispatch refused after close returned plain ErrClosed. The memory worker treats that as a failed job: it logs at Error and requeues, possibly after Close returned. On postgres it can record an attempt for a job that never ran.'
severity: significant
resolution: dispatch now returns ErrClosed wrapped with context.Canceled, so the memory worker stops quietly as it does on cancellation.
status: addressed
---
