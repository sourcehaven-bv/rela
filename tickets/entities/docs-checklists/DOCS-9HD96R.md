---
id: DOCS-9HD96R
type: docs-checklist
title: 'Docs: Reject reserved system:* principals at the API boundary (system:scheduler impersonation)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have godoc — `principal.ReservedPrefix` and
`principal.IsReserved` both carry doc comments explaining *why* the namespace is
reserved (the names are grantable in `acl.yaml` and the ACL matches them by raw
string, so it cannot tell a forged `system:scheduler` from the real one), not
merely what the function does.
- [x] Non-obvious decisions documented at the call site:
  - `sanitizeUser` carries an explicit "the check does NOT go here, and here's
why" note — it is the one function every HTTP username source passes through, so
the next reader would otherwise assume it covers this. It can only signal
"unusable" by returning `""`, which every caller treats as fall-through, i.e. a
silent demotion.
  - `stampAuditPrincipal` documents the `/api/`-vs-elsewhere split against the
existing RR-T15E constraint.
  - `rejectReason` documents the drift it prevents, with the concrete bug.
  - `trimReservedNoise` documents why only the *leading* run is stripped.
- [x] ~~New package documentation~~ (N/A: no new package)

## Project Documentation

- [x] `docs/server-security.md` — new "Reserved `system:` identities" section:
the rule, the per-source table, the off-API degrade behaviour, the
case-sensitivity note, and an **upgrade note** for deployments whose IdP issues
`system:`-shaped subjects (they will be locked out; grep the log for `rejected
reserved principal`).
- [x] `docs/scheduled-tasks.md` — the scheduler grant cannot be borrowed over
the API; `run_as:` is unaffected because it is operator-authored config read
in-process.
- [x] `docs/acl-security.md` — the same boundary guarantee stated beside the
`system:provisioner` grant, plus why a forged subject can never be provisioned
into a stub's `principal_property`.
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `docs/data-entry.md`~~
(N/A: no metamodel, command, or UI surface changed)
- [x] ~~`CLAUDE.md`~~ (N/A: the rule lives in the `IsReserved` godoc, which is
where someone about to add an entry point will actually encounter it)

**Important:** these three files carry an `auto-generated from
docs-project/entities/` banner. The edits were made in the **source**
(`docs-project/entities/guides/GUIDE-*.md`) and regenerated with
`scripts/generate-docs.sh`; editing `docs/*.md` directly would have been
silently reverted by the next regeneration. `git status` confirms only the three
intended generated files changed.

## External Documentation

- [x] ~~README~~ (N/A: no user-facing feature or install-level change)
- [x] ~~CHANGELOG~~ (N/A: not maintained in this repo)
- [x] Migration/upgrade impact captured — the lockout risk is documented as a
blockquoted upgrade note in the server-security guide, which is where an
operator hitting it will look.
