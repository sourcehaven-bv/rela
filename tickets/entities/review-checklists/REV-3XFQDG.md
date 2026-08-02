---
id: REV-3XFQDG
type: review-checklist
title: 'Review: extract views cluster to viewsHandler (TKT-I37338)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./...` — full suite)
- [x] Lint clean (`golangci-lint run ./internal/dataentry/`) — 0 issues
- [x] `gofmt` clean
- [x] `just plimsoll` passes — `App` directive ratcheted 114 → 90
- [x] `just arch-lint` passes
- [x] Builds across default / `postgres` / `memorybackend` tags
- [x] Package coverage 79% against the 55% floor

## Manual Review

- [x] `/code-review` run (cranky-code-reviewer, word-diff re-derivation vs develop originals) — **zero findings**
- [x] Substitution fidelity exact — every moved body verbatim modulo the receiver/field table and the ACL seams carried across in the post-#1228 / #1235 rebases (see below); the three receiver-drops (`resolveLinkTarget`, `findListByEntityType`, `resolveRelationWidgets`) verified genuinely receiver-free
- [x] Rebase onto post-#1228 develop — semantic conflict in `resolveRelationColumnValues` resolved by porting develop's DEC-ZBI39P / BUG-R9EHKV gating onto the handler copy, NOT by taking the extraction's pre-fix body. Verified by mutation: bypassing `viewReader.Filter` fails `TestACLViews_RelationColumnRedactsHiddenNeighborTitle` and `TestACLViews_RelationColumnDropsUnreadableNeighbor`; both pass with it restored
- [x] Rebase onto post-#1235 develop — `collectMentions` gained a `visibility.Reader` parameter in the same block this PR deletes from api_v1.go; ported to the moved `handleV1Views` as `h.viewReader`. Verified by mutation: passing `nil` there fails `TestV1Views_MentionsPopulated`
- [x] Dead code removed — `entityTitle` (helpers.go) lost its last caller once the gated `resolveRelationColumnValues` replaced it with a direct `DisplayTitle` call; deleted (caught by `unused` linter, repo-wide grep confirms no other reference)
- [x] Capture-once + ACL parity — `handleV1Views` keeps exactly 2 schema loads; all three `gateRead` calls preserved in position BEFORE traversal (uniform-404 intact)
- [x] Deletions verified dead — repo-wide grep on develop: zero non-test callers of the nav-enrichment cluster; live sidebar path untouched; deleted tests map 1:1 to deleted symbols
- [x] Wiring parity NewApp ↔ rebindApp — `app.views` constructed before commandHandler in both; `executeView` rebind non-nil at bind time
- [x] plimsoll arithmetic verified against the diff: exactly 24 removed, 0 added (16 moved + 3 package-leveled + 5 deleted)

**Review Responses:** zero findings against the original extraction. The
post-#1228 rebase surfaced one issue not present at review time: the moved
`resolveRelationColumnValues` and `executeView` predated develop's read-out
gating, so a verbatim resolution would have reverted an ACL leak fix. Resolved
by porting the gating onto `viewsHandler` (`viewReader` field + `redactor()`
helper, wired in NewApp and rebindApp) — see [[TKT-I37338]] rebase note. One
informational note recorded:
`viewsHandler.store` is a fixed-by-value handle where develop read the live
`App.store` field per call. Divergence is latent-only — none of the 5
store-wrapping tests exercise a views path — and matches the accepted
writeHandler/attachmentHandler pattern. The ACL-sensitive sidebar count path
deliberately kept `h.services().Store` (live), preserving exact develop
semantics.
