---
id: GUIDE-attachment-security
type: guide
title: "Attachment Security: Scanning, MIME Allowlist & Transforms"
status: published
order: 14
audience: intermediate
summary: "Virus scanning, a sniffed MIME allowlist, and byte transforms for uploaded attachments"
---

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
> Pin tool versions and keep them patched. ImageMagick in particular has a
> history of parser CVEs — see the recipes below for `-limit` hardening.

rela **confines** every scan/transform command it runs, rather than leaving
isolation entirely to you. Each command gets:

- **no network access** (bubblewrap on Linux, `sandbox-exec` on macOS), which
  removes the server-side request forgery vector — a crafted upload cannot make
  the server fetch internal services or cloud metadata;
- a **read-only view of the system**, with only the command's own temp directory
  writable;
- **resource ceilings** on memory, processes, file size, and CPU (Linux), so a
  malicious file cannot exhaust the host;
- **process-group cleanup**, so a helper spawned by the tool cannot outlive the
  timeout;
- a **bounded pool**, so concurrent uploads cannot multiply resource use.

Where no sandbox mechanism is available (Windows, BSD, or a Linux kernel without
unprivileged user namespaces), commands **refuse to run** rather than run
unconfined — the same fail-closed stance as a scanner that cannot start. Only
command execution is blocked; the server still starts. The startup log states
which mechanism is in use. Deployment-level isolation (a container, a dedicated
user, a no-egress network policy) remains worthwhile defence in depth.

## Configuration

Attachment policy lives in two places in `schema.yaml`: a global
`attachments:` block (the safety floor for every `file` property) and per-property
overrides.

```yaml
# Global safety floor — applies to every `file` property unless overridden.
attachments:
  allow: default-safe        # MIME allowlist preset, or an explicit list
  scan_cmd: [clamdscan, --no-summary, --stream, "{in}"]   # configuring this enables scanning

entities:
  report:
    properties:
      # Inherits the global scan command + allowlist.
      evidence:
        type: file
        max: 5
        transform:
          - cmd: [vips, thumbnail, "{in}", "{out}[Q=85,strip]", "2048"]

      # A signed PDF: scanned (inherits the global command), kept byte-for-byte
      # (no transform), and only PDFs are accepted on this field.
      signed_contract:
        type: file
        accept: [application/pdf]

      # An internally-generated, already-trusted export: opt out of scanning.
      nightly_export:
        type: file
        scan: off
```

## MIME allowlist (`allow`, `accept`)

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

## Scanning (`scan_cmd`, `scan: off`)

**Configuring a `scan_cmd` is what turns scanning on** — there is no separate
"required" toggle. If a scan command is configured for a `file` property
(globally via `attachments.scan_cmd`, or on the property itself), every upload
to it is scanned, **fail-closed**: the upload is rejected if the scanner reports
a hit **or** if the scanner cannot run (missing binary, timeout, daemon down).

| config | behavior |
| ------ | -------- |
| `scan_cmd` set (global or per-property) | Scan every upload, fail-closed. |
| no `scan_cmd` anywhere | No scanning. rela logs a one-time startup warning so the omission is a conscious choice, not an accident. |
| property `scan: off` | Skip scanning on that one property, even when a global `scan_cmd` exists. Silences the warning for it. |

A property's own `scan_cmd` overrides the global one. To silence the startup
warning without scanning, either set a global `scan_cmd` or mark each file
property `scan: off`.

A scan command is a **verdict gate**: rela ignores its output and reads only its
**exit code** — `0` means clean, any non-zero means "not clean" and the upload
is rejected. `scan_cmd` is an **array** of arguments — never a shell string, so a
filename or byte sequence can never inject a shell command. Use the `{in}`
placeholder for the file; rela substitutes a path to a temp file it owns.

## Transforms (`transform`)

`transform` is an ordered list of steps that **rewrite the bytes**. Each step is
one of two kinds — set exactly one per step:

- `{cmd: [...]}` — an **external command**. Use `{in}` for the input file and
  `{out}` for the output file rela should read back; if a command writes to
  stdout instead, omit `{out}`. External commands are sandboxed (see
  Safe-invocation guarantees).
- `{image: {...}}` — a **native, in-process image transform**. No external tool,
  no sandbox needed: rela decodes the upload with a memory-safe pure-Go decoder,
  bakes in EXIF orientation, and re-encodes to a canonical format.

Transforms are **opt-in per field** — they mutate bytes, so rela never applies
one unless you ask.

(Note the asymmetry with scanning: a **scan** is a gate judged by exit code; a
**transform** is a filter whose output replaces the stream.)

### Native image transform (`image`)

The `image` step handles the common raster formats with no external dependency:

```yaml
avatar:
  type: file
  transform:
    - image: { reencode: jpeg, quality: 85 }   # strip metadata, orient, normalize
```

- **Inputs:** PNG, JPEG, GIF, and WebP (decode). **Output:** `reencode: jpeg`
  (the default) or `png` — there is no pure-Go WebP encoder, so a WebP upload is
  re-encoded to JPEG/PNG. `quality` sets JPEG quality (ignored for PNG).
- **EXIF orientation is always applied** — a phone photo tagged "rotate 90°" is
  stored visually upright. There is no opt-out: storing sideways would be worse
  than not processing.
- **Metadata is stripped** as a side effect of re-encoding: EXIF, GPS, and any
  trailing data are gone from the stored bytes.
- **The stored file extension is updated** to match the output format (`.jpg`/
  `.png`), so the download `Content-Type` stays consistent with the bytes.
- **Why it needs no sandbox:** the Go decoders are memory-safe, so a crafted
  image cannot achieve code execution — the residual risk is denial-of-service
  only, which rela bounds with a pixel cap (rejects decompression bombs from the
  header, before allocating), a decode timeout, and a process-wide concurrency
  limit. Keep rela updated: the underlying `golang.org/x/image` decoders receive
  periodic security fixes for malformed-input crashes (rela runs `govulncheck`
  in CI to catch them).
- **Not handled:** animated (multi-frame) GIF is **rejected** rather than
  silently flattened to one frame — use a `cmd:` transform if you need to process
  it. HEIC/AVIF/JPEG-XL/RAW and SVG are not supported by the native step; use a
  `cmd:` transform (e.g. vips) for those.
- **Available everywhere**, including `rela attach` on the CLI and on platforms
  where the external-command sandbox is unavailable — because it runs in-process.

## Safe-invocation guarantees

These apply to **external** scan and `cmd:` transform commands (the native
`image:` transform runs in-process and needs none of them). For every such
command, rela:

- runs the binary **directly with array args** — no shell, so no injection;
- substitutes `{in}`/`{out}` with **temp-file paths it owns** — your command
  never sees, and never has to sanitize, the user's filename;
- enforces a **timeout** (60s per command) and an **output-size cap** (the
  per-attachment limit, 64 MiB);
- **probes every configured binary at startup** and warns if one is missing,
  so you find a typo or an uninstalled tool at boot, not on first upload.

## Running under systemd

rela confines external commands with **bubblewrap**, which has to *create*
namespaces. A conventionally hardened unit forbids exactly that, and the failure
is quiet: the service starts, serves HTTP, and logs the sandbox as unavailable —
after which every upload to a scanned property is **rejected**, because scanning
is fail-closed.

Three directives break it. Each is sufficient on its own:

| Directive | Symptom | What works |
| --------- | ------- | ---------- |
| `RestrictNamespaces=yes` | bwrap exits 1 | `RestrictNamespaces=user mnt pid net ipc uts cgroup` — an **allowlist**; `cgroup` is required |
| `SystemCallFilter=@system-service` | bwrap killed by SIGSYS (exit 159) | add `mount umount2 pivot_root unshare setns clone clone3` |
| `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX` | bwrap exits 1 | add `AF_NETLINK` — bwrap configures loopback in its new netns |

Everything else you would normally reach for is compatible: `NoNewPrivileges`,
`ProtectSystem=strict`, `ProtectHome`, `PrivateDevices`, `ProtectProc=invisible`,
an empty `CapabilityBoundingSet`, `MemoryDenyWriteExecute`, `LockPersonality`.

A working unit — `systemd-analyze security` rates this **2.4 OK**:

```ini
[Unit]
Description=rela server
After=network.target clamav-daemon.service
Wants=clamav-daemon.service

[Service]
Type=simple
User=rela
Group=rela
# Only needed if clamd.conf sets LocalSocketMode 660. Debian ships 666.
SupplementaryGroups=clamav
ExecStart=/usr/local/bin/rela-server --project /var/lib/rela/project --bind 127.0.0.1 --port 8080
Restart=on-failure
RestartSec=2
WorkingDirectory=/var/lib/rela

# ---- filesystem ----
ProtectSystem=strict
ProtectHome=yes
StateDirectory=rela
ReadWritePaths=/var/lib/rela
# Compatible with scanning ONLY because --stream sends contents over the socket.
# A path-based scan would break here: clamd lives in the host's /tmp namespace
# and cannot see this private one.
PrivateTmp=yes

# ---- privilege ----
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
ProtectHostname=yes
ProtectProc=invisible
RestrictSUIDSGID=yes
RestrictRealtime=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
SystemCallArchitectures=native
RemoveIPC=yes
UMask=0077

# ---- required for the attachment sandbox (bubblewrap) ----
RestrictNamespaces=user mnt pid net ipc uts cgroup
SystemCallFilter=@system-service mount umount2 pivot_root unshare setns clone clone3
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK

[Install]
WantedBy=multi-user.target
```

Confirm it worked — this line is logged at startup:

```text
external command confinement  detail="sandbox bubblewrap (no network, temp-dir-only writes) + memory/PID/file-size/CPU limits"
```

If it instead says `sandbox unavailable (commands will refuse to run)` while a
`scan_cmd` is configured, rela logs a **warning** saying every upload will be
rejected. Under systemd, suspect the unit before the kernel: the sysctls named in
that message (`kernel.unprivileged_userns_clone`,
`kernel.apparmor_restrict_unprivileged_userns`) are usually already correct, and
the three directives above are the actual cause.

The syscalls are listed individually rather than via systemd's `@mount` set.
`@mount` also carries `chroot`, `move_mount`, `open_tree`, `fsopen`, `fsconfig`
and `fsmount`, which bubblewrap does not use — and because the set is
systemd-maintained, its contents can *widen* on an upgrade without the unit
changing. An explicit list can only ever be narrowed by a future systemd, which
fails closed (commands refuse to run) rather than silently granting more.

Tested against systemd 252 (Debian 12).

## Recipes

These are starting points. Test them against your own deployment, pin versions,
and sandbox the tools.

### Virus scanning — ClamAV

Run the ClamAV daemon (`clamd`) and scan over its unix socket:

```yaml
attachments:
  scan_cmd: [clamdscan, --no-summary, --stream, "{in}"]
```

Configuring the command enables scanning for every file property (those that
don't opt out with `scan: off`). `clamdscan` exits non-zero when a signature
matches, which rela maps to a rejected upload.

Verified on Debian 12 (`clamav-daemon` + `bubblewrap`, stock `clamd.conf`): a
clean upload stores normally, an EICAR test file is rejected with 422.

On Windows, point `scan_cmd` at your AV's CLI scanner (e.g. a Microsoft
Defender `MpCmdRun.exe -Scan -ScanType 3 -File {in}` invocation that exits
non-zero on detection).

#### Use `--stream`, not `--fdpass`

**`--fdpass` cannot work under the sandbox.** It hands the open file descriptor
to `clamd` over the socket, and the daemon — in a different mount and user
namespace — rejects what it receives:

```text
/tmp/rela-cmdexec-XXXX/in: Not a regular file ERROR
```

Path-based scanning (neither flag) fails too, for an unrelated reason: rela's
temp directory is `0700` and owned by the server's user, while `clamd` runs as
`clamav` and does the `open()` itself, so it gets `Permission denied`.

`--stream` sends the file's **contents** over the socket. It needs neither
shared filesystem visibility nor descriptor passing, which is why it is the only
transport that works confined.

#### Reaching `clamd` from inside the sandbox

Reaching the daemon takes **two** things, and having only one looks like having
neither.

1. **The socket.** Each command gets its own mount namespace, so the socket path
   is invisible unless rela binds it in.
2. **The config file.** `clamdscan` parses `clamd.conf` at startup to learn where
   `LocalSocket` is — *before* it connects. The sandbox's read allowlist excludes
   `/etc` wholesale (it holds `passwd`, `shadow`, and rela's own config), so
   without an explicit bind the scanner fails with
   `ERROR: Can't parse clamd configuration file /etc/clamav/clamd.conf` and never
   reaches the socket at all.

rela binds the well-known locations of both, read-only:

```text
sockets   /var/run/clamav/clamd.ctl        configs  /etc/clamav/clamd.conf
          /run/clamav/clamd.sock                    /usr/local/etc/clamav/clamd.conf
          /var/run/clamav/clamd.sock
          /tmp/clamd.socket
```

so a stock ClamAV install (Debian/Ubuntu `clamav-daemon`, Homebrew `clamav`)
needs no extra configuration. Missing paths are skipped, so the list is harmless
on hosts without ClamAV.

Binding a socket does **not** re-open network egress — a unix socket is a
filesystem object, and the network namespace stays isolated. `--stream` over a
`LocalSocket` therefore works with no egress at all; only a **TCP** `clamd` on
another host needs it, and the sandbox has no per-command egress opt-in, so
provide that at the deployment layer.

If your `clamd.conf` lives elsewhere, or its `LocalSocket` points outside those
paths, bind the extra paths explicitly:

```yaml
attachments:
  scan_cmd: [clamdscan, --no-summary, --stream, "{in}"]
  scan_sockets: [/opt/clamav/run/clamd.sock, /opt/clamav/etc/clamd.conf]
```

Despite the name, `scan_sockets` binds any read-only path, which is what a
non-default config file needs. Name the config **file**, never its directory:
`/etc/clamav` also holds the signature databases and `freshclam.conf`, which can
carry a `DatabaseMirror` proxy credential.

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
- **Scanning is fail-closed.** Once a `scan_cmd` is configured, uploads to that
  property stop working when the scanner is down — that is the intended safety
  behavior. Set `scan: off` on a property where availability matters more than
  the guarantee.
