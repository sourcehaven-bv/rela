---
id: DOCS-SKMBCW
type: docs-checklist
title: 'Documentation: Command exec ungated under default NopACL: refuse on non-loopback bind, with an explicit override flag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the `SelectCommandAuthorizer` godoc
carries the full decision matrix, and the `commandAuthorizer` godoc states the
trap the seam exists to avoid: `readGateFromContext` returns the permissive
`nopReadGate` under BOTH NopACL and ReadOnlyACL, so a guard written against the
ctx read gate alone fails OPEN. That reasoning lives at the seam rather than
only in a ticket. `gatedAuthorizer.d` has an explicit "do not drop this field"
note explaining it exists so the type cannot be constructed without a policy.
- [x] Function/type docs if public API — `SelectCommandAuthorizer`,
`UngatedCommandAuthorizer` and the new `CommandAuthNotifier` are all documented;
`CommandAuthNotifier`'s doc says why BOTH hooks are required (instrumenting only
the granting path leaves the denying path — the one that changes behavior on
upgrade — silent).

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-visible surface that the README covers;
it does not document server flags or the ACL model.)
- [x] ~~CLAUDE.md updated~~ (N/A: introduces no new pattern. The change applies
an existing documented one — the two-impl-at-the-wiring-seam shape already
described for `scriptEntityReader` / `DenyReader` — rather than establishing a
new rule.)
- [x] Help text accurate — `--allow-unauthenticated-commands` help text names
the risk, states when the opt-in is legitimate (single-user deployment isolated
by Docker port-publishing, a host firewall, or an authenticating reverse proxy),
and points at `docs/server-security.md`. Modeled on the existing
`--unconfined-commands` text, which does the same thing for the sandbox
dimension of the same shell-exec surface.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repository keeps no CHANGELOG file;
release notes are derived from commit messages, and the commit explains the
pre-existing fail-open and the new default.)
- [x] API docs updated — the operator-facing security documentation is the API
surface that changed here. `GUIDE-server-security` gains the mode/bind decision
table, the `--allow-unauthenticated-commands` guidance, and an upgrade note for
deployments that relied on the old ungated default; `GUIDE-acl-security` gains
the matching "authorization depends on the policy AND the bind" section,
replacing the now-wrong "authorization is bimodal" framing.

**Edited at the source, not the output.** `docs/server-security.md` and
`docs/acl-security.md` are GENERATED from the `docs-project/` guide entities by
`scripts/generate-docs.sh`; the Docs CI job regenerates and fails on any diff.
An earlier revision of this ticket hand-edited the generated file, which CI
correctly caught as out-of-date — the prose now lives in
`docs-project/entities/guides/GUIDE-server-security.md` and
`GUIDE-acl-security.md`, with the generated files reproduced from them.
