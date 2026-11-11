---
id: TKT-QFJ6LQN
type: ticket
title: Headless rela-server build for sqlite and postgres in releases
kind: enhancement
priority: medium
effort: m
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

## Description

Ship a headless `rela-server` for the sqlite and postgres backends. It serves
the REST API (`/api/v1`), SSE events and the remote MCP endpoint, and embeds no
web UI: no Vue SPA bundle and no `<rela-editor>` custom-app bundle. It builds
without running the frontend build.

Use case: API/MCP-only deployments (integrations, agents, a separately hosted
UI) that do not want to ship or expose the SPA.

## Scope

In scope:

- A `headless` build tag. Under it `internal/dataentry` embeds no `static/`
and no `app_editor_dist/`.
- In a headless build, `/` answers a small 200 text body that points to
`/api/v1` and carries the existing OpenAPI `Link` header. Other non-API paths
and `/static/` answer 404. `CheckEmbeddedSPA` passes.
- In tenant mode, `StaticShell` answers 404 for tenant-free requests.
- Release: `rela-server-headless-sqlite` and `rela-server-headless-postgres`
for linux/darwin/windows × amd64/arm64, in separate archives
`rela-headless-sqlite_*` and `rela-headless-postgres_*`.
- `just` recipes to build both locally; CI compiles and tests the
`headless` tag.
- Docs: mention the headless variants where the release archives are listed.

Out of scope:

- Removing API routes that only the SPA uses (collab websocket, custom apps).
They stay; custom apps lose the editor asset (`_rela-editor.js` 404s, as on a
checkout without a frontend build).
- A headless filesystem-backend build, `rela` CLI changes, Docker images.

## Acceptance criteria

1. `go build -tags sqlite,headless ./cmd/rela-server` and
`-tags postgres,headless` succeed on a clean checkout with no frontend build;
the binaries contain no SPA assets (`index.html` absent, smaller than the full
build).
2. A headless server answers `/api/v1/_schema` normally, `/` with 200 text
and the `Link` header, `/assets/x.js` and `/static/favicon.svg` with 404.
3. A non-headless build behaves as today (existing router tests pass).
4. `goreleaser check` passes, and a snapshot build produces both headless
archives.
5. CI runs the headless-tagged tests.
