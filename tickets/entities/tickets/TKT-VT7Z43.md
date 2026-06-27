---
id: TKT-VT7Z43
type: ticket
title: 'Phase 2: generic cmd: processor harness + scan/transform config + doc recipes'
kind: enhancement
priority: medium
effort: l
status: done
---

The processing engine of the attachment pipeline (FEAT-KTZJIV). ONE generic
external-command processor covers virus scan, EXIF strip, resize, and CDR — rela
ships no native scanners/transformers; the *recipes* ship in the docs. Builds on
the Phase 0 seam (TKT-YGLHDL) and the native validation/config from Phase 1
(TKT-4T06HY). See RES-INM8JP (cmd:-only direction; mirrors rela's auth-via-proxy
house style).

## Scope

### The `cmd:` processor (safe invocation is load-bearing)
- A single `Processor` impl that runs a configured external command per upload:
  - **Array args, never a shell string** — `exec` directly, no `sh -c` (no injection).
  - **Templated `{in}`/`{out}`** — rela owns the temp file paths and substitutes them;
the operator never builds a path from the (attacker-influenced) filename.
  - **Timeout + output-size cap** (≤ `MaxAttachmentBytes`) enforced by rela.
  - **Present-probe + startup warn** if a referenced binary is absent — never crash.
  - Scan semantics: non-zero exit / FOUND ⇒ reject; transform semantics: stdout/`{out}`
replaces the stream.

### Wire into declarative config
- `scan: required` runs the field's (or global) scan command, **fail-closed**: reject
on FOUND/non-zero **and** when the command can't run.
- `transform: [cmd: [...]]` per field, ordered; strip still applies alongside resize.
- Reuses the tri-state `scan` config + unset warning landed in Phase 1.

### Doc recipes (vetted, sandboxed snippets — NOT a code registry)
- clamav scan (`clamdscan --stream`), Windows Defender; vips/exiftool EXIF strip; vips
resize/thumbnail; qpdf/ghostscript CDR; ffmpeg transcode. Each snippet shows
array args, templated I/O, the tool's own resource limits, and a "third-party
parser on untrusted input — pin versions, sandbox" banner.

## Acceptance

- `cmd:` processor: array-args only (a shell-metachar in config is data, not executed);
`{in}`/`{out}` resolve to rela-owned temp paths; timeout + output cap enforced;
missing binary → startup warning, not panic.
- `scan: required` with a recipe rejects EICAR and rejects when the command is missing.
- A `transform: [cmd:]` recipe rewrites the stored bytes; failure aborts the write
(attach-first ordering preserved).
- Docs build with the recipe snippets; cross-compile unaffected; race clean.

## Out of scope

Lua conditional-policy selection of processors (Phase 3 / TKT-40PZ15). No native
clamd client, no in-tree EXIF stripper, no native resize — all are `cmd:`
recipes.
