---
id: RR-3FFBJM1
type: review-response
title: Wire-format comment attached to TreeScalar only
finding: In api/configure.ts the moved file-header comment sits directly above TreeScalar, so editors show the $m/$i encoding rules on TreeScalar and nothing on TreeMap or TreeValue.
severity: minor
resolution: TreeMap now carries the $m/$i paragraph and TreeValue a one-line description with a pointer to configure/tree.ts; TreeScalar has no comment.
status: addressed
---
