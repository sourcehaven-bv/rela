---
id: RR-840DNS4
type: review-response
title: Docs do not say the regex matches part of a value
finding: RE2 is unanchored so niveau=~hoog also matches zeer_hoog. Enum users may expect an exact match.
severity: minor
resolution: 'Added a note under the operator table in docs/metamodel.md and the mirror: partial and case-sensitive match / anchor with ^...$ / a misspelled pattern matches nothing.'
status: addressed
---
