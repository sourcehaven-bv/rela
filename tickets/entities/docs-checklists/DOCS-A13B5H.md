---
id: DOCS-A13B5H
type: docs-checklist
title: 'Docs: Replace command open/reveal launcher with an ACL-gated HTTP download'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported/package-level symbols (`containedPath`, `containProjectPath`, `newRunKey`, `commandFileStore` and its methods, `mintFileToken`, `handleCommandFile`)
- [x] Comments explain *why*, not *what* (the token-is-a-capability rationale, why the re-check runs per download, why `exec_id` is not the token-table key, why a nil store is inert while a nil ACL denies)
- [x] Stale comments corrected rather than left (the `containedProjectPath` TOCTOU note now says this change *widened* the window instead of inheriting the old launcher text; four comments that overclaimed were rewritten — see RR-A98CSH)

## Project Documentation

- [x] `docs-project/entities/guides/GUIDE-data-entry.md` — new **File Downloads** section; `file` message row now says "Offer a file for download" with `path`, `label`; the dead `open` message-type row removed; `auto_open` marked inert with a `rela migrate` pointer; examples changed from `/tmp` to `out/` because a path outside the project root is now refused
- [x] `docs-project/entities/guides/GUIDE-server-security.md` — section 5 rewritten around token-scoped download + per-download re-authorization; section 6 (`/api/open-url` scheme allowlist) removed with the rest renumbered; TOCTOU section rewritten
- [x] `docs/` regenerated via `just docs` and verified idempotent (re-running produces no diff)
- [x] Demo project corrected (`prototypes/data-entry/project/data-entry.yaml`) — its `generate-pdf` wrote to `/tmp` with `action: "open"`, which would now render no Download button

## External Documentation

- [x] ~~README updated~~ (N/A: no top-level feature or install change)
- [x] Breaking/behavior changes called out for users:
  - `auto_open` no longer does anything and is no longer sent in the `/api/v1/_commands` response. The YAML key is still parsed so unmigrated projects load; `rela migrate` strips it.
  - `rela-desktop` users lose Open/Reveal and get a browser download. Accepted in the ticket: no real users today.
  - Command output must be written **inside the project root** to be downloadable. A file elsewhere is still listed, without a button.
- [x] Migration provided rather than leaving operators to hand-edit (`command-auto-open`, verified end-to-end through the real CLI including idempotence)
