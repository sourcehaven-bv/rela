---
id: REV-UIWKBO
type: review-checklist
title: 'Review: Rename metamodel.yaml to schema.yaml with backward-compatible dual-name discovery'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` passes
- [x] `just lint` clean (0 issues)
- [x] `just coverage-check` passes
- [x] `just arch-lint` clean
- [x] `just plimsoll` clean
- [x] `just lint-md` clean (0 issues in 246 files)
- [x] `just docs` regenerated; diff is rename-only plus the deprecation note

Final run after review fixes: tests green, lint 0 issues, coverage 77.1% (PASS).

## Code Review

- [x] `/code-review` run (cranky-code-reviewer)
- [x] All critical findings addressed
- [x] All significant findings addressed

Six findings, all verified against the code before accepting — two critical:

| ID | Severity | Outcome |
|---|---|---|
| RR-KUSIQW | critical | **Real bug shipped in the first commit.** `internal/schema/` was skipped by the sweep entirely; `ExecuteCleanup` rebuilt `filepath.Join(projectRoot, "metamodel.yaml")`, so `analyze schema --cleanup` failed on *every* new-name project. Fixed by passing the resolved path, not a root. |
| RR-URUZIS | critical | The both-files refusal was **unreachable dead code** (`SchemaIsLegacy` implies `schema.yaml` is absent), and the docs I wrote promised a safety property that did not exist. Replaced with orphan detection + reporting; docs corrected. |
| RR-66X91V | significant | Tautological test replaced with real assertions. |
| RR-K2ELC7 | significant | Process-wide `sync.Once` → per-root `sync.Map`. |
| RR-C0UN1K | significant | Misleading doc on `Context.Exists` rewritten. |
| RR-0JTI3W | significant | `dataentry` FS bypass + per-request stat removed. |

Minor/nit findings actioned rather than deferred: e2e and demo fixtures flipped
to `schema.yaml` (they were exercising *only* the legacy path — backwards for a
rename whose new name is canonical), and `--check`'s double project discovery
collapsed into a single walk via `CheckPending`.

Deferred with a ticket: **TKT-5YMHT4** — desktop stderr is unreadable in a
packaged `.app`, so the deprecation notice needs UI surfacing. Out of scope
here, but not dropped.

## Acceptance Verification

- [x] Each acceptance criterion verified with evidence

Re-verified end-to-end with a rebuilt binary after the review fixes:

| AC | Result |
|---|---|
| 1 | fresh project loads, no warning — PASS |
| 2 | legacy project loads, warns once naming the root and `rela migrate` — PASS |
| 3 | real `schema.yaml` + 0-type decoy `metamodel.yaml` → **24 types** loaded, decoy ignored — PASS |
| 4 | `.rela`-only / no-project → error names `schema.yaml` — PASS |
| 5 | `rename-type` rewrites the legacy file in place, no stray `schema.yaml` — PASS |
| 6 | `migrate --check` flags the rename and **exits 1**; `migrate` prints `Renamed metamodel.yaml → schema.yaml`; content migrations still applied — PASS |
| 7 | `init` creates `schema.yaml`; refuses in a legacy dir **and writes nothing** — PASS |
| 8 | MCP lists both `get_schema` and `get_metamodel`; the alias returns real data — PASS |
| 9 | warning once per project, from the shared startup path — PASS |

Post-fix additions verified directly:

- `analyze schema --cleanup` on a `schema.yaml` project: dry-run reports `schema.yaml: remove_entity_type ...` and the **apply path now succeeds** (previously errored naming a nonexistent file).
- Orphaned `metamodel.yaml` beside a live `schema.yaml`: `migrate --check` reports it and exits 1 (confirmed exit=1); `migrate` reports it; **the operator's file is preserved, not deleted**.

## Pre-merge

- [x] Branch: `feat/schema-yaml-rename`
- [ ] PR created
- [ ] CI green

PR pending — see `/pr`.
