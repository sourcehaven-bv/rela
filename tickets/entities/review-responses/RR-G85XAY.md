---
id: RR-G85XAY
type: review-response
title: 'Design: delete does not fence a lineage for tags'
finding: Delete then recreate (or rename onto a deleted id) would inherit old tags and allow duplicate names.
severity: critical
resolution: Tags resolve only after the newest delete row; a move deletes same-name rows in the whole lineage; VersionByTag picks the newest (R2).
status: addressed
---
