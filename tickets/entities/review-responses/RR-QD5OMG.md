---
id: RR-QD5OMG
type: review-response
title: 'Design: ordinal sentinel and races'
finding: version 0 = current is a magic value; ordinals renumber on purge.
severity: minor
resolution: Split TagCurrent/TagVersion; ordinal resolved under the purge lock (R9).
status: addressed
---
