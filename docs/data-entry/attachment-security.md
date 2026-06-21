# Attachment security: scanning, MIME allowlist & transforms

rela inspects every uploaded attachment on the write path before it is stored.
Two layers do the work:

1. **Native input validation** (always on, pure Go, no external tools) — a
   sniffed MIME allowlist and download hardening. This is your out-of-box
   defense against stored-XSS and disallowed file types.
2. **External-command processing** (opt-in) — virus scanning and byte
   transforms (metadata strip, resize, CDR) are driven by **commands you
   configure**. rela ships no scanner or image library; it runs the tools you
   already trust (ClamAV, vips, exiftool, qpdf, …) with safe invocation.

This split mirrors how rela handles authentication: integration is pushed to a
well-understood external boundary, keeping rela's core small. You bring the
binary; rela drives it safely and records the policy declaratively in your
metamodel.

> **Running a third-party parser on untrusted input is a real attack surface.**
> Pin tool versions, keep them patched, and prefer sandboxing (containers,
> seccomp, a dedicated user). ImageMagick in particular has a history of
> parser CVEs — see the recipes below for `-limit` hardening.

## Configuration

Attachment policy lives in two places in `metamodel.yaml`: a global
`attachments:` block (the safety floor for every `file` property) and per-property
overrides.

```yaml
# Global safety floor — applies to every `file` property unless overridden.
attachments:
  allow: default-safe        # MIME allowlist preset, or an explicit list
  scan: required             # off (default) | required
  scan_cmd: [clamdscan, --no-summary, --stream, "{in}"]

entities:
  report:
    properties:
      # Inherits the global scan + allowlist.
      evidence:
        type: file
        max: 5
        transform:
          - cmd: [vips, thumbnail, "{in}", "{out}[Q=85,strip]", "2048"]

      # A signed PDF: must be clean, must stay byte-for-byte (no transform),
      # and only PDFs are accepted on this field.
      signed_contract:
        type: file
        scan: required
        accept: [application/pdf]
```

### MIME allowlist (`allow`, `accept`)

The allowlist is checked against the **sniffed** content type
(`http.DetectContentType` on the file's magic bytes), never the client-supplied
`Content-Type` header — which is trivially spoofed. A file whose extension
implies one type but whose bytes sniff as an incompatible one (the classic
`.jpg.php` / SVG-polyglot trick) is rejected.

- `allow: default-safe` (the default when unset) permits common images
  (`png`, `jpeg`, `gif`, `webp`), `pdf`, plain text, CSV, ZIP, and the generic
  `application/octet-stream` (office documents sniff this way). It **blocks**
  `image/svg+xml`, `text/html`, `xhtml`, JavaScript, and executable
  extensions (`.exe`, `.dll`, `.sh`, `.ps1`, …) — the active/script-carrying
  types that drive stored-XSS and code execution.
- `allow: [image/png, application/pdf]` — an explicit list narrows the global
  floor.
- Per-property `accept: [application/pdf]` narrows a single field further.

### Scan policy (`scan`, `scan_cmd`)

`scan` is a tri-state:

| value      | behavior |
| ---------- | -------- |
| _(unset)_  | No scanning. rela logs a one-time startup warning so the omission is a conscious choice, not an accident. |
| `off`      | No scanning. Silences the warning. |
| `required` | The `scan_cmd` runs on every upload. **Fail-closed:** the upload is rejected if the scanner reports a hit **or** if the scanner cannot run (missing binary, timeout, daemon down). |

`scan_cmd` is an **array** of command arguments — never a shell string, so a
filename or byte sequence can never inject a shell command. Use the `{in}`
placeholder for the file; rela substitutes a path to a temp file it owns. A
non-zero exit code means "not clean" and rejects the upload.

### Transforms (`transform`)

`transform` is an ordered list of commands that rewrite the bytes. Each step is
`{cmd: [...]}`. Use `{in}` for the input file and `{out}` for the output file
rela should read back; if a command writes to stdout instead, omit `{out}`.
Transforms are **opt-in per field** — they mutate bytes, so rela never applies
one unless you ask.

## Safe-invocation guarantees

For every scan and transform command, rela:

- runs the binary **directly with array args** — no shell, so no injection;
- substitutes `{in}`/`{out}` with **temp-file paths it owns** — your command
  never sees, and never has to sanitize, the user's filename;
- enforces a **timeout** (60s per command) and an **output-size cap** (the
  per-attachment limit, 64 MiB);
- **probes every configured binary at startup** and warns if one is missing,
  so you find a typo or an uninstalled tool at boot, not on first upload.

## Recipes

These are starting points. Test them against your own deployment, pin versions,
and sandbox the tools.

### Virus scanning — ClamAV

Run the ClamAV daemon (`clamd`) and scan over its stream interface:

```yaml
attachments:
  scan: required
  scan_cmd: [clamdscan, --no-summary, --fdpass, "{in}"]
```

`clamdscan` exits non-zero when a signature matches, which rela maps to a
rejected upload. `--fdpass` hands the file descriptor to the daemon (fast, no
copy); use `--stream` instead if `clamd` runs on another host.

On Windows, point `scan_cmd` at your AV's CLI scanner (e.g. a Microsoft
Defender `MpCmdRun.exe -Scan -ScanType 3 -File {in}` invocation that exits
non-zero on detection).

### Strip image metadata (EXIF/GPS) — exiftool

```yaml
evidence_photo:
  type: file
  transform:
    - cmd: [exiftool, -all=, -overwrite_original, "{in}", -o, "{out}"]
```

`exiftool -all=` removes every metadata block (EXIF, GPS, XMP, IPTC) losslessly
— it does not re-encode the pixels.

### Strip metadata + resize — vips

```yaml
avatar:
  type: file
  transform:
    - cmd: [vips, thumbnail, "{in}", "{out}[Q=85,strip]", "512"]
```

`vips thumbnail` resizes to fit 512px; the `[strip]` save option drops metadata
and `[Q=85]` sets JPEG quality. vips handles HEIC/TIFF/RAW that the native
sniffer cannot.

### Resize / convert — ImageMagick (with hardening)

```yaml
photo:
  type: file
  transform:
    - cmd: [magick, "{in}", -strip, -resize, "2048x2048>", -limit, memory, "256MiB", -limit, disk, "1GiB", "{out}"]
```

`-strip` removes metadata; `-resize 2048x2048>` only shrinks (never upscales);
the `-limit` flags cap ImageMagick's resource use — important given its parser
CVE history. Prefer vips or exiftool where they suffice.

### Document disarm (CDR) — qpdf / Ghostscript

Re-write a PDF to neutralize embedded JavaScript and malformed structures:

```yaml
contract:
  type: file
  accept: [application/pdf]
  transform:
    - cmd: [qpdf, --linearize, --object-streams=generate, "{in}", "{out}"]
```

For heavier sanitization, flatten through Ghostscript:

```yaml
contract:
  type: file
  accept: [application/pdf]
  transform:
    - cmd: [gs, -q, -dNOPAUSE, -dBATCH, -dSAFER, -sDEVICE=pdfwrite, -sOutputFile={out}, "{in}"]
```

## Notes & limits

- **Existing files are not retroactively processed.** Scanning, the allowlist,
  and transforms are write-time gates. Tightening policy does not re-scan or
  re-strip attachments already stored. (A bulk rescan tool may come later.)
- **Synchronous.** Processing runs during the upload request, so the browser's
  progress bar covers it. Keep `scan_cmd`/transforms fast for large files.
- **`required` is fail-closed.** If you mark a field `scan: required`, uploads
  to it stop working when the scanner is down — that is the intended safety
  behavior. Use `off` (explicitly) where availability matters more than the
  guarantee.
