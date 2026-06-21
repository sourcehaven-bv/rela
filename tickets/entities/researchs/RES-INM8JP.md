---
id: RES-INM8JP
type: research
title: Research
summary: 'cmd:-only attachment processing: one generic external-command processor (array args, templated I/O, timeout, cap, present-probe) covers scan/strip/resize/CDR via doc recipes — no native clamd/EXIF/resize code. MIME allowlist + sniffing + download hardening stay native (pure-Go input validation). Phased 0 (seam) -> 1 (cmd: harness + MIME validation) -> 2 (recipes + config + Lua policy). Mirrors rela''s auth-via-proxy house style.'
status: done
---

## Problem

The attachment write path (`internal/attachment.WriteAttachment`) streams
uploaded bytes straight to the store with no inspection. Users want to (a)
virus-scan uploads (e.g. ClamAV) and (b) transform them (strip image
EXIF/metadata, resize / thumbnail). Today there is no seam to interpose either,
and no declarative way to say "scan everything, thumbnail this one field."

## Context

The single chokepoint is `Service.WriteAttachment` — both the CLI (`Attach`) and
the data-entry HTTP upload handler flow through it. The critical line is:

```go
if err := s.deps.Store.AttachFile(ctx, e.ID, propName, fileName, r); err != nil {
```

`r io.Reader` is the raw stream. Inserting a processing step that wraps `r`
before `AttachFile` covers both scan (read bytes → verdict → maybe reject) and
transform (consume `r` → produce `r'`). Existing guards already present: 64 MiB
cap (`MaxAttachmentBytes` + `CapAttachmentReader`), `NormalizeFileName` /
`ValidateFileName` / suffix-on-collision, ACL-gated writes, download served via
handler (not static path). Constraints: **8 cross-compile targets incl.
Windows** and a **no-CGO discipline** (postgres build-tag rules in CLAUDE.md) —
rules out libclamav / libvips / imagemagick as linked deps.

User decisions captured (2026-06-21):
- Synchronous processing is fine ("files are small-ish, sync + progress indicator").
- Config: **both global and per-field**.

## FINAL DIRECTION — `cmd:`-only processing (supersedes the native-first survey below)

**Processing is one generic external-command (`cmd:`) processor. rela ships NO
native scanners or transformers** (no clamd client, no hand-rolled EXIF
stripper, no native resize). Input *validation* that needs no external tool
(MIME allowlist, content sniffing, download hardening) stays native and in-core.

**Why `cmd:`-only (user reasoning, 2026-06-21):**
- **`cmd:` is wanted regardless** — it's the only way to cover the long tail
(HEIC/RAW/CDR/video/commercial-AV/DLP). Once it exists it *already* does virus
scanning, EXIF strip, resize — **everything**. Every native processor is then
pure additive maintenance: a second code path doing a subset of what `cmd:`
already covers.
- **The native benefits don't carry their weight here.** Performance (no per-upload
exec) is not a primary concern for small sync uploads. "Works once you point at
the daemon" is marginal vs. copying a doc snippet.
- **Native *narrows* choice.** Baking in clamd assumes everyone wants clamd; a Windows
operator wanting Defender, or a shop with a commercial AV, is *better* served by
`cmd:`. clamav itself is just a `cmd:` recipe (`clamdscan --stream -`).
- **Architectural fit / house style.** rela already **outsources auth to the
proxy/headers** rather than building it in. "Push integration to a
well-understood external boundary, keep the core small" is the established
pattern. `cmd:` is the attachment-processing analogue of "auth is the proxy's
job." Native processors would violate exactly that principle.

**Consequences accepted:**
- Scan and EXIF-strip become **opt-in via a `cmd:` recipe** — nothing scans/strips
out-of-box until the operator wires a command. This makes the **unset-scan
startup warning** (below) the primary nudge.
- MIME allowlist + sniffing + download hardening **stay native, default-on** (they're
pure-Go input validation, not tool-driving — the "outsource it" logic doesn't
apply). This keeps the SVG-XSS / polyglot defense on by default with zero
operator setup.

## Decisions (final, 2026-06-21)

### Native, in-core (input validation — no external tool)
| Control | Default | Override | Phase |
|---|---|---|---|
| **MIME allowlist** | `default-safe` preset, **sniffed** + extension-mismatch reject | per-field `accept:`, global extend | 1 |
| **Download hardening** | `Content-Disposition: attachment` + `X-Content-Type-Options: nosniff` always | — | 1 |
| **Unset-scan startup warning** | warn if ≥1 `file` prop and `scan` neither `required` nor explicit `off` | explicit `off`/`required` silence | 1 |

- **`default-safe` allowlist:** allow png/jpeg/gif/webp, pdf, plain text, office docs
(docx/xlsx/pptx/odt…), zip, csv. Block `image/svg+xml`, `text/html`,
`application/xhtml+xml`, executables/scripts. Validated against the **sniffed**
type, sniff↔extension mismatch rejected (polyglot / `.jpg.php` defense).
- **Unset-scan warning:** config model must distinguish "unset" from "explicit off"
(tri-state / `*ScanPolicy`, not a bare bool). Warning is the only signal — never
blocks startup or uploads.

### `cmd:` processors (everything that drives an external tool)
| Concern | How |
|---|---|
| **Virus scan** | `scan: [cmd: [clamdscan, --stream, ...]]` (or any AV) — fail-closed: reject on non-zero/FOUND **and** when the command can't run, when the field requires it |
| **EXIF/metadata strip** | `transform: [cmd: [vips/exiftool/...]]` per field |
| **Resize / thumbnail** | `transform: [cmd: [vips, thumbnail, ...]]` per field |
| **CDR (PDF/office disarm)** | `transform: [cmd: [qpdf/ghostscript, ...]]` per field |

- **One mechanism, recipes in docs.** rela ships the `cmd:` processor; the vetted,
sandboxed *recipes* (clamav, vips, imagemagick, exiftool, qpdf, ghostscript,
ffmpeg, Windows Defender, internal DLP) ship in the docs. Knowledge ships;
binary-driving code does not. No curated named-processor registry (maintenance
trap: per-tool flag drift, false abstraction, still needs a generic hatch → two
mechanisms).
- **`cmd:` safe-invocation discipline (load-bearing — the line between "extensible" and
"RCE via upload"):** **array args, never a shell string** (no `sh -c` → no
injection); **templated `{in}`/`{out}`**, rela owns the temp paths (operator
never builds a path from the filename); **timeout + output-size cap** enforced
by rela; **present-probe + startup warn** if a referenced binary is absent
(never crash); docs banner: "you are running a third-party parser on untrusted
input — pin versions, sandbox."
- **Lua = policy, not transport.** The planned Lua write-veto hook (TKT-40PZ15) is the
optional *conditional-policy* layer ("scan required only for the `legal` type",
"pick recipe by principal/quota") that selects/gates `cmd:` processors. It does
not move bytes.

### Cross-cutting
- **Existing files:** write-time gates do not retroactively quarantine already-stored
attachments. Documented; a "rescan/restrip existing" tool is a later idea.

## Config surface

```yaml
# metamodel top-level — native input-validation floor
attachments:
  allow: default-safe          # named preset | explicit sniffed-mime list (native)
  scan: off                    # off (default; warns if unset) | required (cmd: recipe must be wired)

# per-property — all processing via cmd: recipes (see docs)
evidence_photo:
  type: file
  max: 5
  transform:
    - cmd: [vips, thumbnail, "{in}", "{out}", "2048"]   # array args, templated I/O
signed_contract:
  type: file
  scan: required               # this field must pass its scan recipe
  accept: [application/pdf]     # native allowlist narrowing
  scan_cmd: [clamdscan, --no-summary, --stream, "{in}"]   # recipe (or inherit a global default)
```

## Phasing (final)

- **Phase 0 — the seam.** `attachment.Processor` consumer-side interface, no-op default,
**conditional buffering** (buffer ≤64 MiB only when a processor is registered;
otherwise keep today's zero-copy stream). Benchmark memory-vs-temp-file here.
- **Phase 1 — native input validation + the warning.** `default-safe` sniffed MIME
allowlist with extension-mismatch reject; download hardening headers; unset-scan
startup warning; tri-state scan config plumbing. **No external tools yet** —
pure-Go, default-on, addresses SVG-XSS immediately. Highest-leverage, cheapest
controls.
- **Phase 2 — the `cmd:` processor harness.** Generic external-command processor: array
args, templated `{in}`/`{out}`, timeout, output-size cap, present-probe +
startup warn; wire `scan: [cmd:]` (fail-closed on `required`) and `transform:
[cmd:]` into the declarative config. **Ship the doc recipes** (clamav scan,
vips/exiftool strip, vips resize, qpdf CDR, Defender, ffmpeg). This is where
scan + strip + resize + CDR all actually become usable — via recipes, not native
code.
- **Phase 3 — Lua policy hook (deferred).** TKT-40PZ15 repurposed: conditional selection
/gating of `cmd:` processors by entity type / principal / quota / time. Policy
only; the heavy lifting stays in the `cmd:` harness.

**Dropped from earlier drafts:** native clamd INSTREAM client, hand-rolled
JPEG/PNG metadata stripper, native stdlib resize. All are subsets of `cmd:` and
were net-added maintenance for benefits (perf, zero-binary) the user
de-prioritized.

## Recommendation

**`cmd:`-only processing + native input validation, phased 0→1→2 with Lua policy
deferred (3).** One processing mechanism (recipes are prose, not Go), arbitrary
long-tail coverage, no per-tool code to maintain, and consistent with rela's
auth-via-proxy house style. Real risks: (1) **buffering regression** in Phase 0
— keep the no-processor path zero-copy and benchmark; (2) **`cmd:`
safe-invocation** — array-args / templated-I/O / timeout / cap / probe is the
security crux; (3) **weaker out-of-box posture for scan/strip** — mitigated by
the native allowlist+hardening (default-on) and the unset-scan startup warning.

---

## Survey record (Q1–Q4) — informed the above; native conclusions SUPERSEDED by the cmd:-only direction

### Q1 — ClamAV API. clamd has a stable `INSTREAM` socket protocol (`zINSTREAM\0`,
4-byte big-endian length-prefixed chunks, zero-length terminator; `stream: OK` /
`stream: <Sig> FOUND`; ≤ `StreamMaxLength`). A native Go client is ~80 lines, no
dependency. *Superseded:* we drive clamav via a `cmd: [clamdscan, --stream]`
recipe instead of a native client — clamav is not privileged over other tools.

### Q2 — Lua for clamd: wrong layer (binary I/O through gopher-lua is painful). Confirmed:
Lua is **policy**, not transport.

### Q3 — EXIF strip library survey. Lossless strip = drop JPEG APP1/APP13/APP14 + PNG
ancillary chunks. `scottleedavis/go-exif-remove` wraps the **unmaintained
`dsoprea` stack** (active fork: `superseriousbusiness/go-jpeg-image-structure`).
Hand-rolling was tractable. *Superseded:* strip is now an `exiftool`/`vips`
`cmd:` recipe — no in-tree stripper to maintain.

### Q4 — OWASP File Upload Cheat Sheet (still governs the NATIVE validation layer):
allowlist + **sniff, don't trust the header** + reject sniff/extension mismatch;
size limit (✅ 64 MiB); store outside webroot / serve via handler (✅); download
hardening (`Content-Disposition: attachment`, `nosniff`); SVG/HTML are the
stored-XSS sharp edge → blocked by the default allowlist. Filename-as-key is a
deliberate deviation from "rename to UUID" (mitigated by
Normalize/Validate/suffix; document it).

### Sources

- ClamAV `clamd(8)` — INSTREAM protocol (linux.die.net / Debian manpages)
- OWASP File Upload Cheat Sheet — cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html
- Lossless EXIF strip = drop JPEG APP1/APP13/APP14 + PNG ancillary chunks; `scottleedavis/go-exif-remove` wraps the unmaintained `dsoprea` stack

Related: [[metamodel-types]] [[data-entry-server]] [[lua-scripting]] — feeds
FEAT-870YCY (Attachments as a first-class product feature).
