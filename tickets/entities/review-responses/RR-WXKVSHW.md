---
id: RR-WXKVSHW
type: review-response
title: Postgres race step duplicated a hardcoded package list
finding: The postgres race step copied the suite steps' package list and missed cmd/rela-server's TestTenantMode_RevokeRacesLastRelease.
severity: minor
resolution: 'The postgres race step was removed: Postgres Backend keeps -race on every suite, as on develop.'
status: addressed
---
