---
id: DEC-7B6DNZQ
type: decision
title: Record types take their owner's access rules and classification
context: Record types (TKT-J7Q9C4K) store rows owned by an entity. A policy rule requires new data to have read and write rules matching its classification. Records could get their own grants per type or follow the owner entity's ACL.
consequences: Records are as visible as their owner and no more. A confidential record type needs a confidential owner type. GUIDE-records states this. Per-type grants stay a possible narrowing addition in TKT-EV8U8T4.
date: "2026-11-14"
status: accepted
---

## Decision

A record type has no access rules of its own. Its rows are read by whoever can
read the owner entity, and appended by whoever can update every face of the
owner. A record type therefore carries its owner type's classification.

An operator who needs records with a stricter classification than the owner,
such as a salary ledger on a person, declares a separate owner type with the
stricter rules (for example `salary-account`, related to the person) and hangs
the record type on that.

## Why

- One gate, one place to audit: the owner's row gate already answers both
questions, through the same `acl.yaml` the operator maintains.
- Per-type grants (`records:read`, `records:append`) remain possible later
(TKT-EV8U8T4) as an additional, narrowing allowlist. They are not needed to keep
records as restricted as their owner.
