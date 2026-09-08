---
id: IMPL-DFFQTH
type: implementation-checklist
title: 'Implementation: Entity commenting stage 1: property and section anchors'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Verified against a real `rela-server` on a scratch project (`comments: {enabled:
true, on: ["*"]}`), plus a second copy with the block removed.

- **AC1 (disabled is absent)** — with no `comments:` block: `GET`/`POST`
  `/_comments/...` → 404, `.rela/comments/` never created, and `/_schema` omits
  the `commentable` key entirely.
- **AC2** — comment created against `status` and read back.
- **AC3 (unforgeable author/id)** — POST body carrying `"id":"EVIL"` and
  `"author":"mallory@evil.com"` returned a server-minted id and
  `author: alice@example.com`.
- **AC4** — unknown/non-commentable type → 400.
- **AC7** — nonexistent entity id → 404 (indistinguishable from denied).
- **AC9 (detached is soft)** — flagged, still rendered; never a 422.
- **AC10 (lifecycle)** — `rela rename id TKT-001 TKT-042` from a SEPARATE
  process re-keyed the thread: comments readable at `TKT-042`, old id 404s, and
  the on-disk file moved to `TKT-042.yaml`.
- **AC12 (input validation)** — NUL in body → 400; empty body → 400.
- **Author refusal** — with no resolvable identity the add is refused
  (403 "Comments require an identified author") rather than persisting
  `"unknown"`, which is what keeps `-own` checks meaningful.
- **SPA** — the real `CommentsPanel` rendered against payloads captured from the
  running server: section anchor, author, date, body, Resolve/Edit/Delete
  (driven by the server's `editable`/`deletable`), the "1 open" count, and all
  three anchor options (both properties + the view section).

Not verified in a live browser: the Chrome extension was not connected in this
environment. Covered instead by the component test above plus the built bundle
(the panel and its scoped CSS ship in the `EntityDetail` chunk).

Backend-only ACL criteria (AC5/AC6/AC8 own-vs-any, ownership-conferred
permission, load-time rejection) are pinned by unit tests
(`TestAuthorizer_OwnVersusAny`, `TestAuthorizer_AsksAboutTheTargetEntity`,
`internal/acl/commentperms_test.go`) — the scratch project has no `acl.yaml`, so
per-principal permissions are inert there by design.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Notes:**

- Routes live on a focused `commentsHandler` (the `exportHandler` /
  TKT-JF5JI8 pattern), so the subsystem cost `App` exactly one method — the
  public `SetComments` setter. The plimsoll directive moved 86 → 87 with the
  reason recorded at the declaration site.
- `dataentry` → `comments` was added to `.go-arch-lint.yml`; only the
  `comments` package is granted, never `filecomments`/`memcomments` — backend
  choice stays a wiring-site decision.
- Five broken godoc links found by the `doclink` gate were fixed rather than
  suppressed (two named a `PermCommentRead` constant that does not exist — the
  real one is unexported; one said `EntityDelete` for `EntityDeleted`; two
  bracketed unexported methods, which Go cannot link at all).
- Gates run clean: full Go suite, `arch-lint`, `plimsoll`, `comment-lint`,
  `golangci-lint`, `coverage-check` (79.0%), `vue-tsc`, ESLint (0 errors),
  2142 frontend tests, production build.
