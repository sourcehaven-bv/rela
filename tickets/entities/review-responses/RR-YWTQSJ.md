---
id: RR-YWTQSJ
type: review-response
title: Edge batch did redaction and priming it discarded
finding: readableTypes went through ResolveHeadersChecked, which served faces, primed traversals and redacted headers that the caller threw away.
severity: significant
resolution: 'Replaced with Resolver.ReadableTypes: scanHeaders only, with no world, serving or redaction.'
status: addressed
---
