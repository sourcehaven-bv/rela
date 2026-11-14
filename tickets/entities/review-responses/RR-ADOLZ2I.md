---
id: RR-ADOLZ2I
type: review-response
title: go test default 10 minute timeout
finding: Seeding 10k entities 12 times on a slow runner may exceed 10 minutes.
severity: minor
resolution: 'Addressed: -timeout 30m.'
status: addressed
---
