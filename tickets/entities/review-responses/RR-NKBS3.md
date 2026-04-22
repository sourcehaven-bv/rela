---
id: RR-NKBS3
type: review-response
title: 'Two API clients: ApiClient class and `api` fixture, with different signatures'
finding: crud.spec/data-entry.spec use ApiClient (createEntity(plural, properties)); rest use `api` fixture (createEntity(plural, {properties})). Consolidate.
severity: minor
reason: ApiClient is used by 2 legacy specs with a different signature. Migration is mechanical but broad. Defer to a cleanup ticket.
status: deferred
---
