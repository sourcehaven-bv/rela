---
id: DOCS-VWM0Y8
type: docs-checklist
title: 'Docs: rela CLI sync client'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package + exported symbols have godoc (internal/cli/sync/*)
- [x] Non-obvious logic explained (topo order, cursor advance, dedupe invariant, idempotent resume)

## Project Documentation

- [x] CLI reference updated (`docs/cli-reference.md` § rela sync — replaced the stale cache-rebuild stub)
- [x] Dedicated feature doc added (`docs/sync.md` — model, push, pull, conflicts, proxy deployment, limitations)

## External Documentation

- [x] ~~API reference~~ (N/A: the /api/sync/ server API was documented under TKT-PV0R3V; this ticket is the client)
- [x] Operator/deployment guidance (proxy `--skip-jwt-bearer-tokens`, `--bearer-token-login-fallback=false`, aud/iss alignment) in `docs/sync.md § Deployment behind an OAuth proxy`
