---
id: DOCS-BOWIP5
type: docs-checklist
title: 'Docs: Make sandbox read paths operator-configured (RELA_SANDBOX_READ_PATHS)'
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `unsafeReadPath` (why each refused
directory, why symlinks are resolved), `validateSpec` (bind order vs the
writable dir), `hasKey` (merge keys), `warnIfNoSandboxReadPaths` (once per
process, when it stays quiet).
- [x] Function/type docs if public API: `EnvSandboxReadPaths`,
`EnvUnconfinedCommands`, `ApplyHostEnv`, `SetHostReadOnly` (readable and
connectable exposure), `ParseReadPaths`, `HostReadOnly`,
`CheckProjectNotExposed`, `UnconfinedByDefault`, `WithExtraReadOnly`,
`Spec.ExtraReadOnly`.

## Project Documentation

- [x] ~~README updated~~ (N/A: the README does not cover sandbox configuration)
- [x] ~~CLAUDE.md updated~~ (N/A: no new contributor pattern; cmdexec rules there still hold)
- [x] Help text accurate (if CLI changes): `rela-server --sandbox-read-paths`
help checked with `--help`; it states the default, the exposure and to list
files and socket files, never `/run`.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable): `docs/transforms.md` "Sandbox read
paths" (Debian 13 xelatex list, per-process note, fontconfig and alternatives
caveats, readable and connectable exposure, refused paths, project check, `/tmp`
socket warning, upgrade note with the old list) and the controls table;
`GUIDE-attachment-security.md` (regenerated `docs/attachment-security.md`): unit
sets the variable, clamd section, converters now reach the clamd socket,
`launchctl setenv` for the desktop, upgrade note for `scan_sockets`.
