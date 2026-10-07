---
id: TKT-AM1IS7
type: ticket
title: Make sandbox read paths operator-configured, per purpose (scan / transform)
kind: enhancement
priority: high
effort: m
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

## Description

On Linux, rela confines external commands (export transforms, attachment scans
and transforms, document commands) with bubblewrap. What a command may read was
a hardcoded list in `internal/cmdexec`: `/usr`, `/bin`, `/sbin`, `/lib*`, plus
`/etc/fonts`, `/etc/alternatives`, `/var/lib/texmf`, `/var/lib/fontconfig`, and
for the attachment runner seven clamd socket paths and three `clamd.conf` paths.
A project could add scanner paths with `attachments.scan_sockets`.

That list is incomplete for real hosts, and the only way to extend it is a rela
release. On Debian 13 every `pandoc --pdf-engine=xelatex` export fails:

```text
xdvipdfmx:fatal: Unrecognized paper format: a4
```

libpaper2 reads paper names from `/etc/paperspecs`, which the sandbox does not
contain. Reproduced on the sourcehaven atlas host (rela 26.10.2): `rela render
--transform=pdf` and the web export (HTTP 500) both fail; the same bwrap
invocation with `--ro-bind /etc/paperspecs` succeeds.

Adding `/etc/paperspecs` to the list would fix this host and leave the next
missing path for the next release. The paths a converter needs depend on the
distro and on which tools the operator installed, so they are host
configuration.

## Change

- Built in: only `/usr`, `/bin`, `/sbin`, `/lib`, `/lib64`, `/lib32`, without
which no command can start.
- Everything else comes from the operator: `RELA_SANDBOX_READ_PATHS`
(PATH-style, `:`-separated), read by the server and the CLI; `rela-server
--sandbox-read-paths` defaults to it.
- `cmdexec.DefaultScannerSockets` and `cmdexec.DefaultScannerConfigs` are
removed.
- `attachments.scan_sockets` is removed. A schema that still sets it fails
validation with a message naming the replacement, instead of being silently
ignored.

This is a breaking change for operators: a host that relied on the built-in
paths must set `RELA_SANDBOX_READ_PATHS` before upgrading.

Decided with the user on 2026-10-06: env var + server flag (same pattern as
`RELA_UNCONFINED_COMMANDS`), one host-wide list, and removing `scan_sockets` in
the same release rather than deprecating it.
