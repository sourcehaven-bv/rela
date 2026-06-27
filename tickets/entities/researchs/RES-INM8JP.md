---
id: RES-INM8JP
type: research
title: Research
summary: 'cmd:-only attachment processing: one generic external-command processor covers scan/strip/resize/CDR via doc recipes. Scanning is enabled by configuring a scan_cmd (no separate `required` toggle; per-field `scan: off` opt-out). MIME allowlist + sniffing + download hardening stay native. Phased 0 (seam) -> 1 (native validation + unconfigured-scan warning) -> 2 (cmd: harness + recipes). Mirrors rela''s auth-via-proxy house style.'
status: done
---

## AMENDMENT (2026-06-21, post-implementation): scan is command-driven, not a `required` toggle

The `scan` tri-state below (`unset` / `off` / `required`) was simplified during
implementation. **Configuring a `scan_cmd` is what enables scanning** — there is
no separate `required` value (if you wire a scanner, you want it used). `scan`
survives only as a per-property **opt-out**: `scan: off` skips scanning on that
field despite a global `scan_cmd`. The startup warning fires when a `file`
property has **no scan command configured** (and no `scan: off`). Net: one knob
(`scan_cmd` presence) + an off-switch, removing the "set a command but forgot to
require it → silent no-scan" footgun. The rest of the design (cmd:-only, native
MIME validation, phasing) is unchanged. See
`docs/data-entry/attachment-security.md` for the final config surface.

---

## Problem

The attachment write path (`internal/attachment.WriteAttachment`) streams
uploaded bytes straight to the store with no inspection. Users want to (a)
virus-scan uploads (e.g. ClamAV) and (b) transform them (strip image
EXIF/metadata, resize / thumbnail). Today there is no seam to interpose either,
and no declarative way to say "scan everything, thumbnail this one field."

## Context

The single chokepoint is `Service.WriteAttachment` — both the CLI (`Attach`) and
the data-entry HTTP upload handler flow through it. `r io.Reader` is the raw
stream; a processing step wrapping `r` before `AttachFile` covers both scan and
transform. Constraints: 8 cross-compile targets incl. Windows + no-CGO
discipline → rules out libclamav / libvips / imagemagick as linked deps.

## FINAL DIRECTION — `cmd:`-only processing

**Processing is one generic external-command (`cmd:`) processor. rela ships NO
native scanners or transformers.** Input *validation* that needs no external
tool (MIME allowlist, content sniffing, download hardening) stays native and
in-core.

**Why `cmd:`-only (user reasoning):** `cmd:` is wanted regardless (long-tail
coverage); once it exists it does scan/strip/resize/CDR — every native processor
is additive maintenance for a subset. Native *narrows* tool choice (clamd-only
vs Defender/commercial AV). Mirrors rela's house style of **outsourcing auth to
the proxy**: push integration to a well-understood external boundary, keep the
core small.

## Decisions (final)

### Native, in-core (input validation)
- **MIME allowlist** — `default-safe` preset, validated against the **sniffed** type
(not the header), rejecting SVG/HTML/executables and sniff↔extension mismatches
(polyglot / `.jpg.php`). Per-field `accept:` narrows. Default-on.
- **Download hardening** — `Content-Disposition: attachment` + `nosniff` + CSP
sandbox, always.
- **Unconfigured-scan startup warning** — warn once when a `file` property has no
scan command and no `scan: off`. Never blocks startup/uploads.

### `cmd:` processing (external tools)
- **Scanning is enabled by configuring `scan_cmd`** (global or per-property);
presence = intent, always **fail-closed** (reject on a hit or when the scanner
can't run). `scan: off` per-property opts out of a global command. No `required`
toggle. (See AMENDMENT above — simplified from the original tri-state.)
- **Transforms** — `transform: [{cmd: [...]}]` per field, ordered (strip, resize,
CDR). Opt-in.
- **One mechanism, recipes in docs** — rela ships the `cmd:` processor; vetted
sandboxed recipes (clamav, vips, exiftool, qpdf, ghostscript, ffmpeg, Defender)
ship in `docs/data-entry/attachment-security.md`. No curated named-processor
registry.
- **Safe-invocation discipline (load-bearing):** array args (no shell → no
injection), templated `{in}`/`{out}` temp paths rela owns, timeout + output-size
cap, present-probe + startup warn.
- **Lua = policy, not transport** — TKT-40PZ15 (deferred) selects/gates `cmd:`
processors conditionally; it does not move bytes.

### Cross-cutting
- Existing files are not retroactively processed (write-time gates). A rescan tool
is a later idea.

## Phasing (as built)

- **Phase 0 — seam.** `attachment.Processor` interface, no-op default, conditional
buffering (zero-copy when no processor; ≤64 MiB buffer otherwise). [TKT-YGLHDL]
- **Phase 1 — native validation.** `default-safe` sniffed allowlist +
extension-mismatch reject; download hardening; `scan` config (off opt-out) +
unconfigured-scan startup warning. Pure-Go, default-on. [TKT-4T06HY]
- **Phase 2 — `cmd:` harness.** Generic external-command processor (array args,
templated I/O, timeout, cap, present-probe); `scan_cmd` (presence-enabled,
fail-closed) + `transform: [cmd:]`; doc recipes. [TKT-VT7Z43]
- **Phase 3 (deferred)** — Lua conditional-policy hook. [TKT-40PZ15]

## Recommendation

`cmd:`-only processing + native input validation, phased 0→1→2 with Lua
deferred. One processing mechanism (recipes are prose, not Go), arbitrary
long-tail coverage, no per-tool code, consistent with auth-via-proxy. Key risk:
`cmd:` safe-invocation (array-args / timeout / cap / probe) is what stands
between "extensible" and "RCE via upload."

### Sources

- ClamAV `clamd(8)` — INSTREAM protocol (clamav is now a `cmd:` recipe, not native)
- OWASP File Upload Cheat Sheet — governs the native validation layer
- Lossless EXIF strip = drop JPEG APP1/APP13/APP14 + PNG ancillary chunks
(`scottleedavis/go-exif-remove` wraps the unmaintained `dsoprea` stack) — now an
`exiftool`/`vips` `cmd:` recipe

Related: [[metamodel-types]] [[data-entry-server]] [[lua-scripting]] — feeds
FEAT-870YCY (Attachments as a first-class product feature).
