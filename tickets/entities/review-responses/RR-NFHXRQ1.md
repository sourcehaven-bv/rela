---
id: RR-NFHXRQ1
type: review-response
title: Explicit -race duplicates GOFLAGS on push
finding: Two wiring-step lines pass -race while GOFLAGS also sets it on push.
severity: nit
reason: 'Harmless: cmd/go accepts the bool flag twice, and the explicit flag keeps those tests racing on PRs.'
status: wont-fix
---
