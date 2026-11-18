---
id: FEAT-0BH46JQ
type: feature
title: Trustworthy, readable audit log (GRC gap 11)
summary: 'Audit log as a service: pluggable writers, a tamper-evident SQL implementation, and a gated read API'
description: Auditors ask who changed what and whether the log itself can be trusted. The audit log becomes a service with writer and reader capabilities.
priority: high
status: planned
---

## Use case

A GRC auditor asks who changed a control, when, and whether the record can be
trusted. Today the log is best-effort JSONL on local disk with no API.

## Scope

- Writer and reader as separate capabilities.
- SQL implementation (postgres, sqlite) in the tenant schema, written in the write's own transaction, hash-chained, with a verify command.
- JSONL stays for the fs backend.
- data-entry: per-entity Changes tab and a global audit view behind `audit:read`.
- Retention and migration of existing JSONL logs.
