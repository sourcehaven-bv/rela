---
id: RR-WDAEYZ
type: review-response
title: 'Search structured terms, sort: and _position?scope=search use raw values'
finding: '/_search prop:/status:/sort: and _position?scope=search still filter and sort raw rows on fields hidden by visible:.'
severity: significant
reason: Out of scope for the list fix. BUG-BINAR2 covers /_search structured terms and sort:, and its body now also names the _position?scope=search prev/next path, which resolves through executeQuery. The fix belongs where executeQuery parses the query so both paths share it. BUG-WJSYQT's prevention text now limits its claim to the list endpoint and the paths built on its pipeline and points at BUG-BINAR2.
status: deferred
---
