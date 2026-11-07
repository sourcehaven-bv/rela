---
id: TKT-H91N91L
type: ticket
title: Peer identity and credentials
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A peer instance needs an identity on the source instance.

- A `system:<peer>` principal with a token, using the existing token store and identity edge.
- Outbound credentials for a peer in `.rela/secrets.yaml`, never in config files.
- Roles for the peer principal come from `acl.yaml` as for any principal.

## Acceptance

- A peer token reads only what its role allows, through `_reconcile` and the list endpoints.
- No token value appears in logs, config or the audit log.
