---
id: RR-FWO810
type: review-response
title: Per-comment goldmark parses on read path
finding: resolveAnchor resolves per comment and Acceptable again; adding per-comment parses for blocks and segments costs ~100 parses per list read on a 100 KB body with 50 comments.
severity: significant
resolution: 'Plan revised: textanchor.NewDocument and quotefind.NewDocument prepare once; rela builds one Body per entity read used by ResolveText/ApplyReplacement/Acceptable; benchmark 50 anchors on 100 KB.'
status: addressed
---
