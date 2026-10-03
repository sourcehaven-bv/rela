---
id: RR-8ZPDBZ
type: review-response
title: Unquoted ref in seqtrace-compare recipe
finding: SEQTRACE_REF='{{ref}}' breaks on a ref containing a quote.
severity: nit
resolution: Uses {{quote(ref)}}.
status: addressed
---
