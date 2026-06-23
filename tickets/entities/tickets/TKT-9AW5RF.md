---
id: TKT-9AW5RF
type: ticket
title: 'Custom apps: sandboxed-HTML extensions served in the data-entry SPA via a REST-API bridge'
kind: enhancement
priority: medium
effort: l
status: done
---

Add a per-project "apps" mechanism to the data-entry web app: user-authored
single-file HTML+JS applications served in a locked-down sandboxed iframe inside
the Vue SPA, talking to the existing ACL-gated REST API through a
`MessageChannel` bridge. Design recorded in RES-HZ9MMR.

## Decided approach

- **Storage (C1):** apps live as files under a project `apps/` directory
(`apps/<id>.html`), declared in `data-entry.yaml` under an `apps:` map, loaded
via `os.OpenRoot` (mirrors `actions/`). Backend-agnostic across fsstore/pgstore
— these dirs are never in the store.
- **Read bridge (A1):** the bridge exposes **only** the existing `/api/v1/*`
REST endpoints (list/get/search/trace/analyze), already scoped per-principal by
`readGate` + `VisibleSearcher`. No query DSL, no Lua on the read path. Future
Lua read functionality is exposed via the **same REST API**, not a separate
bridge.
- **Write path (B1+B2):** writes go through the existing CRUD endpoints
(entitymanager re-authorizes + audits) and registered Lua actions via `POST
/_action/{id}`.
- **Security (D):** `GET /api/v1/_apps/{id}` serves app HTML with a hardened
CSP (`default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline';
img-src data: blob:`) mirroring the theme-logo handler; rendered in `<iframe
sandbox="allow-scripts allow-forms" srcdoc=...>` on a new Vue `/app/:id` route;
`MessageChannel` transport (not raw `postMessage`); default no outbound egress
with an optional CSP origin allow-list; new `OpRunApp` ACL Op (5-point
checklist) gating an app's ability to run; app/bridge routes added to
`sensitivePathPrefixes`.
