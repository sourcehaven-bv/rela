---
id: REV-ZK8P63
type: review-checklist
title: 'Review: Move operator customisation into a single custom/ folder and serve arbitrary assets from it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test -p 2 ./...` — all packages pass
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK
- [x] `just plimsoll` — clean (customAssets keeps the surface off App)
- [x] `just coverage-check` — PASS, 77.4%
- [x] `npx playwright test` — 242 passed, 8 skipped, 0 failures
- [x] `just docs-check` — regenerated from `docs-project/` source

## Code Review

`/code-review` (cranky-code-reviewer). Verdict: containment airtight, no
critical findings. 5 findings recorded, 4 fixed, 1 deferred with reason.

- **RR-CR2-DIVERGE (significant, FIXED)** — `customAssetExists` (stat) and
  `openCustomEntry` (read) disagreed on oversize and unreadable files, so the
  shell injected a `<link>` that then 404'd. **Reproduced with a probe before
  fixing.** Especially bad because `TestCustomAssetExists_MatchesOpen` claimed
  to pin exactly this and covered neither case. Fixed by adding size +
  readability to the exists check; both cases added to that test.
- **RR-CR2-AMPLIFY (significant, FIXED)** — 4 MiB buffered *before* the size
  check, on a deliberately unauthenticated route. Moved to `info.Size()` from
  the stat already being done.
- **RR-CR2-AC11COMMENT (minor, FIXED)** — my comment claimed `fonts` and
  `fonts/` hit different guards. **Verified false**: `path.Clean` makes them
  identical, both dying at `IsDir`. Corrected, plus a subtest pinning the real
  (single) guard.
- **RR-CR2-STALECOMMENTS (minor, FIXED)** — test comments still described
  `os.OpenRoot` as "defense-in-depth behind the allowlist", inverting the
  security story after the allowlist was removed.
- **RR-CR2-SERVECONTENT (minor, DEFERRED with reason)** — no ETag/Range, and
  every method returns a body. Deferred explicitly rather than inherited: the
  fix replaces the read path this ticket just hardened and wants its own
  containment re-verification.

## Acceptance Verification

- [x] **AC1** PASS — `TestSelectShell`, `TestSPAShellInjection` (5 cases).
- [x] **AC2** PASS — e2e fetched `logo.svg` (200, `image/svg+xml`) and
      `fonts/brand.woff2` (200, `font/woff2`) over the wire.
- [x] **AC3** PASS — `TestOpenCustomEntry_NeverEscapes`, 9 vectors, asserting
      content absence against files with no decoy inside `custom/`.
- [x] **AC4** PASS — discrimination test: folder version wins, `ROOT-VERSION`
      never appears; root-only case yields the stock shell.
- [x] **AC5** PASS — byte-identical stock shell, `/_custom/*` 404s.
- [x] **AC6** PASS — docs rewritten; the false "only these two exact filenames"
      claim replaced; exposure warning placed above the layout example.
- [x] **AC7** PASS — `data.avif` serves as `octet-stream`, not 404.
- [x] **AC8** PASS — 6 dot-segment cases incl. nested.
- [x] **AC10** PASS — in-project symlink refused (the case a single
      `os.OpenRoot` would have followed).
- [x] **AC11** PASS — both directory spellings 404; no index resolution.

## Note

Two of the five findings were defects in my own *tests and comments* rather than
in the code: a test whose comment claimed a property it did not check, and a
comment documenting a guard that does not exist. Both are the kind that mislead
the next reader into removing a real check.
