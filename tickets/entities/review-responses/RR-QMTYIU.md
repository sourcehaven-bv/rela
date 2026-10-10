---
id: RR-QMTYIU
type: review-response
title: api connect may prompt
finding: connect can prompt for config or auth; with empty stdin a future fixture with a security scheme would behave unpredictably.
severity: minor
resolution: Pass --yes to api connect.
status: addressed
---
