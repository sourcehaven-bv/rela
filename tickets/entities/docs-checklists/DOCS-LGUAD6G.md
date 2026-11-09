---
id: DOCS-LGUAD6G
type: docs-checklist
title: 'Docs: Reconcile endpoint POST /api/v1/_reconcile'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the reconcileHandler threat model, the after+"\x01" successor, why the slot is released before the write, the gate's refund and eviction rules, the CSRF exemption entry, the arch-lint edges
- [x] Function/type docs if public API: SetReconcile, appbuild.Elements, the v1 Reconcile wire types, Fingerprint and Digest text codecs

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the endpoint is documented in the API reference, which the README already links)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing _batch and visibility wrapper patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes; the client is step 6)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): docs/data-entry/api-reference.md gains "Reconcile (/_reconcile)"; docs/acl-security.md (via GUIDE-acl-security) gains "Reconcile" with the threat model; the OpenAPI spec documents the route
