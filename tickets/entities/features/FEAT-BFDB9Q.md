---
id: FEAT-BFDB9Q
type: feature
title: 'Custom apps: user-authored sandboxed-HTML extensions for the data-entry web app'
summary: Per-project user-authored HTML+JS "apps" served in a sandboxed iframe inside the data-entry SPA, talking to the existing ACL-gated REST API via a MessageChannel bridge.
description: Per-project user-authored single-file HTML+JS apps served in a locked-down sandboxed iframe inside the data-entry SPA, talking to the existing ACL-gated REST API via a MessageChannel bridge. Apps stored as files under apps/, declared in data-entry.yaml; reads via /api/v1/* (readGate-scoped), writes via existing CRUD endpoints + registered Lua actions. Modeled on Datasette Apps; design in RES-HZ9MMR.
priority: medium
status: proposed
---
Lets users extend the data-entry web app with custom single-file HTML+JS
applications (dashboards, specialized forms, domain mini-tools) without forking
the Vue SPA or shipping Go code. Modeled on Simon Willison's Datasette Apps.

Apps are stored as files under a project `apps/` directory, rendered in a
locked-down `<iframe sandbox>` on a `/app/:id` SPA route, and communicate with
the host via a `MessageChannel` bridge that exposes **only** the existing
ACL-gated REST API — reads via `/api/v1/*` (scoped by `readGate` +
`VisibleSearcher`), writes via the existing CRUD endpoints (entitymanager
re-authorizes + audits) and registered Lua actions via `POST /_action/{id}`.

Design recorded in RES-HZ9MMR.
