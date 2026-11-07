---
id: RR-KIPJ213
type: review-response
title: View-building rule private to dataentry
finding: entityView decides what the reader's view is; a later sync listing in another package may copy it and drift.
severity: minor
resolution: The canonical.Digest doc and the reconcile package doc say to build the view from a visibility-redacted row and not to write a second redaction.
status: addressed
---
