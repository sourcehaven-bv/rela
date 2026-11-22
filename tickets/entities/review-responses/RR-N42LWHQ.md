---
id: RR-N42LWHQ
type: review-response
title: Superset path not exercised on sqlite
finding: 'Code review: the sqlite differential covered exact conditions only.'
severity: minor
resolution: assertLoweredMatchesGo also runs three date conditions narrowed by a Where filter, on memstore, fsstore and sqlite.
status: addressed
---
