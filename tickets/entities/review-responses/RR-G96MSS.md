---
id: RR-G96MSS
type: review-response
title: Release window hard-coded as 15 minutes in OpenAPI and docs
finding: 'internal/openapi/channels.go:226 and docs/realtime.md:241,386 repeat realtimelimits.PendingRelease as a literal. Fix: build the OpenAPI text from the constant and state the value once in the docs.'
severity: nit
resolution: OpenAPI builds the release window from realtimelimits.PendingRelease with a test that the description quotes it; the guide states 15 minutes once and names it the release window elsewhere (f2c103e67)
status: addressed
---
