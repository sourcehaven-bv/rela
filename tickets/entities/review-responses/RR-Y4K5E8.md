---
id: RR-Y4K5E8
type: review-response
title: 'Naming: ''Reader'' vs ''RawStore'' reads as a quality distinction, not a safety boundary'
finding: The two ReadDeps fields carry the plan's most important invariant, but 'Reader'/'RawStore' doesn't convey WHICH is safe for what — a future reader could reasonably assume RawStore is just the lower-level handle and use it for a read-out. Consider names that encode the boundary in the identifier itself (e.g. VisibleReader / WritePrepStore, or ReadOut / RawForWritePrep) so the wrong choice looks wrong at the call site, not just in the godoc.
severity: nit
resolution: Renamed to VisibleReader (read-out) and WritePrepStore (write-prep), so the safety boundary is encoded in the identifiers — using WritePrepStore for a read-out now looks wrong at the call site, not just in the godoc.
status: addressed
---
