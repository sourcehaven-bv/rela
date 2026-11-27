---
id: RR-ICMRXCW
type: review-response
title: Title fallback compares a partial display title
finding: The deprecated title fallback used DisplayTitle, which builds a partial title when a templated display property is hidden. The reader is shown the id instead (BUG-R9EHKV).
severity: nit
resolution: Use WholeDisplayTitle so the fallback compares the string the reader sees.
status: addressed
---
