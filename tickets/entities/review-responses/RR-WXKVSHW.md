---
id: RR-WXKVSHW
type: review-response
title: Postgres race step duplicated a hardcoded package list
finding: The postgres race step copied the suite steps' package list and missed cmd/rela-server's TestTenantMode_RevokeRacesLastRelease.
severity: minor
resolution: Both jobs call the shared script; the postgres call adds cmd/rela-server, internal/lease and internal/piles.
status: addressed
---
