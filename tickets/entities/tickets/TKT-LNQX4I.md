---
id: TKT-LNQX4I
type: ticket
title: 'Native in-process image processing: decode-verify, EXIF-orientation, re-encode (Phase 1)'
kind: enhancement
priority: medium
effort: m
status: done
---

## Summary

Add a **pure-Go, in-process image processor** for the common raster formats
(**PNG, JPEG, GIF, WebP-decode**) behind the existing `attachment.Processor`
seam. It runs as a **native transform step** in the current pipeline (`MIME →
scan → transforms`), an alternative to shelling out to vips/ImageMagick via
`cmd:`.

Because the decoders are memory-safe Go, this needs **no OS sandbox** — the
threat collapses from RCE+DoS to **DoS-only**, and DoS is bounded by a pixel cap
+ timeout + concurrency limit. It also delivers **metadata-strip and correct
orientation for free** via re-encode.

**Phase 1 scope (this ticket):** decode-verify + EXIF-orientation + re-encode.
Orientation is IN scope deliberately — re-encode that strips EXIF but ignores
the orientation flag would store phone photos sideways, which is *worse than
today*. Resize/thumbnail is Phase 2 (separate ticket); exotic formats
(HEIC/AVIF/JXL) and SVG are Phase 4.

Research: [[RES-WDFS96]] (measured binary cost **+417 KiB / <1%**; `x/image`
v0.40.0 already in the module graph, `image/jpeg` already linked).

## Motivation / why now

- rela does **no in-process image decoding** today; the only way to strip
EXIF/GPS or normalise an image is an operator-configured external tool
(`transform: [{cmd: [vips|exiftool …]}]`). That is a deployment burden and a
parser-CVE surface (the `cmdexec` bwrap work — [[RES-WDFS96]] Option A — exists
precisely to contain it).
- For the overwhelming majority of real uploads (png/jpeg/gif/webp), we can skip
the external tool *and* the sandbox entirely by decoding in memory-safe Go.
- Metadata strip (privacy: GPS/EXIF) and orientation-correct storage are common,
expected behaviours that should be **built-in**, not opt-in via a shell recipe.

## Design

### Package: new `internal/imgproc` (pure Go, CGo-free)

Self-contained, no domain imports (so it stays testable + `arch-lint` clean).
Public surface (consumer-side; keep it small):

```go
// Config bounds a single normalize operation.
type Config struct {
    MaxPixels    int64         // reject before decode if W*H exceeds this (bomb guard)
    MaxBytes     int64         // io.LimitReader bound on input
    Timeout      time.Duration // per-op wall-clock (decode+encode) via context
    Output       Format        // canonical re-encode target: PNG | JPEG
    JPEGQuality  int           // when Output==JPEG
    Orientation  bool          // apply EXIF orientation before re-encode (default true)
}

// Normalize decodes r (memory-safe), applies EXIF orientation, and re-encodes
// to cfg.Output. Returns ErrTooLarge (dims over cap), ErrUnsupported (format we
// don't handle → caller may fall through to cmd:), or a wrapped decode error.
func Normalize(ctx context.Context, r io.Reader, cfg Config) ([]byte, Info, error)
```

Pipeline inside `Normalize`:
1. `io.LimitReader(r, cfg.MaxBytes)` — cap input.
2. `image.DecodeConfig` first → compute `W*H`; reject `> MaxPixels` **before**
allocating the pixel buffer. This is the decompression-bomb guard (a tiny file
that declares 50 gigapixels).
3. `image.Decode` **inside a worker goroutine** guarded by `recover()` (crafted
input panics — the 2026 webp/bmp CVEs are exactly this — become handled errors)
and a `context` deadline (CPU/entropy bombs get killed).
4. Read EXIF orientation (stdlib has none; use a tiny orientation-only reader —
evaluate `disintegration/imaging.AutoOrientation` vs a ~40-line inline
orientation parser to avoid the dep; decision in Open Questions). Apply the
rotation/flip.
5. Re-encode to `cfg.Output` via stdlib `image/png` / `image/jpeg`. The
re-encoded bytes contain **no metadata** — strip is automatic.
6. Return bytes + `Info{Format, Width, Height}`.

Format support: PNG/JPEG/GIF decode via stdlib; WebP decode via
`golang.org/x/image/webp`. **WebP has no pure-Go encoder** → WebP uploads
re-encode to PNG or JPEG (documented behaviour, not a silent surprise). Anything
`imgproc` can't decode returns `ErrUnsupported` so the caller can fall through
to a `cmd:` transform if one is configured.

### Metamodel: native `image:` transform step

Extend `TransformStep` (`internal/metamodel/types.go:320`) so a step is EITHER a
command OR a native image op — mutually exclusive, validated at load:

```go
type TransformStep struct {
    Cmd   []string   `yaml:"cmd,omitempty"`
    Image *ImageStep `yaml:"image,omitempty"`
}

type ImageStep struct {
    Reencode    string `yaml:"reencode,omitempty"`    // "jpeg" | "png"; default "jpeg"
    Quality     int    `yaml:"quality,omitempty"`     // jpeg quality
    Orientation *bool  `yaml:"orientation,omitempty"` // default true
    // (fit/resize deliberately NOT here yet — Phase 2)
}
```

YAML example:
```yaml
avatar:
  type: file
  transform:
    - image: { reencode: jpeg, quality: 85 }   # strip metadata, orient, normalise
```

Metamodel validation (loader): reject a step with both `cmd` and `image` set, or
neither; reject unknown `reencode` values; `image:` only on `type: file`.

### Wiring: `PolicyProcessor` runs native steps in-process

`internal/attachment/policy.go` / `command.go`: today `applyCommands` iterates
`transformCommands` (cmd-only) and requires a non-nil `CommandRunner`. Change
so:
- Native `image:` steps run via `imgproc.Normalize` **with no runner and no
sandbox** — they work even when `p.runner == nil` (out-of-box), unlike `cmd:`
steps.
- `cmd:` steps keep their existing path (runner-gated, sandboxed via `cmdexec`).
- A mixed pipeline runs steps in declared order, threading bytes through.
- On `ErrRejected`-class outcomes (dims over cap, undecodable when the property
*requires* an image) → wrap `attachment.ErrRejected` → 422, consistent with the
existing rejection→422 mapping (`processor.go:64`).

Config plumbing: pixel cap / timeout / default output come from sane package
defaults, overridable globally (extend `AttachmentsConfig`) — mirror how
`maxAttachmentBytes` is resolved. Timeout is independent of the 60s cmd timeout.

### CLI parity

The CLI attach path (`internal/cli/cli_wiring.go:86`) passes a nil
`CommandRunner` today, so `rela attach` does MIME-only. Native `image:` steps
must run there **too** (they need no runner) — this closes a real gap: `rela
attach` currently can't strip metadata at all. Verify the native path is reached
from CLI, add a test.

## Acceptance criteria

1. A `file` property with `transform: [{image: {reencode: jpeg}}]` re-encodes an
uploaded PNG/JPEG/GIF/WebP to JPEG in-process, with **no external tool and no
sandbox**, on all platforms (incl. where bwrap is unavailable).
2. **EXIF orientation is applied**: an orientation-tagged (e.g. Orientation=6)
JPEG is stored visually upright. Pinned by a test with a known fixture.
3. **Metadata is stripped**: output contains no EXIF/GPS (verified by re-reading
the stored bytes).
4. **Decompression-bomb guard**: a small file declaring dims over `MaxPixels` is
rejected (422) **without** a large allocation (assert via DecodeConfig path).
5. **Crafted-input safety**: the known panic corpus (2026 webp/bmp/tiff-style,
truncated, palette-OOB) yields handled errors, never a process crash. Fuzz test
+ `-race` green.
6. WebP upload with `reencode` → stored as PNG/JPEG (documented; asserted).
7. Undecodable/unsupported input with an `image:` step: rejected if the step is
required, or falls through to a configured `cmd:` step if present.
8. `rela attach` (CLI) applies native `image:` steps (parity with HTTP path).
9. Metamodel rejects malformed steps (both cmd+image / neither / bad reencode /
image on non-file).
10. `just arch-lint`, `just plimsoll`, coverage floors, `govulncheck` all pass;
`x/image` bumped to the current patched release (≥ v0.43.0).

## Test plan

- `internal/imgproc`: table tests over formats; DecodeConfig cap; recover+timeout
worker; orientation fixtures (all 8 EXIF orientations); re-encode strips
metadata; fuzz (`FuzzNormalize`) with `-race`; bomb/panic corpus.
- `internal/metamodel`: loader validation for the new step (valid + all invalid
shapes).
- `internal/attachment`: pipeline integration — native-only, cmd-only, mixed,
runner-nil-still-runs-image, rejection→422.
- CLI: `rela attach` applies an `image:` step.

## Non-goals (explicitly deferred)

- **Resize / thumbnail generation + serving** → Phase 2 (own ticket; involves a
storage-key convention for derived artifacts + a GET/`_thumbnails` affordance).
- **HEIC / AVIF / JPEG-XL / RAW** → Phase 4 (`gen2brain/*` WASM-under-wazero,
CGo-free) iff demand.
- **SVG accept** → Phase 4 (Rust `resvg` in wazero — [[RES-WDFS96]] Option B).
Stays blocked by the MIME allowlist until then.
- No change to scanning, ACL, download hardening, or the `cmdexec` sandbox.

## Design review outcomes (resolved)

`/design-review` ran at level `high` and found 7 issues (6 significant, 1
minor), all now **addressed** in this plan. Review-responses: [[RR-7R3EYR]]
[[RR-4G5YBU]] [[RR-S8FEUJ]] [[RR-ZM3PE7]] [[RR-01E7DG]] [[RR-U517I0]]
[[RR-9IVLFO]].

1. **Re-encode must rewrite the filename extension** ([[RR-7R3EYR]]). A re-encode
(incl. every WebP → PNG/JPEG) that leaves a `.webp`/`.jpg` name on PNG/JPEG
bytes breaks the extension-derived download `Content-Type` (with `nosniff`) and
the sniff↔extension check. → `imgproc` returns `Info.Format`; the native step
sets `ProcessInfo.FileName` with the swapped extension; the seam re-resolves the
name (`attachment.go:169`). **New AC 11.**
2. **Output type is safe by construction** ([[RR-4G5YBU]]). MIME allowlist runs
*before* transforms, so it never sees the re-encoded output. → `reencode` target
is restricted to `{png, jpeg}`, both always in `default-safe`; the swapped
extension is re-checked against the effective allowlist at metamodel load.
Original vague AC replaced by **AC 9 (load validation)** + **AC 11**.
3. **Concurrency semaphore is a REQUIRED control, not a backstop** ([[RR-S8FEUJ]]).
N concurrent decodes each near `MaxPixels` (≈200 MB each) OOM the host even
though each passes the cap. → `imgproc` owns a package weighted semaphore
(default bound small, GOMAXPROCS-based); `MaxPixels` × bound × 4 B is a
documented memory budget. **New AC 12** + a test asserting the in-flight cap.
4. **Timeout unblocks the caller; it does not kill the decode** ([[RR-ZM3PE7]]).
Go decoders don't observe `context`, so a bomb goroutine runs to completion. →
Accepted and made safe by: DecodeConfig cap runs *before* spawning, and the
semaphore bounds how many such goroutines can exist (bounded, not unbounded).
Documented; **goleak test** asserting N bombs leave ≤ bound goroutines.
5. **Animated GIF is rejected, not silently flattened** ([[RR-01E7DG]]).
`image.Decode` yields only frame 1 → silent data loss. → Detect via
`gif.DecodeAll` (`len(frames) > 1`) and **reject** (422, "animated GIF not
supported by native image processing; use a `cmd:` transform"). Single-frame
GIFs re-encode normally. **New AC 13** + edge test.
6. **Orientation default pinned; likely made non-configurable in Phase 1**
([[RR-U517I0]]). The `*bool`(nil→true) → `bool`(false) mapping is an easy
inversion of the very default this ticket exists to protect. → Leaning toward
**always-on orientation in Phase 1** (drop the `orientation:` knob entirely,
removing the trap); if kept, a test pins that an omitted field still orients.
Decision recorded; **AC 2** already covers the upright assertion.
7. **Add a coverage floor for `internal/imgproc`** ([[RR-9IVLFO]]) in
`.testcoverage.yml` (minor). Added to the implementation prerequisites.

### Revised / added acceptance criteria

11. A re-encode that changes format **changes the stored filename extension** to
match (`.jpg`/`.png`); the download `Content-Type` and any re-validation are
consistent with the stored bytes.
12. Concurrent decodes are **capped by a semaphore** to a documented memory
budget; worst-case in-flight allocation stays within it.
13. **Animated (multi-frame) GIF is rejected** (422) by a native `image:` step,
not flattened. Single-frame GIF re-encodes normally.

(AC 9 now also asserts the `reencode` target is within the effective allowlist;
the orientation knob may be dropped per finding 6 — if so, AC updated to "an
`image:` step always applies EXIF orientation".)
