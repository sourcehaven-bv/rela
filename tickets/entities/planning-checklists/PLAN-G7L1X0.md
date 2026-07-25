---
id: PLAN-G7L1X0
type: planning-checklist
title: 'Planning: acl: migrate a scheduler read grant into acl.yaml (new FileTypeACL + runner create-path + operator notice)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Premise correction (verified before planning):** reads ALREADY fail closed —
`acl.readQuery` returns `DenyAll` when no role confers read
(`readquery.go:49-50`). So this ticket **repairs a regression we shipped in
TKT-ZF2DTV**, it does not harden a default. Any deployment that has an
`acl.yaml` and runs scheduled tasks has had those tasks silently reading nothing
since it merged.

**Scope correction (RR-SVQ5HE, accepted 2026-07-25):** the create-when-absent
path is **dropped**. Verified by probe that `scriptEntityReader` returns the raw
store when no policy exists (`appbuild.go:254`, `if d == nil { return st }`) —
so a project without `acl.yaml` has **no regression to repair**, while creating
one would flip *every* principal to deny-by-default. Net-harmful; removed.

**Scope:**

IN: `migration.FileTypeACL`; a migration adding a scheduler read grant to an
**existing** `acl.yaml`; wiring that file type into the migrate file list; an
operator-visible description naming the symptom; tests.

OUT: **creating `acl.yaml` when absent** (RR-SVQ5HE); per-job role authoring UX;
changing `run_as` semantics; the data-entry fail-open fallback (rela#1198); the
NopACL branches (`d == nil` → raw store is correct parity, not a bypass).

**Acceptance Criteria:**

1. An `acl.yaml` lacking a scheduler assignment gets the grant added; the rest of the file — comments and formatting included — is preserved.
2. An `acl.yaml` that already has the assignment is left untouched (`Detect` false, idempotent).
3. A project with **no** `acl.yaml` is left alone — no file created, no error, no notice. It is already correct.
4. A malformed `acl.yaml` fails the migration without clobbering the file.
5. The migrated policy passes `acl.Policy.Validate` / `LoadPolicy` — a broken generated file would block boot (`appbuild.go:551`).
6. Post-migration, a scheduled task under `system:scheduler` reads again; a task under an unassigned `run_as` still reads nothing.

## Research

- [x] Full survey of the migration framework, runner, callers, acl.yaml loading and validation taken before planning
- [x] Existing patterns identified and will be followed
- [x] Reference implementation read end-to-end (`short_id_default.go`)

**Survey findings that shape the work:**

1. **Migrations are `yaml.Node` transforms registered via `init()`** — one new file in `internal/migration/` with `Register(&…{})`. `Detect`/`Apply` take the *document* node; every migration starts with `GetDocumentRoot`. `InsertMapKeyAfter` is the only helper accepting a composite value node, which is what a `roles:`/`read:` subtree needs.
2. **Skip-on-missing is now exactly right.** `projectsetup/migrate.go:64` and `:106` `continue` when `fs.Stat` fails. With AC3 dropped this is the *desired* behavior, not a blocker — a missing `acl.yaml` must stay missing. No change needed there; adding the file to `getMigrateFiles` is sufficient and safe.
3. **`Description()` is the only operator-visible per-migration string** (printed at `cli/migrate.go:39`, `:71`). `migration.Result`/`MigrateFileResult` carry no notice field. Since the change is now narrow and non-destructive, `Description()` alone is adequate — no new plumbing.
4. **`Policy.Validate` enforces `update ⊆ read` / `delete ⊆ read`** (`policy.go:576-593`) — satisfied here since the grant is read-only, but a policy that violates it hard-fails `LoadPolicy` and blocks boot, so it must be test-pinned.
5. Exact shapes: `roles:` is `map[string]RoleDef`; `assignments:` is `map[string]string` (principal → **one** role name, scalar); `RoleDef.Read` is `[]string` with `"*"` wildcard.
6. No production code has ever written `acl.yaml`, and `rela init` doesn't scaffold one. This migration edits only files an operator authored deliberately.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**The grant** (read-only, so `Validate` passes):

```yaml
roles:
  scheduler-system:
    read: ["*"]
assignments:
  system:scheduler: scheduler-system
```

`read: ["*"]` restores the pre-TKT-ZF2DTV behavior exactly — scheduled tasks
previously read the whole graph. Narrowing per-deployment is the operator's job,
and `run_as` exists for that.

**Migration** — new `internal/migration/acl_scheduler_grant.go`, registered via
`init()`, `FileTypes: []FileType{FileTypeACL}`. `Detect` → true when
`assignments` lacks a `system:scheduler` key. `Apply` → upsert the role and the
assignment via `GetMapValue`/`InsertMapKeyAfter`, creating the
`roles:`/`assignments:` mappings when absent *within an existing file*.
Idempotent by construction (AC2).

**Wiring** — add `FileTypeACL` to `migration.FileType` and an entry to
`projectsetup.getMigrateFiles`. The existing skip-on-missing loop then gives AC3
for free.

**Operator notice** — `Description()` names the symptom, since it is the string
that reaches the operator: *"add a scheduler read grant to acl.yaml — scheduled
tasks read nothing without it"*.

**Alternatives rejected:** creating the file when absent (RR-SVQ5HE — breaks
every principal, repairs nothing); adding a create-path to `migration.Apply`
(would let every migration conjure files; the package's contract is
read-transform-write); writing the grant from `appbuild` at boot (silently
mutating a security policy at runtime is worse than an explicit `rela migrate`).

**Files:** `internal/migration/{migration.go (FileTypeACL),
acl_scheduler_grant.go (new), acl_scheduler_grant_test.go (new)}`;
`internal/projectsetup/migrate.go` (file list only);
`docs-project/entities/guides/` for the operator note.

## Security Considerations

- [x] Input sources identified — operator-run command against project-local files; no external input
- [x] Validation approach — migrated policy must pass `Policy.Validate`, pinned by a `LoadPolicy` round-trip test
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak

- **The dominant risk was the create-path; it has been removed** (RR-SVQ5HE). What remains only edits files an operator wrote deliberately, and only widens one system identity's reads back to what they were before the regression.
- **The grant is a privilege grant** and must therefore be minimal in kind: `read: ["*"]` with **no write verbs**. A scheduled task's writes keep going through entitymanager's own ACL.
- **A malformed existing file fails without clobbering** (AC4) — the runner's encode/write runs only after a successful parse.
- **A generated file that fails `Validate` would block boot** — pinned by test (AC5).
- No secret material is read or written.

## Test Plan

- [x] Scenarios per AC; [x] edge cases; [x] negative tests; [x] integration approach

Following the package's established style (inline YAML string literals, no
fixtures dir):

- **Detect table**: no assignments block / assignments without the key / key already present (false) / empty file.
- **Apply table**: assert on the mutated node tree, then re-assert `Detect(&doc) == false` for idempotence (the package's own convention).
- **Comment preservation**: `yaml.Marshal(&doc)` + `strings.Contains` on a comment and on the new keys — the `id_prefix_rename_test.go` pattern.
- **Validity**: feed migrated YAML to `acl.LoadPolicy` and assert it parses and validates (AC5 — the check that prevents shipping a boot-blocking file).
- **Runner integration**: real temp file via `storage.NewOsFS()`, `Apply`, read back, `strings.Contains`.
- **Absent file** (AC3): `projectsetup.MigrateWithFS` on a project with no `acl.yaml` → no file created, no error.
- **Negative**: malformed YAML → error, original bytes unchanged on disk.
- **End-to-end** (AC6): scheduled task with the grant reads; with an unassigned `run_as`, reads nothing.

## Risk Assessment

- [x] Risks + mitigations; [x] security risks; [x] effort (s — reduced from m after RR-SVQ5HE)

- **Widening `system:scheduler` to `read: ["*"]`** — restores prior behavior exactly; the alternative (guessing a narrower set) would silently break working jobs.
- **Generated file blocks boot** if it fails `Validate` — mitigated by the `LoadPolicy` round-trip test.
- **An operator who deliberately removed the scheduler assignment** would have it re-added by `Detect`. Acceptable: `run_as` is the supported way to scope a task, and the migration runs once.
- No changes to `internal/migration`'s contract or to the runner; `projectsetup` gains one list entry.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created at review

**Impact:** `docs-project/entities/guides/` scheduled-tasks guide (why a
policy-configured project needs the grant, beside the existing `run_as` section;
and that policy-less projects need do nothing) → regenerates
`docs/scheduled-tasks.md`. Note: `docs/*.md` are GENERATED — edit the source
entities.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-SVQ5HE (critical, addressed — create-path
dropped)
