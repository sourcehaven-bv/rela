---
id: IMPL-EMCI7Z
type: implementation-checklist
title: 'Implementation: Move operator customisation into a single custom/ folder and serve arbitrary assets from it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — new: `TestValidCustomEntry` (17 cases),
      `TestOpenCustomEntry_NeverEscapes` (9 vectors),
      `TestOpenCustomEntry_SymlinkInsideProject`, `TestRootLevelCustomNotServed`,
      `TestRootLevelCustomNotInjected`, `TestOpenCustomEntry_Directories`,
      `TestCustomEntryContentType`.
- [x] Integration tests written — 6 Playwright e2e, including a real asset
      (`custom/logo.svg` + nested `fonts/brand.woff2`) fetched over the wire and
      referenced from `custom.css`, plus the inverted containment test.
- [x] Feature implemented — `project.CustomDir`, `validCustomEntry`,
      `openCustomEntry` (nested `os.OpenRoot`), `customAssetExists` via stat,
      `appEntryContentType` reused, `slog.Warn` on oversize.
- [x] Edge cases handled: empty file, directory (both spellings), no index
      resolution, symlink escape, oversize, dot-segments, nested paths,
      `custom/` absent, unknown extension.

## Manual Verification

- [x] **AC1** `custom/custom.css` + `custom.js` served and injected —
      `TestSelectShell`, `TestSPAShellInjection` (5 cases) green; injected URLs
      unchanged from TKT-3DBK6I.
- [x] **AC2** assets served — e2e fetched `logo.svg` (200, `image/svg+xml`) and
      `fonts/brand.woff2` (200, `font/woff2`) over the wire.
- [x] **AC3** containment — 9 traversal vectors; **no spelling reached a file
      outside `custom/`**, verified against sensitive files with no decoy inside.
- [x] **AC4** discrimination — with BOTH root and folder copies present, the
      folder version wins and `ROOT-VERSION` never appears; with only the root
      copy, the shell is byte-identical to stock.
- [x] **AC5** stock deployment — byte-identical shell, `/_custom/*` 404s.
- [x] **AC6** docs — see docs checklist.
- [x] **AC7** unknown extension — `data.avif` serves as `octet-stream`, not 404.
- [x] **AC8** dot-segments — `.env`, `.env.backup`, `.git/config`, `.DS_Store`,
      `sub/.hidden`, `sub/.git/config` all 404, incl. nested (per-segment) cases.
- [x] **AC10** symlink — a link inside `custom/` to `../metamodel.yaml` is
      refused. This is the case a single `os.OpenRoot` would have followed.
- [x] **AC11** directories — `fonts` (IsDir check) and `fonts/` (fs.ValidPath)
      both 404; `fonts/index.html` is not resolved for a directory request.

## Quality

- [x] Follows project patterns — `openAppEntry` chain copied, `appContentTypes`
      reused, uniform 404, `apps_test.go` table conventions, e2e page-object rule.
- [x] No silent failures — oversize logs `slog.Warn` naming the file and cap, so
      an operator's too-large image is diagnosable rather than a mystery 404.
- [x] `just lint` 0 issues, `just arch-lint` clean, `just plimsoll` clean,
      coverage 77.4% PASS, `go test -p 2 ./...` green, e2e 6/6.

## Note on a test that was wrong before it was right

The first version of the traversal test asserted `openCustomEntry` **errors** on
`../secret.txt`. It failed — because `path.Clean` ANCHORS that to
`custom/secret.txt`, which the fixture itself created. The implementation was
correct; the assertion was not.

Rewritten to assert **containment** (sensitive files outside `custom/`, no decoy
inside, assert the content never appears). The original shape would have passed
against a genuinely leaky implementation, since it only proved "this request
errored", not "no file outside the directory is reachable".
