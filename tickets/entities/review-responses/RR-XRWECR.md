---
id: RR-XRWECR
type: review-response
title: Use slices.Concat instead of append(globals, ...)
finding: 'fieldfilter.go: append(globals, ...) may write into the backing array of globals.'
severity: nit
resolution: Uses slices.Concat(globals, r.conferrableRoles), which always allocates a fresh slice.
status: addressed
---
