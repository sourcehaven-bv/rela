---
id: TKT-4T06HY
type: ticket
title: 'Phase 1: native MIME allowlist + content-sniffing + download hardening + unset-scan warning'
kind: enhancement
priority: medium
effort: m
status: done
---

First user-visible phase of the attachment processing pipeline (FEAT-KTZJIV) —
the **native, pure-Go input-validation** controls that need no external tool, so
they are default-on with zero operator setup. Builds on the Phase 0 seam
(TKT-YGLHDL). See RES-INM8JP (cmd:-only direction).

## Scope (all native, no external tools)

### MIME allowlist (sniffed)
- `default-safe` named preset: allow png/jpeg/gif/webp, pdf, plain text, office docs
(docx/xlsx/pptx/odt…), zip, csv. Block `image/svg+xml`, `text/html`,
`application/xhtml+xml`, executables/scripts.
- **Validate against the sniffed type** (`http.DetectContentType` / magic bytes), not
the client header. **Reject sniff↔extension mismatch** (polyglot / `.jpg.php`).
- Per-field `accept: [...]` narrows; global config can extend the preset.

### Download hardening
- Download handler always sets `Content-Disposition: attachment` and
`X-Content-Type-Options: nosniff` (force-download, no inline render) — closes
the SVG/HTML stored-XSS vector for anything that slips the allowlist.

### Scan config plumbing + unset-scan startup warning
- Add the tri-state `scan` config (`off | required`, distinguishing **unset** from
explicit `off` — `*ScanPolicy` / tri-state, not a bare bool), global +
per-field. The *enforcement* (running a scan command) lands in Phase 2; this
ticket lands the config model and the warning.
- **Unset-scan startup warning:** if ≥1 `file` property exists and `scan` is neither
`required` nor an **explicit** `off`, emit one startup warning linking the
attachment-security docs. Explicit `off`/`required` silence it. Warning only —
never blocks startup or uploads.

## Acceptance

- Allowlist rejects SVG/HTML/exe and sniff/extension mismatches; accepts the safe set.
- Download responses carry the hardening headers (handler test).
- Unset scan emits exactly one warning; explicit `off`/`required` are silent.
- Pure-Go; no new external dependency; cross-compile unaffected; race clean.

## Out of scope

The `cmd:` processor harness + any actual scan/strip/resize execution (Phase 2);
Lua policy (Phase 3). This ticket is native input validation only.
