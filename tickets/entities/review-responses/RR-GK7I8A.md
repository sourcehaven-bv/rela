---
id: RR-GK7I8A
type: review-response
title: Per-owner pile cap races on create
finding: SELECT FOR UPDATE on a pile row does not serialize two concurrent CreatePile calls for one owner.
severity: minor
resolution: 'Plan: pgpiles takes pg_advisory_xact_lock(hashtext(owner)) in create and add. pilestest adds a concurrent-create case.'
status: addressed
---
