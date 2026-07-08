---
id: IMPL-EKKU9N
type: implementation-checklist
title: 'Implementation: attachments: top-level key rejected by metamodel loader'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (loader round-trip + reflection parity)
- [x] Integration tests written (HTTP upload through the real handler for global allow: + scan_cmd:)
- [x] Happy path implemented (`"attachments": true` in validTopLevelKeys)
- [x] Edge cases from planning handled (global-config fall-through: property sets neither Accept nor ScanCmd)
- [x] ~~Error handling in place~~ (N/A: additive whitelist entry; no new error paths)

## Test Quality

- [x] Using fixture builders (`newAppFromParts`/`newGlobalAttachmentsApp`, `multipartBody`, existing writeACL/seedEntity helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: fixed fixture IDs, matches package convention)
- [x] Property comparisons use returned bytes / codes, not incidental strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- **AC1 — `rela validate` accepts the block:** built `cmd/rela`, ran `rela --project <tmp> validate` against a project whose `metamodel.yaml` has `attachments: { allow: [image/png, text/plain], scan_cmd: [clamdscan, ...] }`. Result: `✓ metamodel.yaml is valid`, exit 0. Before the fix this failed with `unknown key "attachments"`.
- **AC2 — tests catch the bug:** temporarily reverted the one-line fix; `TestParse_AttachmentsTopLevelKeyAccepted` and `TestValidTopLevelKeysMatchStruct` both FAILED with the exact `unknown key "attachments"` error. Restored fix → both pass.
- **AC3 — parity test guards the class:** `TestValidTopLevelKeysMatchStruct` fails if any top-level `Metamodel` yaml tag is missing from `validTopLevelKeys` (verified by the revert above).
- **AC4 — global allow: enforced end-to-end:** `TestAttachmentUpload_GlobalAllowlistEnforced` — text upload → 200, PNG upload → 422, through the real HTTP handler with no per-property Accept.
- **AC5 — global scan_cmd: enforced end-to-end:** `TestAttachmentUpload_GlobalScanCmdRejects` — clean text → 200, "INFECTED" text → 422; rejected upload does not overwrite the clean attachment. Proved the *scan* (not the allowlist) is the rejecting mechanism by re-running with `attachmentRunner=nil` → infected file then passes.

**Automated checks:** `go build ./...` OK; `go test ./...` all pass (one
transient `internal/storage` fsnotify watcher-timeout flake, unrelated to
changed packages — passes 3/3 in isolation); `golangci-lint run` on changed
packages → 0 issues; `just coverage-check` → thresholds PASS (metamodel 82.6%,
dataentry 79.9%, attachment 72.3%).

## Quality

- [x] Code follows project patterns (extended existing httptest attachment-upload pattern; table-free single-purpose tests like siblings; modern `reflect.TypeFor`/`Type.Fields()` idiom)
- [x] Checked for DRY (`newGlobalAttachmentsApp` factors the shared app+runner+ACL+seed setup for the two HTTP subtests; reused `multipartBody`/`putAttachmentAs`/`writeACL`/`seedEntity`)
- [x] No security issues introduced (whitelist entry only *enables* a validation path that already exists; scan stub uses array-arg exec, no shell string)
- [x] No silent failures
- [x] No debug code left behind (throwaway repro tests removed)
