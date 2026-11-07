---
id: TKT-VP02WHG
type: ticket
title: Reconcile endpoint POST /api/v1/_reconcile
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Add `POST /api/v1/_reconcile` (FEAT-CMVZQFI).

- The request names a scope (types, or a view name) and lists ranges with the client fingerprints.
- For each range the server answers `match`, `split` (sub-ranges with fingerprints) or `items` (the elements).
- Stateless; bound to the request identity like every other read; reads nothing the caller could not list.
- Limits on ranges per request and items per answer, and a rate limit.
- OpenAPI entry and handler tests, including ACL tests (hidden rows never appear in a fingerprint or item list).

## Acceptance

- Two principals with different read access get different fingerprints for the same range.
- The endpoint passes the security and router tests.
