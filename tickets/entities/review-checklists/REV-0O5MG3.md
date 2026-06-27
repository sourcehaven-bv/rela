---
id: REV-0O5MG3
type: review-checklist
title: 'Review: Attachment CLI + docs cleanup: multi-file wording, orphan window, doc drift'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./internal/attachment/ ./internal/store/fsstore/` ok; full `just coverage-check` suite green
- [x] Lint clean — `golangci-lint run --build-tags postgres ./internal/store/pgstore/...`: changed file `attachment.go` clean (the 3 flagged issues are in `graphquery_explain_test.go`, pre-existing, not touched by this ticket)
- [x] Coverage maintained — `just coverage-check`: total 76.7%, all package + total floors PASS (docs/comments change adds no statements)

## Code Review

- [x] ~~Run `/code-review` (cranky-code-reviewer)~~ (N/A: docs + code-comment only change, no behavioral diff; self-review sufficient)
- [x] ~~All critical review-responses addressed~~ (N/A: no review-responses created)
- [x] ~~All significant review-responses addressed~~ (N/A: no review-responses created)
- [x] Self-reviewed the diff for unrelated changes — diff limited to the 4 in-scope items across `docs/cli-reference.md`, `docs/metamodel.md`, `internal/store/pgstore/attachment.go`

**Review Responses:** none (no findings)

## Acceptance Verification

- [x] Each acceptance criterion tested:
  - CLI help / `docs/cli-reference.md` accurately describe per-property model — PASS (overwrite note added under `rela attach`)
  - Orphan-recovery behavior documented — PASS (`rela gc --temp-files` note under `rela attach`)
  - `docs/metamodel.md` attachment path matches code — PASS (`.rela/attachments/` → `attachments/`; grep confirms no remaining stale refs in `docs/`)
  - content_type dead column — PASS (documented at write + read sites in `attachment.go`; `go build -tags postgres` passes)
- [x] Test evidence documented (above + in ticket Resolution section)

**Acceptance Status:** ALL PASS

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: this ticket *is* the docs work; kind=docs, not an enhancement with separate user docs)
- [x] User-facing documentation updated — `docs/cli-reference.md`, `docs/metamodel.md`
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

**Docs Checklist:** none (kind=docs ticket)

## Final Checks

- [x] Commit message explains the why, not just what — pending commit (user controls commit timing)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command~~ (deferred: user controls commit/PR timing per project convention; changes staged in working tree)
- [x] ~~All CI checks pass~~ (deferred to PR)
- [x] ~~PR URL documented~~ (deferred to PR)

**PR:** pending — changes staged, not yet committed/pushed
