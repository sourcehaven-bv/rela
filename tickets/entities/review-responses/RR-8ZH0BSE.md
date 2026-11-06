---
id: RR-8ZH0BSE
type: review-response
title: Misspelled validation key silently gives advisory
finding: 'config.yaml is decoded leniently, so validaton: strict is ignored.'
severity: minor
reason: config.yaml is shared by several features and decoded leniently on purpose; strict decoding is a separate change. The resolved mode is now logged at boot.
status: deferred
---
