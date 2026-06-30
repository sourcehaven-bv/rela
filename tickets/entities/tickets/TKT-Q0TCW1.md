---
id: TKT-Q0TCW1
type: ticket
title: 'Docs: convert security.md to generated GUIDE-server-security; correct ACL read-side coverage claims'
kind: docs
priority: medium
effort: s
status: done
---

The hand-written `docs/security.md` claimed property-level `visible:` redaction
only applied to the **write-form** response. That is stale: the data-entry
serializer already redacts on every REST read (per-entity GET, list rows,
`?include=` peers, `/_search`).

This ticket:

- Converts `security.md` into the generated docs pipeline as
`GUIDE-server-security` (`docs/server-security.md`), guarded by `docs-check`.
- Corrects the ACL coverage list: entity-level read filtering, property
redaction, and groups + containment inheritance are shipped. The remaining gaps
are MCP reads (no gate/redaction, local-only) and the search
match-on-hidden-field oracle (`/_search` redacts the body but the index still
matches hidden text).
- Adds a property-redaction section + the search-oracle/MCP residuals to
`GUIDE-acl-security`.
- Repoints inbound links (audit-log, sync, api-reference) and user-facing
code/godoc pointers to `server-security.md`.

PR: #1059. Background analysis: RES-H5AB7S. The search-oracle is the recommended
follow-up ticket.
