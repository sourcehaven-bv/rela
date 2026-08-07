---
id: TKT-8P1TM7
type: ticket
title: 'Sync channel (/api/sync/) bypasses visible: redaction — enforce a trusted-replica boundary + round-trip-safe redaction'
kind: enhancement
priority: medium
status: backlog
---

## Problem

The machine-to-machine sync channel (`/api/sync/`, FEAT-NJ9FEN) returns each
record's **full canonical body** — all entity properties and all relation meta —
gated only by the **row-level** read verdict (`permitsSyncReadEntity` /
`permitsSyncReadRelation` in `internal/dataentry/sync_handlers.go`). It does NOT
apply `visible:` field/meta redaction, so a principal with ordinary reader
access can pull `visible:`-redacted values straight off the sync GET, bypassing
the redaction the v1 read endpoints enforce.

Surfaced by the TKT-B1F5Q1 code review (RR-FOD7IB). Applies to both entity
fields (pre-existing since TKT-73C6B2) and relation meta (TKT-B1F5Q1).
Documented as a deferred gap in the "What still leaks (deferred)" section of
`docs/acl-security.md` + the docs-project mirror.

## Why it can't be naively fixed

The sync GET is a **read that feeds a write**: the client hashes the returned
body and pushes it back under `If-Match`. Simply routing the sync GET through
`visibleRelationMeta` / `stripHiddenProperties` would drop the hidden fields
from the body the client re-pushes, **erasing them on the authoritative store**
— the CLAUDE.md "never redact a read that feeds a write" data-destruction bug.
So the leak cannot be closed by redacting in place.

## Fix directions (pick one or combine)

1. **Trusted-replica boundary (operational, cheapest).** The current mitigation
is prose-only: "gate `/api/sync/` behind a network/mTLS/dedicated-sync-principal
boundary." Make it enforceable: a dedicated sync permission (`sync:read` /
`sync:pull`) required on the sync routes, or a config-gated allowlist, so
ordinary reader sessions cannot reach `/api/sync/` at all. Add a test/lint
asserting an un-permissioned principal is 403'd on the sync GET.

2. **Round-trip-safe redaction (complete, larger).** Redact the sync GET body
BUT make the push path merge: on `PUT`, re-read the current hidden fields from
the authoritative store and splice them back before applying, so a redacted
client can round-trip without erasing what it never saw. This closes the leak
for a partially-trusted replica while preserving replication fidelity.

## Origin

TKT-B1F5Q1 review — RR-FOD7IB. Coupled to the entity side (the gap predates
relations; a fix should cover both entity fields and relation meta uniformly).
