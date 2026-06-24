---
id: TKT-THG2JL
type: ticket
title: PlantUML diagram rendering in data-entry (remote server)
kind: enhancement
priority: medium
effort: m
status: review
---

## Description

Add PlantUML diagram rendering to the data-entry UI, mirroring the existing
client-side mermaid integration. PlantUML has no pure-browser renderer (it is a
Java program), so rendering is delegated to a **remote PlantUML server**
configured per-deployment. Presence of the URL is the on/off switch —
**empty/absent = disabled**, blocks degrade to plain fenced code.

## Config & wiring

1. **Server config**: add `PlantUMLServerURL string` to `AppConfig` (`internal/dataentryconfig/config.go`, yaml `plantuml_server_url`), beside `MaxAttachmentBytes`.
2. **Publish to SPA** via existing `/api/v1/_config`: add field to `V1AppConfig` (`internal/dataentry/api_v1.go:253`) and set it in `handleV1Config` (~`:1786`). No new endpoint.
3. **Frontend type**: add to `AppConfig` in `frontend/src/types/config.ts`; already ingested reactively into `schemaStore.app` (no store change).

## Rendering surfaces (two)

- **Client-rendered** (entity body, sections, property markdown via `marked`): handled purely in JS. Add `renderPlantUMLDiagrams(container)` to `frontend/src/utils/markdown.ts` mirroring `renderMermaidDiagrams`: find `pre > code.language-plantuml`, deflate+base64-encode the source (PlantUML's encoding), replace `<pre>` with `<img class="plantuml-diagram" src="${server}/svg/${encoded}">`. Gate on `schemaStore.app.plantumlServerUrl` being non-empty.
- **Server-rendered documents** (goldmark): `documents:` reports rendered by `markdownToHTML` (`document.go`, consumed in `DocumentView.vue` / `DocumentsPanel.vue`). Add `htmlutil.ConvertPlantUMLBlocks` (rewrite `language-plantuml` → `<pre class="plantuml">`) and call it beside `ConvertMermaidBlocks` (`document.go:356`). Recommend merging both into one `ConvertDiagramBlocks` to remove the two-call-site smell. The JS pass must also handle the `pre.plantuml` form (both forms, like mermaid).

Call the new JS pass from the same three components that call
`renderMermaidDiagrams`: `EntityDetail.vue`, `DocumentView.vue`,
`DocumentsPanel.vue`.

## Out of scope / notes

- Metamodel help text (`simpleMarkdownToHTML`, `/api/help`) is a server-rendered surface too, but `HelpModal.vue` doesn't even upgrade mermaid today — leave at parity (skip), note it.
- Default must be empty (disabled). Never default to `www.plantuml.com` — would leak private data to a third party.
- CSP: main SPA has no CSP today so `<img>` to the server works; optional follow-up to tighten `img-src` to the configured server.
- Encoding: PlantUML's deflate+custom-base64 (`~30` lines, or `pako` which may already be transitively present via mermaid).

## Acceptance

- With `plantuml_server_url` set, a ```plantuml block renders as a diagram in entity body, sections, and documents.
- With it empty/absent, the same block renders as a plain code block (no broken image, no network call).
