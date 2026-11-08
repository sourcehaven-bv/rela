---
id: RR-M6QBRC1
type: review-response
title: Direct store writers bypass the guard
finding: rela import and data migrations write to the store without entitymanager.
severity: significant
resolution: 'Plan changed: the importer and data migration steps refuse rela-managed types; the docs state that hand edits on fs are outside the guard.'
status: addressed
---
