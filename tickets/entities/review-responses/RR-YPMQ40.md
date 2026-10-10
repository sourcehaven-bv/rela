---
id: RR-YPMQ40
type: review-response
title: 'Nits: Go default label, duplicate ids, strict flag parsing'
finding: Go DisplayLabel returned an empty label for a comments field; relation counts did not dedupe ids; comment_counts=yes is silently false.
severity: nit
resolution: DisplayLabel now returns 'comments'. Relation counts use new Set(ids).size, pinned by a test. The lenient flag parse is kept to match include_content.
status: addressed
---
