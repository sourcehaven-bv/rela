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
- `default-safe` named preset: allow png/jpeg/gif/webp, pdf, plain text, office docs,
zip, csv. Block `image/svg+xml`, `text/html`, `application/xhtml+xml`,
executables/scripts.
- Validate against the **sniffed** type (`http.DetectContentType` / magic bytes),
not the client header. **Reject sniff↔extension mismatch** (polyglot /
`.jpg.php`).
- Per-field `accept: [...]` narrows; global config can extend the preset.

### Download hardening
- Download handler always sets `Content-Disposition: attachment` and
`X-Content-Type-Options: nosniff` (force-download, no inline render).

### Scan config plumbing + unconfigured-scan startup warning
- `scan` config exists only as a per-property **opt-out** (`scan: off`).
**Scanning is enabled by configuring a `scan_cmd`** (Phase 2 runs it); there is
no `required` value — wiring a scanner is the intent to use it.
- **Unconfigured-scan startup warning:** if ≥1 `file` property has no scan command
(no global `attachments.scan_cmd`, no property `scan_cmd`) and no `scan: off`,
emit one startup warning linking the attachment-security docs. Configuring a
command or `scan: off` silences it. Warning only — never blocks startup or
uploads.

## Acceptance

- Allowlist rejects SVG/HTML/exe and sniff/extension mismatches; accepts the safe set.
- Download responses carry the hardening headers.
- Unconfigured scan emits exactly one warning; a configured command or `scan: off`
is silent.
- Pure-Go; no new external dependency; cross-compile unaffected; race clean.

## Out of scope

The `cmd:` processor harness + any actual scan/strip/resize execution (Phase 2);
Lua policy (Phase 3). This ticket is native input validation only.
