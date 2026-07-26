---
id: DOCS-P7CJLR
type: docs-checklist
title: 'Docs: lua: elevated reads on rela.bypass_acl''s admin handle'
status: done
---

## Code Documentation

- [x] Godoc on every new exported symbol (`lua.ElevationRecorder`,
`script.ReadElevation`, `script.NewLuaScriptRunnerWithElevatedReads`,
`audit.ElevationRecorder`, `audit.NewElevationRecorder`,
`audit.OpACLBypassRead`, `appbuild.NewElevationAuditor`)
- [x] Godoc explains the *why*, not just the *what* — each of the
load-bearing decisions carries its rationale at the declaration site:
  - `WriteDeps.ElevatedReader`: why it is SEPARATE from `WritePrepStore`
(which exists on every writer runtime, so reusing it would dissolve the two-key
gate)
  - `WriteDeps.ElevationRecorder`: why once-per-closure and not per-read
  - `newElevatedHandle`: why a nil reader leaves the methods PRESENT and
RAISING rather than absent
  - `registerElevatedReads`: why it deliberately does NOT share code with
the gated bindings
  - `appbuild.NewElevationAuditor`: the typed-nil-in-interface trap
- [x] Comments explain non-obvious decisions (the `defer` that records the
audit row on the raise path; marking the binding used BEFORE the read)

## Project Documentation

- [x] `docs-project/entities/guides/GUIDE-lua-scripting.md` — retitled the
section "Elevated writes" → "Elevated access"; added the three read methods to
the `admin` method table; documented that elevated reads return raw data; added
a worked example (cross-entity uniqueness invariant); documented the deliberate
difference in what `nil` means between `rela.get_entity` (missing OR hidden —
oracle-free) and `admin.get_entity` (genuinely absent); extended the
safety-properties list with the audit row, closure-scoping for reads, and the
fail-loud-not-degraded contract.
- [x] `docs-project/entities/guides/GUIDE-acl-security.md` — added the
sanctioned escape hatch to the script-reads section: when an automation needs
more than its caller can see, use `rela.bypass_acl` rather than widening a role
in `acl.yaml` (scoped to a closure, visible at the call site, leaves an
`acl-bypass-read` row — where a widened role would grant access on every path
with nothing in the log).
- [x] Generated docs regenerated via `just docs`; `docs/lua-scripting.md`
and `docs/acl-security.md` updated and committed. Edited the SOURCES in
`docs-project/`, not the generated output.
- [x] `just lint-md` passes (245 files, 0 issues)
- [x] The pre-push docs gate (`git diff --exit-code docs/ README.md`)
confirms generated output is in sync.

## External Documentation

- [x] ~~API reference updated~~ (N/A: no HTTP API surface changed — this is
a Lua binding on an operator-gated automation path)
- [x] ~~Migration guide~~ (N/A: purely additive. Existing `bypass_acl`
closures keep working unchanged; the new methods appear on a handle that only
exists when an operator has already set `allow_acl_bypass: true`)
- [x] ~~CHANGELOG~~ (N/A: this project does not maintain one)

## Operator-facing notes

- [x] The new audit op `acl-bypass-read` is documented in
`internal/audit/audit.go` (wire-contract constant block, alongside `acl-bypass`)
AND in the ACL/Lua guides, including the deliberate choice to record NO subject
and NO entity data, and why folding it into the existing `acl-bypass` op would
have silently changed what every existing `op == "acl-bypass"` query means.
