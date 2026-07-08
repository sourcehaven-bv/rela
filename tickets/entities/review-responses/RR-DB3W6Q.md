---
id: RR-DB3W6Q
type: review-response
title: included merge semantics (later page wins) were undocumented
finding: data had an explicit 'later page wins' dedupe comment; included silently did the same via Object.assign but the doc comment only said 'included maps are merged', leaving the two merges' semantics visibly unaligned for the next reader.
severity: nit
resolution: 'Doc comment updated: ''included maps are merged (later page wins, same rule as data)''.'
status: addressed
---
