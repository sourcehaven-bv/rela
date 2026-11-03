---
id: REV-MCPCNV
type: review-checklist
title: 'Review: MCP convert_entity and list_conversions tools'
started: "2026-10-24"
status: done
---

## Automated Checks

- [x] Tests pass: `go test ./internal/mcp/... ./internal/dataentry/... ./internal/dataentrywire/... ./internal/cli/...`
- [x] Lint clean: `golangci-lint run` on the changed packages, 0 issues

## Code Review

- [x] Self-reviewed the diff for unrelated changes
- [x] ACL goes through `entitymanager.Converter`; no check is reimplemented

## Acceptance Verification

- [x] Each acceptance criterion tested: `TestListConversions`, `TestConvertEntity_Flow`, `TestConvertEntity_ACL`, `TestConversionTools_Registration`

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: see DOCS-MCPCNV
- [x] Docs-checklist marked as done

## Final Checks

- [x] No TODOs or FIXMEs left unaddressed
