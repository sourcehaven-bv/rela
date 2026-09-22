---
id: IMPL-1YYH8O
type: implementation-checklist
title: 'Implementation: Replace command open/reveal launcher with an ACL-gated HTTP download'
status: done
---

<!-- @managed: claude-workflow v1 -->

Created retroactively: the ticket moved `ready` → `review` directly, so the
`in-progress` automation never fired. Content reflects what was actually done.

## Development

- [x] Unit tests written for new code (`internal/dataentry/command_files_test.go`: token mint/lookup/expiry, containment, directory rejection, download handler 404 paths, per-download re-authorization)
- [x] Integration tests written (`TestCommandExecEmitsTokenNotPath` runs a real command through the SSE loop and downloads the emitted token; `CommandModal.test.ts` drives a canned SSE stream through the component)
- [x] Happy path implemented (`file` message → token → `GET /api/command-file/{token}` → hardened download)
- [x] Edge cases from planning handled (path outside project, traversal, nonexistent file, directory, missing file at download time, unknown/expired token, mint failure)
- [x] Error handling in place (containment and non-regular-file failures log at Warn and drop the download link without failing the run; a `crypto/rand` failure logs at Error; unknown/expired/unauthorized/unopenable all return an indistinguishable 404)

## Test Quality

- [x] Fixture builders (`newHandlerTestApp`, `commandPolicyACL`, `mustContain`, `downloadFile` mirror the package's existing helpers)
- [x] No hardcoded values in assertions (paths built from `t.TempDir()`; tokens read back from the store or the SSE stream, never literals)
- [x] Only values that matter (header assertions name the four security headers; the body is compared to the bytes actually written)
- [x] Interpolated values from objects (download URLs built from the minted token, not a fixed string)
- [x] Property comparisons from original object (`entry.cmd.Permission` compared against the `CommandConfig` passed to `mint`)

**Mutation-tested rather than assumed.** Three deliberate regressions, each
caught: removing the per-download `authorizeCommand` fails both AC-4 subtests;
hiding the Download link fails the frontend test; making the frontend helper
ignore its SSE body fails 2 of 7. A test that cannot fail is not evidence.

## Manual Verification

- [x] Feature manually tested end-to-end (real `rela-server` on the demo project, port 8799)
- [x] Each acceptance criterion verified (AC-1/3/4/6 confirmed against the running server; AC-2/5 by test)
- [x] ~~Edge cases manually verified on the real target~~ (N/A: deferred by Jeroen's explicit decision to merge on green CI — see below). The ticket's test plan calls for verifying on the actual headless remote deployment, since the original symptom (`xdg-open` silently no-opping) is only observable there. A local macOS server cannot reproduce it. Jeroen was asked and chose to merge on green CI accepting this, so the check happens post-merge or not at all.

Live evidence: `/api/open-file` and `/api/open-url` both 404; a real
`generate-pdf` run emitted `{"type":"file","token":...}` with no path; the token
downloaded a valid `%PDF-1.4` with `attachment`, `nosniff`, sandbox CSP and
`no-store`. **One process, one token, three principals: alice 200, bob 404,
unknown 404** — authorization is evaluated per download against the caller.

Also learned live (pre-existing, not introduced here): `acl.yaml` is read at
startup, not hot-reloaded, so revoking a permission needs a restart.

## Quality

- [x] `go build ./...` clean; full Go suite green; `-race` clean on `internal/dataentry` (466s) and `internal/migration`
- [x] `just arch-lint` OK — no warnings
- [x] `just plimsoll` exit 0
- [x] `golangci-lint run ./...` — 0 issues
- [x] `just comment-lint` clean; one `comment-report` advisory finding fixed rather than suppressed (the `containedPath` witness type)
- [x] `just coverage-check` — 79.7%, both thresholds PASS
- [x] Frontend: `typecheck` clean, `lint` 0 errors, 2933 tests pass
- [x] `just docs` regenerated and verified idempotent
