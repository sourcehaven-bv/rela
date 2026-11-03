---
id: RR-TYOC63
type: review-response
title: Vale runtime error loses its message
finding: Claimed Vale writes runtime errors to stdout and stderr stays empty.
severity: minor
resolution: 'Checked against Vale 3.25.1: a bad --config prints the E100 JSON on stderr, which the error already includes.'
reason: 'Checked against Vale 3.25.1: a bad --config prints the E100 JSON on stderr, which the error already includes.'
status: wont-fix
---
