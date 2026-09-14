---
id: DOCS-ECEGAR
type: docs-checklist
title: 'Docs: ClamAV attachment scanning recipe + hardened systemd unit'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have godoc
- [x] Non-obvious decisions explain WHY

`DefaultScannerConfigs` states why a config file needs binding at all (clamdscan
parses `clamd.conf` *before* it connects, and `readOnlyPaths` excludes `/etc`
wholesale), why entries are files and never the `/etc/clamav` directory
(signature databases plus `freshclam.conf`, which can carry a `DatabaseMirror`
proxy credential), and — added in review — the exposure scope: binds are
per-runner, and `internal/attachment` shares one runner between scans and
attachment `cmd:` transforms, so exiftool/qpdf see these files too, while export
and document rendering build their own unbound runners.

`SandboxErr` was reworded in review so its contract leads with the trap: nil
does NOT mean "confined" (it is also nil on operator opt-out), and the
prohibition names the real risk — skipping a *scan* on a non-nil value converts
a fail-closed rejection into an unscanned upload.

`HasConfiguredScan` documents why it is deliberately not the negation of
`HasUnconfiguredScan`, and why a nil metamodel returns false rather than
panicking (a startup diagnostic must not be able to take the server down).

## Project Documentation

- [x] User-facing guide updated
- [x] Generated docs regenerated and consistent

`docs-project/entities/guides/GUIDE-attachment-security.md` is the source;
`docs/attachment-security.md` is generated from it by `just docs` and was
regenerated after every edit. Verified consistent — the systemd unit extracted
from the *generated* copy was the one deployed for final verification.

Changes:

1. **Recipe** — `--fdpass` → `--stream` in both the ClamAV recipe and the
Configuration example.
2. **New subsection "Use `--stream`, not `--fdpass`"** — explains that `--fdpass`
cannot work under bwrap (clamd `fstat`s the passed descriptor across the
namespace boundary) and that path-based scanning fails separately because rela's
temp dir is `0700` and clamd runs as another user.
3. **Corrected two false claims** — the stock install genuinely does need the
config bind (now provided by default), and `--stream` over a `LocalSocket` needs
no network egress; only a TCP `clamd` on another host does.
4. **Rewrote "Reaching `clamd` from inside the sandbox"** — reaching the daemon
takes *two* things (socket + config), and having one looks like having neither.
Both default lists are shown.
5. **New section "Running under systemd"** — the three directives that silently
break the sandbox, with symptoms and fixes, plus a complete unit rated
`systemd-analyze security` 2.4 OK, and how to confirm from the startup log.
6. **Review follow-up** — `@mount` replaced with the explicit syscalls bwrap
issues (`mount umount2 pivot_root`), with the rationale that a
systemd-maintained set can *widen* on upgrade whereas an explicit list can only
narrow, which fails closed.

Also noted in the guide: `scan_sockets` binds any read-only path despite its
name, which is what a non-default config file needs.

## External Documentation

- [x] ~~API reference~~ (N/A: no API surface change — no new endpoint, request
or response field)
- [x] ~~Migration notes~~ (N/A: no config change is required of existing
operators. The old `--fdpass` recipe never worked confined, so anyone running it
has a broken scanner today; adopting `--stream` fixes it. The
previously-required `scan_sockets: [/etc/clamav/clamd.conf]` workaround keeps
working — the default binds are additive.)
- [x] ~~Changelog~~ (N/A: not maintained in this repo)

## Verification

- [x] Documented steps actually work

The guide's recipe and its systemd unit were both applied verbatim in a Debian
12.15 VM — the unit extracted programmatically from the generated
`docs/attachment-security.md`, not retyped — against a real `clamd` and the
final `rela-server` binary: clean upload → 200, EICAR → 422, startup log
`sandbox bubblewrap …`, `systemd-analyze security` 2.4 OK, and no
`scan_sockets:` key present in the project schema.
