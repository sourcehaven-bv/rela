---
id: RR-0GPIS
type: review-response
title: docCacheDir name is ambiguous
finding: internal/dataentry/document.go:47. const docCacheDir = "documents" reads as a full path but is a subdirectory of .rela/. Rename to docCacheSubdir or similar.
severity: nit
resolution: Renamed const docCacheDir → docCacheSubdir across document.go + tests. Name now reads as a directory name, not a full path.
status: addressed
---

From post-impl cranky review. Readability only; no behavior change.
