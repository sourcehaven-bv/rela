---
id: TKT-N1KNNZU
type: ticket
title: Range fingerprints for types with redacted fields
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

For a type where the reader has `visible:` grants, the stored hash is not the
view hash. Hashing the raw row would reveal when a hidden value changes.

- For those types, compute the view hash of the redacted projection in Go, one page of headers at a time, and fold it into the range fingerprint.
- The result is per principal and is never cached across principals.
- Follow `.claude/rules/collection-reads.md`: no per-row lookups, a budget test.

## Acceptance

- A test shows that a change to a hidden field does not change the fingerprint a reader gets.
- A test shows that the redacted and unredacted paths agree when the reader has no hidden fields.
