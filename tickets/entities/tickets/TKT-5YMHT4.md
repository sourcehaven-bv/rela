---
id: TKT-5YMHT4
type: ticket
title: Surface the legacy-schema deprecation notice in the desktop UI
kind: enhancement
priority: low
effort: s
status: backlog
---

Deferred from TKT-FNARO6 code review (RR-K2ELC7).

`project.WarnIfLegacySchema` writes the `metamodel.yaml` deprecation notice to
stderr. That reaches CLI and server operators, but **nobody reads stderr from a
packaged macOS `.app` bundle** — so for desktop-only operators the notice is
effectively invisible.

That matters because the one-shot warning is the entire exit strategy for the
backward-compatible dual-name support: when the legacy name is eventually
dropped at a major version, a desktop-only user will have had no warning at all.

**Suggested approach:** the welcome screen (`cmd/rela-desktop/welcome.go`)
already describes the expected project layout and was updated to say
`schema.yaml`. A banner on project open — "this project uses the old
`metamodel.yaml` name; run `rela migrate`" — is the natural place. The state is
already available: `project.Context.SchemaIsLegacy` is set by discovery, and
`discoverProject` (`main.go`) already has the Context in hand.

Note the warning is now deduplicated **per project root** (`sync.Map`), not per
process, so a desktop session that opens several legacy projects gets a notice
for each — the UI surface should follow the same rule.
