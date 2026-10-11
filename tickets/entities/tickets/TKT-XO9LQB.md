---
id: TKT-XO9LQB
type: ticket
title: restish discovers rela's spec via Link service-desc; CI e2e with real restish
kind: enhancement
priority: medium
effort: s
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

Follow-up to TKT-3DLP0K. restish cannot find rela's spec on its own: it looks
for `Link` headers on a GET of the base URL and for `/openapi.json` under it,
and rela serves the spec at `/api/v1/_openapi.json`. Users must configure
`spec_files` by hand. Nothing in CI runs restish, so a change that breaks the
spec for real clients is only caught manually.

## Scope

- `Link: </api/v1/_openapi.json>; rel="service-desc"` (RFC 8631) on the
response to `GET /`, so `restish api connect <origin>` discovers the spec.
- A Go test that drives a real restish binary against an in-process
rela-server: discovery through the Link header, then upload a file with the
generated `put-<type>-attachment` command, download it and compare bytes. Skips
without restish locally; a CI job installs a pinned, checksum-verified restish
2.3.0 and requires it.
- docs/restish.md: drop the `spec_files` step.

Out of scope: OAuth in CI (needs an IdP), serving the spec at `/openapi.json`.

## Acceptance

1. `GET /` carries the Link header; a unit test pins it.
2. The restish test passes in CI with restish installed and fails if it is missing there.
3. The guide's setup works without `spec_files`.
