---
id: TKT-L0K1GHQ
type: ticket
title: Reconciliation element and reader view hash
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Define the reconciliation element for FEAT-CMVZQFI and the hash it carries.

- An element is the pair (address, view hash). The address is `type/id@face` for an entity and `from--type--to` for a relation.
- The view hash is the canonical hash of the row as the reader sees it. It must use the same function as the entity and relation ETag (BUG-J01V0Q), so the two can never disagree and a `visible:`-hidden value never shows up in it.
- A relation is an element only when both endpoints are readable and in scope.
- Define a fixed-width element fingerprint (a hash of address plus view hash) that can be summed modulo 2^N, and the additive range fingerprint `{count, sum}`.

Put this in a small leaf package with no store or HTTP imports. See RES-G8CYB1R.

## Acceptance

- One function produces the view hash, and the ETag code uses it.
- Unit tests pin the fingerprint and show that a redacted field does not change the view hash.
