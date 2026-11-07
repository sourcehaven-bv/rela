---
id: TKT-0Y566OJ
type: ticket
title: Reconcile client and rela reconcile diff command
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Add a Go client for `_reconcile` and a CLI command:

`rela reconcile <url> --scope ... [--token-env ...]`

It runs range-based reconciliation against a local project and prints what was
added, changed and gone. It changes nothing (diff only). Content for changed
rows is fetched in batches through the existing list and GET endpoints.

Useful on its own: compare two instances, check a backup, check an `fsimport`
result.

## Acceptance

- An end-to-end test against a test server shows correct added, changed and gone sets.
- Docs for the command in `docs-project/`.
