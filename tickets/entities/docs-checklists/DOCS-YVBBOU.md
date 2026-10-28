---
id: DOCS-YVBBOU
type: docs-checklist
title: 'Docs: rela.attach_render (TKT-1TS55R)'
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (attachRenderer gate order, write check before render, absent-id fallback type (RR-NGMI), upload slot before render, fsstore hash cache and singleflight, Tx refusal)
- [x] Function/type docs if public API (lua.AttachRenderer, lua.AttachRenderRequest, lua.AttachedFile, lua.ErrAttachRenderUnavailable, dataentry.AttachRenderer, appbuild.SetAttachRenderer, audit.OpAttachRender, audit.AttachRendered, store.AttachmentInfo.SHA256)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows existing rules; internal/dataentry/CLAUDE.md still names gateDocumentPermission and gateElevatedDocument correctly)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-lua-scripting: rela.attach_render section with options, principal per runtime, file audience, audit, availability and loop hazard; docs/transforms.md: attaching an export; GUIDE-attachment-security: ingress list, SHA-256 hash and fsstore cache caveat; docs/data-entry/api-reference.md: sha256 in attachment metadata; docs/ regenerated)
