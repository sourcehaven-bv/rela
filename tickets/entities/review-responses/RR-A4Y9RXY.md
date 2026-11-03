---
id: RR-A4Y9RXY
type: review-response
title: Contract run silently reuses a stale server binary
finding: globalSetup reused an existing bin/rela-server-memory when RELA_CONTRACT_SERVER was unset.
severity: minor
resolution: Without RELA_CONTRACT_SERVER it always runs go build, which only redoes what changed.
status: addressed
---
