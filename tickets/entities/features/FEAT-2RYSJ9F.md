---
id: FEAT-2RYSJ9F
type: feature
title: Trustworthy, readable audit log (GRC gap 11)
summary: 'Audit log as a service: pluggable writers, a tamper-evident SQL implementation on postgres and sqlite, and a gated read API in data-entry'
description: Auditors ask who changed what and whether the log itself can be trusted. The audit log becomes a service with writer and reader capabilities; the SQL implementation records each write in the same transaction as the write, chains rows by hash, and serves reads to data-entry under a permission.
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
