---
id: RR-8RADMD
type: review-response
title: RenameEntity godoc overstated the dry-run guarantee
finding: The godoc said a dry run emits no audit record, but a denied dry run writes a denied-write record.
severity: nit
resolution: The godoc now says no rename record is written and that a denial still writes its denied-write record.
status: addressed
---
