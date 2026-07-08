---
id: BUGA-JVVOSU
type: bug-analysis-checklist
title: 'Analysis: attachments: top-level key rejected by metamodel loader'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

**Steps:** Parse a metamodel containing a valid top-level `attachments:` block
(per `docs/attachment-security.md`), e.g. `attachments: { scan_cmd: [clamdscan,
--no-summary, --fdpass, "{in}"] }`, then `rela validate` / `Parse`.

**Observed:** `SchemaValidationError: unknown key "attachments" (valid keys:
automations, entities, includes, namespace, relations, types, validations,
version)`. Confirmed via a throwaway test against `internal/metamodel.Parse`.
The whole project fails to load.

## Feature-is-live verification (per user challenge)

The `attachments:` block is **not dead config** — both sub-features are wired:

- **`attachments.allow:`** (global MIME allowlist) — enforced on BOTH the CLI and data-entry paths. `internal/attachment/policy.go:50-52` (nil-runner-safe native step).
- **`attachments.scan_cmd:`** (global scan fallback) — enforced on the **data-entry HTTP upload path only**. `internal/dataentry/app.go:568` wires a real `CmdRunner` into `NewPolicyProcessor` at `handlers_attachment.go:302`; `internal/metamodel/attachments.go:103-104` uses the global cmd as the per-property fallback. The **CLI path wires `nil`** (`internal/cli/cli_wiring.go:144`), so `scan_cmd` does not run under `rela attach` (allowlist still does).

Conclusion: the feature works; the loader whitelist is the single broken link.
Fix is correct, not a paper-over.

## Root Cause (5-whys)

- why1: `checkUnknownKeys` (`loader.go:745`) rejects `attachments` — absent from `validTopLevelKeys` (`loader.go:16`).
- why2: Commit `5574ce01` (#1026) added the `Attachments` struct field + validator but not the whitelist entry.
- why3: The whitelist is a hand-maintained duplicate of the struct's top-level yaml tags; they can drift.
- why4: No test loads a full metamodel with a top-level `attachments:` block through the real `Parse()` path. Every attachment test builds `AttachmentsConfig`/`Metamodel` as a **Go struct literal** (`policy_test.go:17`, `newTestAppV1` in `api_v1_test.go:2669`), bypassing the whitelist; or exercises per-property `file` config (already whitelisted). No CLI/e2e for the config path.
- why5 (systemic): allow-list validation is decoupled from the type it validates, and the feature's tests validate behavior via struct literals rather than round-tripping real YAML, so a wiring gap between a live documented feature and its parser is invisible to CI.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Fix:** add `"attachments": true` to `validTopLevelKeys` (`loader.go:16`).

**Tests (preventive measures — the `adds-measure` requirement):**

1. `internal/metamodel/loader_test.go` — `TestParse_AttachmentsTopLevelKeyAccepted`: a real metamodel with a top-level `attachments: { allow, scan_cmd }` block round-trips through `Parse()` with no error AND the parsed `Attachments` field is populated (asserts wiring). **This is the test that actually catches this bug** — the HTTP test below uses struct literals and would pass even with the bug present.
2. `internal/metamodel/loader_test.go` — `TestValidTopLevelKeysMatchStruct`: reflection parity test asserting every top-level `yaml` tag on the `Metamodel` struct is in `validTopLevelKeys`. Closes the systemic drift class (why4/why5) so no future top-level field can silently drift.
3. `internal/dataentry/handlers_attachment_write_test.go` — extend the existing httptest upload pattern (`newTestAppV1` + `multipartBody`) to prove the **global** `attachments: { allow, scan_cmd }` config is enforced end-to-end through the real HTTP handler: a disallowed MIME is rejected (422) and a stub shell `scan_cmd` that exits non-zero rejects the upload. This is the established data-entry test pattern (httptest handler integration, used across 20+ files), chosen over a new CLI/Playwright harness.

**Related areas checked:** `attachments` is the only top-level struct field
missing from the whitelist (verified against the struct's yaml tags). No other
drift.
