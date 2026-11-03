---
id: RR-OPOSN24
type: review-response
title: contract-fake overwrites the e2e SPA embed
finding: just contract-fake depends on build-frontend, which replaces a build:e2e embed.
severity: nit
reason: Every server recipe rebuilds the embed it needs; e2e recipes rebuild theirs too.
status: wont-fix
---
