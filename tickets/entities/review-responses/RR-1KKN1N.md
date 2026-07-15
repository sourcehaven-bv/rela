---
id: RR-1KKN1N
type: review-response
title: 'Relax BOTH date: and end_date: gates; mismatch error must name both props+types'
finding: 'validate_feeds.go gates at :89 (date) and :127 (end_date) both reject datetime (!= PropertyTypeDate). Must relax BOTH — relaxing only date: leaves a datetime end_date rejected (half-built). The new start/end same-kind mismatch error must name both properties and both types in the existing fmt.Sprintf(''%s: ...'', prefix, ...) style, not a vague ''type mismatch''. date is required (verified :85-91) so end_date-without-date can''t occur.'
severity: significant
resolution: 'Adopted. Add an isFeedDateType(t) helper accepting date OR datetime; use at both :89 and :127. Add a same-kind check: if both date: and end_date: set and their types differ, error naming both props + both types, matching the prefix style.'
status: addressed
---
