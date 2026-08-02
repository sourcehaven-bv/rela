---
id: RES-WDFS96
type: research
title: Should rela own the attachment-processing sandbox boundary via WebAssembly (wazero)?
summary: 'No, not as a general sandbox: the shipping bwrap/cmdexec work (PR #1188) already confines attachment tools at native speed with full tool breadth and is the primary answer. WASM cannot run ClamAV/vips/pandoc, so it''s only a narrow complement — portable image/SVG transcode, and separately the Lua sandbox — layered behind the same Processor seam after bwrap lands.'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Problem

rela accepts file attachments on the write path. Today it does **no in-process
decoding** of attachment bytes: the native path
(`internal/attachment/mimecheck.go`) only sniffs magic bytes
(`http.DetectContentType`) against a MIME/extension allowlist. All heavy
processing — virus scan, EXIF strip, resize, PDF disarm — is delegated to
**operator-configured external binaries** run by `CmdRunner`
(`internal/attachment/cmdrunner.go`): clamd, vips, exiftool, qpdf, gs. The
security guide originally handed isolation to the operator (*"prefer sandboxing
(containers, seccomp, a dedicated user)"*).

**The decision:** should rela own the attachment-processing sandbox boundary
itself via WebAssembly (wazero)? The crux: *you can only sandbox the parsing you
run yourself.* rela runs none today — it shells out. A WASM answer means
bringing parsing in-process (Rust→wasip1 guests). But there is a third framing
that dissolves the sandbox question for the common formats entirely: **do the
image decoding in memory-safe Go, so there is no memory-corruption threat to
sandbox** (see Option D — the recommended path).

## ⚠️ Superseding context: the bwrap `cmdexec` work already ships this

**A sibling worktree `../rela-pdf-rendering` (branch `feat/transform-registry`,
draft PR #1188, TKT-JF5JI8) has ALREADY built and wired a process-sandbox answer
to this exact problem** — it confines the *existing* external tools. Key facts,
from source:

- New `internal/cmdexec` = shared safe-exec core (argv/no-shell, `{in}`/`{out}`,
timeout, output cap) **plus** a per-OS `Sandbox` + rlimit layer.
`internal/attachment/cmdrunner.go` now builds on `cmdexec.Runner`, so
**attachment scan/transform commands are already confined** (same core shared
with `internal/transform`).
- **Linux = bubblewrap**: `--unshare-all` (no network), read-only allowlist of
converter-needed paths only, one writable bind, `--die-with-parent`,
`--new-session`; real bwrap probe in `Available()`.
- **macOS = sandbox-exec**: denies network + writes; **KNOWN LIMITATION: reads
NOT restricted** (LaTeX exfil works). Best-effort/dev tier; Linux is prod.
- **Windows/BSD = `unavailableSandbox`** → `Wrap` REFUSES (fail-closed); a
distinct `disabledSandbox` is the only unconfined path (operator opt-out).
- **Resource bounds** (Linux prlimit): AS 2 GiB, NPROC 256, FSIZE 1 GiB,
CPU 120 s, process-group kill.

## Options (full analysis)

### Option A — bwrap/`cmdexec` process sandbox (shipping, PR #1188)
Confine the operator's existing native tools. **Pros**: full tool breadth
(ClamAV/vips/pandoc/gs), native speed, ~5–20 ms overhead, already built & wired.
**Cons**: not portable-equal (macOS can't confine reads; Windows/BSD refuse);
host-kernel dependent; contains an RCE but it's still native code exec.
**Effort**: done.

### Option B — WASM (wazero) `Processor`, Rust→wasip1 modules
wazero running Rust `image`/`resvg`/`oxipng`. **Pros**: one portable identical
boundary; structural memory-safety; pure-Go/CGo-free host; enables safe SVG
accept. **Cons**: only covers what rela reimplements in Rust (can't run
ClamAV/vips/pandoc); ~3–5x CPU; no fuel metering; Rust toolchain + `.wasm`
artifacts. **Effort**: Medium; additive to A.

### Option D — Pure-Go native image decode/resize/re-encode (RECOMMENDED for common images)
For **PNG/JPEG/GIF/WebP-decode**, process in-process with memory-safe Go and
**no sandbox**: `io.LimitReader` → `image.DecodeConfig` pixel-cap (bomb guard) →
`image.Decode` in a recover()+timeout worker → resize (`x/image/draw`
CatmullRom) → **re-encode to a known-good format** (strips metadata for free) →
concurrency semaphore + `GOMEMLIMIT` backstop.
- **Pros**: no sandbox needed — threat collapses RCE+DoS → **DoS-only**. Pure
Go, CGo-free, **identical on every platform** (incl. Windows/BSD where bwrap
refuses). No Rust, no `.wasm`, no bwrap kernel dependency. Re-encode covers the
metadata-strip transform recipe natively.
- **BINARY-SIZE COST (measured, not estimated):** injecting the full Option D
stack (stdlib png/jpeg/gif + `x/image` webp+draw + `disintegration/imaging`)
into the real `cmd/rela` main and rebuilding: **+417 KiB on a ~44.6 MiB binary
(< 1 %)**. Reason it's tiny: `image/jpeg` is *already linked* today and
`x/image` v0.40.0 is *already in the module graph transitively*; only `x/image`
subpackages + imaging are genuinely new code. Standalone the stack is ~1.5 MiB
over a bare runtime, but the *marginal* cost against rela is 417 KiB. **This is
not a real objection.**
- **Cons / residual risk**: DoS bounded not eliminated (no per-decode alloc cap;
pixel-cap+timeout+semaphore do the work). `x/image` has a continuous CVE stream
(2026: tiff/webp/bmp) — staying patched (now v0.44.0) is a **hard operational
requirement**; rela already runs govulncheck per-PR, the exact control needed.
**Coverage gaps** (no pure-Go decoder): HEIC/AVIF/JXL/RAW, and **WebP encode**
(decode only → re-encode to PNG/JPEG). **SVG** has no pure-Go rasteriser safe
enough (oksvg partial+unmaintained) — stays blocked, or needs B.
- **Effort**: Low–Medium — days, not weeks.
- **Exotic-format note**: `gen2brain/{heic,avif,jpegxl}` are CGo-free (C/Rust
codec compiled to WASM under wazero) — the natural extension for HEIC/AVIF/JXL
if ever needed (larger binary; blast-radius bounded by the WASM sandbox).

### Option C — bwrap now, add Landlock later (no WASM/no native)
Ship A; optionally tighten Linux with `go-landlock`. Still
Linux-strong/elsewhere-weak; no memory-safety. Low incremental effort.

## Recommendation

**By format tier:**
- **PNG/JPEG/GIF/WebP-decode → Option D (pure-Go native).** Don't sandbox what
you can decode memory-safely. Cheapest to build (~days, +417 KiB), simplest to
operate, works on every platform. Covers the overwhelming majority of real
uploads and delivers metadata-strip + thumbnails natively.
- **ClamAV scan, HEIC/RAW via vips, PDF disarm, pandoc export → Option A
(bwrap).** No pure-Go path; confine the native tool. Already shipping.
- **HEIC/AVIF/JXL decode (if wanted) → gen2brain WASM-under-wazero** (CGo-free
middle ground).
- **SVG accept → Option B (Rust resvg in wazero)**, or stays blocked. The one
place WASM uniquely earns its place on the attachment path.
- **Lua sandbox** (`internal/lua/runtime.go`: no mem limit, no instruction
counter, no SSRF guard) — untouched by cmdexec, a cleaner wazero target than
attachments; higher-value WASM work if there's appetite. Separate initiative
(`lua-scripting`).

**Net:** WASM is **not** the answer to "own the attachment sandbox boundary."
Common images → *don't need a sandbox* (D); native tools → *bwrap* (A,
shipping); WASM survives only for safe-SVG-accept and the separate Lua
initiative.

## Implementation phasing (native image support — Option D)

Built behind the existing `attachment.Processor` seam
(`internal/attachment/policy.go`), as a **native transform kind** in the
pipeline `MIME → scan → transforms`. The current `transform:` is a list of
`{cmd: [...]}` steps; the plan adds a native step kind alongside `cmd:`.

**Phase 1 — native decode-verify + re-encode (the safety win).**
- New `internal/imgproc` package (pure Go): `DecodeConfig` pixel-cap +
`io.LimitReader` + `image.Decode` in a recover()+context-timeout worker + stdlib
re-encode to canonical PNG/JPEG. Formats: PNG, JPEG, GIF, WebP-decode.
- New metamodel affordance, e.g. `transform: [{image: {reencode: jpeg}}]` (or a
property flag `image_normalize: true`) — a *native* transform step distinct from
`cmd:`. Wire it in `PolicyProcessor.applyCommands`/a sibling so native steps run
in-process.
- Config: per-property/global pixel cap, timeout, output format; sane defaults.
- Tests: fuzz + the known bomb/panic corpus (oversize dims, truncated, palette
OOB), `t.Run` table, race-on.
- **Ships the core value**: safe in-process image normalisation + metadata strip,
no external tool, no sandbox.

**Phase 2 — resize / thumbnail.**
- Add `resize`/`thumbnail` params (`{image: {fit: 512}}`) via `x/image/draw`
CatmullRom (or `disintegration/imaging`). Aspect-preserving, never-upscale.
- Optional: generate + store a derived thumbnail attachment (decide storage key
convention — sibling `attachments/<id>/<prop>/.thumb/…` vs a derived-property).
This is the first place a *new stored artifact* appears; may warrant its own
ticket if thumbnail *serving* (a `_thumbnails` affordance / GET route) is in
scope.
- Tests: dimension math, EXIF-orientation handling, quality.

**Phase 3 (optional) — EXIF-orientation + richer metadata handling.**
- Honour EXIF orientation on decode (imaging `AutoOrientation`) so re-encoded
output is visually upright; otherwise metadata is already dropped by re-encode.
- Decide policy: strip-all (default, already free) vs. preserve-orientation.

**Phase 4 (later / separate) — exotic formats + SVG.**
- HEIC/AVIF/JXL via `gen2brain/*` (WASM-under-wazero, CGo-free) *iff* demand.
- SVG accept via Option B (Rust resvg in wazero). Each is its own ticket; neither
blocks Phases 1–3.

**Sequencing note:** Phases 1–3 are pure-Go and **independent of PR #1188** —
they touch `internal/attachment` + metamodel only, not `cmdexec`. Phase 1 alone
is the recommended first slice: highest safety value, lowest cost, no new heavy
deps.

### Key sources
`../rela-pdf-rendering` PR #1188 / TKT-JF5JI8: `internal/cmdexec/*`,
`internal/attachment/{cmdrunner,policy,processor,command}.go`. Go stdlib
`image/*` + `golang.org/x/image` v0.44.0 (already v0.40.0 in graph);
`image.DecodeConfig` bomb guard; `x/image/draw` CatmullRom; 2026 x/image CVEs
(46599 tiff / 46601 webp / 42500 bmp); `gen2brain/{heic,avif,jpegxl}`
(WASM-under-wazero, CGo-free); `srwiley/oksvg` (partial/unmaintained); wazero
v1.12.0; Rust `wasm32-wasip1` (resvg). **Binary-size delta measured 2026-07-24:
+417 KiB / <1 %.** Verified July 2026.
