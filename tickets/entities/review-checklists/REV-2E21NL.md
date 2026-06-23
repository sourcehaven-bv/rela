---
id: REV-2E21NL
type: review-checklist
title: 'Review: Configurable per-property attachment count: file property `max` setting (1..N)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./...` exit 0; frontend `npm run test:run` 1089 passed
- [x] Lint clean — `golangci-lint ./internal/...` 0; `vue-tsc` clean; frontend lint 0 errors; `just arch-lint` OK
- [x] Coverage maintained — `just coverage-check` PASS
- [x] Build default + postgres OK

## Code Review

Two review rounds. Round 1 (per-ticket): caught the max==1 delete-before-attach
data-loss bug + policy duplication. Round 2 (`/code-review`, integrated final
pass): caught a **critical read-path ACL bypass** that only became reachable
once `_attachments` joined the response.

- [x] All critical addressed — RR-N96YV0 (data-loss reorder), **RR-ROF51F (hidden-property leak: skip in computeAttachments + 404 in GET/preflight)**
- [x] All significant addressed — RR-CIH3TZ (consolidate), RR-P5EFFT (capture-state-once), RR-BN2MDO (`.new` temp-marker collision)
- [x] Self-reviewed the diff

**Review Responses:** RR-N96YV0, RR-CIH3TZ, RR-1BS5FH (round 1) + RR-ROF51F
(critical), RR-P5EFFT, RR-BN2MDO, RR-ZR1FYI (round 2) — all `addressed`.

## Acceptance Verification

- [x] Each criterion tested + verified live; round-2 fixes added these tests:
  - **Hidden file property** → omitted from `_attachments` AND per-file download 404s (`TestAttachment_HiddenPropertyNotLeaked`)
  - **`*.new`-named attachment** survives a store reopen (`TestPersistence_AttachmentsSurviveReopen` + storetest `FileNameEndingInNewRoundTrips` all backends)
  - **schema `max`** emitted by both `/_schema` and `/_schema/types/{type}` (`TestV1SchemaEndpoint` + `TestV1SchemaTypesSpecific`)
  - Snapshot-once threading; one-fewer ListAttachments per upload
  - Multi-file gallery + single-cap replace re-verified live (non-regressive)
- [x] Test evidence documented

**Acceptance Status:** ALL PASS

## Documentation (enhancements only)

- [x] Docs-checklist created + linked via `has-docs` — DOCS-372ZRT
- [x] User-facing docs updated — metamodel.md, data-entry/api-reference.md, cli-reference.md
- [x] Docs-checklist marked done — DOCS-372ZRT

**Docs Checklist:** DOCS-372ZRT

## Final Checks

- [x] No TODOs/FIXMEs left; reverted the stray catalog/metamodel.yaml leftover; ready for another developer

## Pull Request

- [x] ~~/pr~~ (deferred: user controls commit/PR timing; full local CI surrogate green)

**PR:** pending — changes staged, not yet committed/pushed
