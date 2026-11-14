---
id: RR-6JJ259C
type: review-response
title: HighestID parsing and case differ per backend
finding: HighestID parses a leading digit run on fs/mem/pg and strict digits on sqlite; fs/mem/pg compare the prefix case-sensitively, so a mixed-case id is not seen.
severity: minor
reason: Needs a store contract change with storetest pins on all four backends; ids are minted upper-case, so this only affects hand-written mixed-case ids.
status: deferred
---
