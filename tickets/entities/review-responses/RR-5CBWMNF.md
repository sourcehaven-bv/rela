---
id: RR-5CBWMNF
type: review-response
title: Title column rule lives in two places and a doc comment is stale
finding: nameColumn returned columns[0] while the link rule lived only in EntityList.
severity: minor
resolution: nameColumn now picks the link column else the first and is the default in toTableColumns. Comments updated and tests added.
status: addressed
---
