---
id: PLAN-6VVJJZ
type: planning-checklist
title: 'Planning: Move operator customisation into a single custom/ folder and serve arbitrary assets from it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-IWMETE. In: `custom/` folder replaces the two root-level
files; arbitrary files served at `/_custom/<path>`; dotfile exclusion; docs +
e2e. Out: back-compat, `@layer`/injection/`disable_custom_injection` changes,
any upload UI, and `apps/`'s own (absent) dotfile policy.

**Acceptance Criteria:** AC1-AC8 on TKT-IWMETE. Four design decisions were
settled with the maintainer BEFORE planning and are recorded there: entry names
stay `custom.css`/`custom.js`; serve everything with an extension map for
Content-Type only; the folder is public+unauthenticated; dot-prefixed segments
404.

## Research

**Research Doc:** N/A — the design questions were settled directly with the
maintainer, and the two load-bearing technical assumptions were settled by probe
(below) rather than by survey.

**Prior art — `apps/` is the template, and it is a close fit.**

`openAppEntry` (`internal/dataentry/apps.go:146-199`) is the exact shape needed,
one nesting level shallower. Its chain: `path.Clean("/"+entry)` → reject `"/"` →
`fs.ValidPath(rel)` → `os.OpenRoot(projectRoot)` → `root.OpenRoot(subdir)` →
`Open(rel)` → `Stat` → reject `IsDir` → `io.LimitReader(max+1)` → length check.
Nested paths already work there (`apps_test.go:418` exercises `sub/asset.js`).

Reusable unchanged: `appContentTypes` (apps.go:206-227, 20 extensions),
`appEntryContentType` (apps.go:233-238, unknown → `octet-stream`),
`maxAppFileBytes` (apps.go:62). The fixed map is deliberate over
`mime.TypeByExtension` — deterministic across deploy boxes, and correctness is
load-bearing under `nosniff` (rationale at apps.go:201-205).

**`apps/` does NOT exclude dotfiles.** Nothing filters `.`-prefixed names;
`path.Clean` only collapses `.`/`..` *segments*. So `apps/<id>/.env` is served
today. The dotfile rule for `custom/` is therefore **new logic with no in-repo
precedent** — worth stating, and worth NOT retrofitting into `apps/` under this
ticket (flagged separately on TKT-IWMETE).

**Survives from TKT-3DBK6I unchanged:** `injectTags`, `insertBefore`,
`errCustomAssetNotFound`, `maxCustomFileBytes`, `customURLPrefix`,
`spaHandlerWithCustom`, `servesSPAShell`, `spaHandler`, the whole `@layer` work,
and `customAssets.enabled` (the live-read closure).

**`mux.HandleFunc(customURLPrefix, …)` (router.go:128) already matches nested
paths** — Go's `ServeMux` prefix pattern needs no change. That line is the one
piece of the current wiring that was already right for a directory.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**VERIFIED BY PROBE — the containment chain, before writing any of it.**

A throwaway probe implemented the intended chain (clean → `fs.ValidPath` →
dot-segment reject → `os.OpenRoot(projectRoot)` → `OpenRoot("custom")` → `Open`)
against a real temp project. Result:

- SERVED: `custom.css`, `logo.svg`, `fonts/b.woff2`
- REJECTED: `.env`, `.git/config`, `.DS_Store`, `../secret.txt`,
`../../etc/passwd`, `/etc/passwd`, `a/../../secret.txt`, `sub/../../secret.txt`,
`""`, `.`, `..`

**The probe also falsified a claim I had written into the ticket.** I had said
the dot-segment rule "incidentally also rejects `..`". It does NOT: `path.Clean`
resolves `..` away *before* the check, so `../secret` cleans to `secret`, which
has no dot segment. Traversal defence rests entirely on `fs.ValidPath` +
`os.OpenRoot`. The dot rule is orthogonal and must not be credited with it. The
ticket now carries the correction.

**Design.**

- New `project.CustomDir = "custom"` in `internal/project/context.go` beside
`AppsDir` (:21), following the documented convention there that filesystem-only
dirs are package constants, not `Context` fields.
- `openCustomAsset(projectRoot, rel)` → `openCustomEntry(projectRoot, rel)`,
copying `openAppEntry`'s chain with one `OpenRoot` level and the added
dot-segment reject. Uniform `errCustomAssetNotFound` on every failure.
- `isCustomAssetName` (exact two-name compare) → `validCustomEntry(rel) bool`
(clean + ValidPath + no dot segment).
- `customAssetContentType` → `appEntryContentType` (delete the two-branch fn).
- `customAssetExists` keeps its stat-not-read shape (RR-CR-DOUBLEREAD) but
resolves under `custom/`.
- **The 4-variant shell model SURVIVES.** Decision 1 kept two well-known entry
points, so `shellVariants{plain,css,js,both}` and `selectShell` still hold; only
the paths they stat change. Worth stating because the research flagged the model
as "a direct product of exactly-two-files" — it is, and that product is still
valid.
- `customCSSTag` / `customJSTag` string constants are unchanged: the URLs
`/_custom/custom.css` and `/_custom/custom.js` are identical before and after.

**Rejected alternatives:**
- *Root-level fallback for back-compat* — rejected on TKT-IWMETE: zero users
(verified unreleased), so a fallback doubles the surface permanently to serve
nobody, and AC4 exists specifically to prove the old layout is gone.
- *Strict extension allow-map (404 unknown)* — rejected by the maintainer:
makes rela gatekeep the operator's own files (AVIF etc. would need a rela
release), and protects nothing from `custom.js`, which is already fully trusted
same-origin code.
- *Operator-extensible MIME map in `data-entry.yaml`* — considered and dropped:
with an `octet-stream` fallback there is nothing left to extend.
- *`mime.TypeByExtension`* — rejected for the reason apps.go:201-205 already
documents: non-deterministic across deploy boxes, and Content-Type correctness
is load-bearing under `nosniff`.

**Files to modify:**
- `internal/project/context.go` — add `CustomDir`.
- `internal/dataentry/custom.go` — ~60% rewrite (see inventory above).
- `internal/dataentry/custom_handler.go` — path validation; content-type swap;
**rewrite the `#nosec G705` rationale**, whose stated boundary ("the two-name
allowlist decides WHICH file may be read") ceases to exist.
- `internal/dataentry/router.go` — line 125 wiring + 4 comment blocks.
- Tests: `custom_test.go` (17 funcs; **3 assertions invert**),
`custom_shell_test.go` (3 funcs, path-only). `custom_layer_test.go` needs NO
functional change (it only inspects embedded CSS).
- `e2e/tests/customisation.spec.ts` (5 tests; the "non-allowlisted" test at :102
inverts its premise), `e2e/pages/customisation.page.ts` (2 hardcoded URLs —
unchanged in value, but the page object gains an asset helper).
- Docs: `docs/customisation.md` (**hand-written, verified NOT generated** — no
`GUIDE-customisation.md` source, no match in `scripts/generate-docs.sh`; edit
directly, do NOT run `just docs` for it), `internal/dataentry/CLAUDE.md`
(:235-239 inverted claim), `docs-project/entities/guides/GUIDE-data-entry.md`
(:247-250, light — and this one DOES need `just docs` after),
`internal/dataentryconfig/config.go` (:129-139 comments),
`frontend/relaCssLayer.ts` (:9 comment URL).

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**This ticket WEAKENS the strongest guarantee TKT-3DBK6I shipped.** That is the
headline, not a footnote. Today traversal is structurally impossible because the
input is compared against two literals before any filesystem call. After this,
the input is an attacker-controlled path and containment rests on `fs.ValidPath`
+ `os.OpenRoot`.

That is the `apps/` model, it is well-tested there
(`TestOpenAppEntry_Traversal`), and the probe above confirms it holds for this
shape — but it must not be waved through *because* `apps/` does it.

**Input:** the request path under `/_custom/`. Validation: `path.Clean("/"+p)` →
reject `"/"` → `fs.ValidPath` → reject any dot-prefixed segment → `os.OpenRoot`
containment → reject `IsDir` → size cap. Every failure → uniform 404, no path in
the message.

**The folder is served PUBLICLY and UNAUTHENTICATED.** Verified: `/_custom/` is
not an `isAPIPath` (`router.go:216`), so it sits outside both
`requireVerifiedJWT` and `attachACLRequest` — deliberately, so the SPA shell and
its assets load unauthenticated. A stray file in `custom/` is readable by anyone
who can reach the server, not merely by a logged-in operator.

Mitigation is (a) the dotfile rule, which catches the realistic accidents
(`.env`, `.env.backup`, `.git/`, `.DS_Store`) — note `.env.backup` has the
*unknown* extension `.backup` and would have been SERVED under an
extension-map-only design; and (b) prominent documentation. A strict extension
gate was rejected as partial anyway: `.txt`/`.json` are both in the map, so a
secret in either would still be served.

**FOR DESIGN REVIEW:** confirm the unauthenticated-path reading and decide
whether docs + dotfile rule suffice, or whether `custom/` warrants its own gate.
This rests on a routing read, not a test.

Unchanged: `custom.js` stays fully trusted, same-origin, no CSP — the opposite
posture from sandboxed `apps/`. Serving more file *types* does not change that.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

- **AC1** → `TestSelectShell` / `TestSPAShellInjection` with entries under
`custom/`; the injected URLs are unchanged, so the tag assertions stand.
- **AC2** → new `TestOpenCustomEntry_Assets`: `logo.svg` → `image/svg+xml`,
`fonts/b.woff2` → `font/woff2`, nested path, `nosniff` present.
- **AC3** → `TestOpenCustomEntry_Traversal` over the probe's 11 rejected vectors
  + `TestHandleCustomAsset_TraversalNeverEscapes` through the real router (the
primary security test; it survives and grows).
- **AC4** → new `TestRootLevelCustomNotServed`: `custom.css` at the PROJECT ROOT
(not in `custom/`) is neither served at `/_custom/custom.css` nor injected. This
is the test that proves the old layout is genuinely gone rather than silently
still working.
- **AC5** → `TestSPAShellInjection` stock case (byte-identical shell) + 404 on
`/_custom/anything` with no `custom/` dir.
- **AC6** → docs review.
- **AC7** → `custom/data.avif` serves 200 `application/octet-stream` — pins
serve-everything so a future "hardening" cannot reintroduce gatekeeping.
- **AC8** → table over `.env`, `.env.backup`, `.git/config`, `.DS_Store`,
`sub/.hidden`, `sub/.git/config` → all 404, INCLUDING the nested cases that
prove the rule is per-segment not per-filename.

**Assertions that INVERT (must be rewritten, not path-patched):**
- `custom_test.go:247` "404 for non-allowlisted" — `/_custom/secret.txt` must
now SERVE when `custom/secret.txt` exists.
- `custom_test.go:342` `TestCustomAssetExists_MatchesOpen` "non-allowlisted
name" subtest — same inversion.
- `custom_test.go:49-70` rejection table — only the traversal entries survive;
`secret.txt`, `CUSTOM.CSS`, `custom.cs` are no longer policy violations.
- `e2e/tests/customisation.spec.ts:102` — premise inverts: the correct test is
that a file OUTSIDE `custom/` is unreachable while one INSIDE is served.

**Edge cases:** empty file (serve 200, still inject); directory request
(`/_custom/fonts` → 404); nested dir traversal; symlink inside `custom/` that
escapes (→ reject, `os.OpenRoot`); oversize; unicode/NUL in path; `custom/`
absent entirely; `custom/` present but empty; case sensitivity (APFS) is no
longer a policy question since arbitrary names are allowed.

**Integration:** e2e must cover an actual asset — write `custom/logo.svg`,
reference it from `custom/custom.css` as `url(/_custom/logo.svg)`, and assert
the browser resolves it (a 200 on the network, not just DOM presence). That is
the whole point of the ticket and cannot be proven by a Go test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

1. **Traversal regression (HIGH impact, LOW likelihood).** The guarantee
genuinely weakens. *Mitigation:* copy `openAppEntry` exactly rather than
improvising; the probe already validated the chain; keep
`TestHandleCustomAsset_TraversalNeverEscapes` through the real router.
2. **Accidental exposure via the public path (MEDIUM).** *Mitigation:* dotfile
rule + prominent docs. Residual risk accepted and documented: a non-dotfile
secret (`custom/notes.md`) IS served.
3. **A stale root-level file silently still working (MEDIUM).** If the old path
were left half-supported, an operator could think they had migrated when they
had not. *Mitigation:* AC4 tests the negative explicitly.
4. **Doc drift (LOW-MEDIUM).** `docs/customisation.md` is hand-written and
`docs/data-entry.md` is generated from `docs-project/`. Editing the wrong one is
exactly the mistake made in TKT-3DBK6I. *Mitigation:* the file list above marks
which is which; `just docs-check` catches the generated side.
5. **`#nosec G705` rationale going stale (LOW).** Its stated boundary
disappears. *Mitigation:* explicitly listed as a rewrite, not a tweak — a stale
security comment is worse than none.

**Effort: M.** Mostly mechanical given `apps/` supplies the pattern; the volume
is in tests (17 funcs) and docs, not in new logic.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

- [x] `docs/customisation.md` — folder layout, asset example, the
public+unauthenticated caveat stated prominently, the dotfile rule, and
**:221-222 rewritten** ("Only these two exact filenames are served... no way to
serve any other project file" — false in both halves).
- [x] `internal/dataentry/CLAUDE.md` — :235-239 inverted security bullet.
- [x] `docs-project/entities/guides/GUIDE-data-entry.md` :247-250 (light) +
`just docs` after.
- [x] `internal/dataentryconfig/config.go` :129-139 comments.
- [x] `frontend/relaCssLayer.ts` :9 comment URL.
- [x] ~~`frontend/CLAUDE.md`~~ (N/A: only names `custom.css`, still accurate.)
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `README.md`~~ (N/A.)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-DR-HTMLCSP (critical); RR-DR-AC4, RR-DR-SYMLINK,
RR-DR-DOTCLAIMS, RR-DR-EXPOSURE (significant); RR-DR-ETAG, RR-DR-SHELLTRIPWIRE,
RR-DR-OPSDETAIL (minor). All folded in below.

**Verdict was "do not implement as planned."** Three of my own claims were
falsified and are corrected on the ticket:

1. **RR-DR-HTMLCSP (critical) — REJECTED after maintainer challenge, and the
rejection is correct.** The finding's mechanism is real (`.html` is in
`appContentTypes`; `apps/` sets a CSP at `apps_handler.go:125`, `/_custom/` does
not) but its conclusion is not. `custom.js` is injected into the SPA's own
document, same-origin, no CSP — it can already read the session cookie and reach
every API endpoint, so an HTML page on the same origin adds NO capability. The
delivery-vector argument fails too: anyone who can write `custom/page.html` can
write `custom.js`, which is already injected and runs for every user on every
load without needing a click. Decision 2 stands unamended; no `sandbox` CSP, no
`.html` override, AC9 dropped.

   *My error, worth recording:* I verified the MECHANISM (would `.html`
   execute?) and treated that as confirming the THREAT, without asking the
   actual question — whether execution granted anything `custom.js` did not
   already grant. Verifying a finding's premise is not the same as verifying its
   conclusion.
2. **RR-DR-AC4 (significant)** — my AC4 test was vacuous. After the change a
root-level file is not in the `custom/` tree at all, so it 404s whether or not a
fallback exists. Rewritten as a discrimination test (ROOT-VERSION vs
FOLDER-VERSION).
3. **RR-DR-ETAG (minor)** — I wrote that RR-CR-ETAG "is unaffected and stays
deferred". Wrong: it was deferred because the shell is 3.4KB and
uncacheable-by-design, which does not transfer to a static 200KB webfont.

**RR-DR-SYMLINK** is the one that changes the implementation's emphasis: the
nested `OpenRoot("custom")` is security-critical, not incidental. A symlink
inside `custom/` pointing at `../metamodel.yaml` never leaves the project root,
so a single `os.OpenRoot(projectRoot)` would follow it and serve the file. The
plan described this as "one `OpenRoot` level" — a structural aside. It gets a
named comment at the nesting site and AC10.

**RR-DR-EXPOSURE / RR-DR-DOTCLAIMS** are documentation-accuracy fixes, and given
this feature's security story lives in comments and CLAUDE.md, they are not
cosmetic. The unauthenticated decision STANDS (gating breaks unstyled-login),
but the docs must say the folder is *more* exposed than `apps/`, and the
overstated dotfile claims (`~`/`#file#` backups are NOT caught; `.well-known/`
is a real false positive) are corrected.

**RR-DR-SHELLTRIPWIRE / RR-DR-OPSDETAIL** — godoc trip-wire on the 4-variant
model; ACs for directory spellings and no-index-resolution; `slog.Warn` on the
oversize branch so an operator whose hero image silently 404s gets a diagnostic.

**Independent confirmation:** the reviewer re-ran the containment chain against
vectors my probe missed (in-project symlink, absolute symlink, symlinked updir,
Windows separators, NUL, URL-encoded dots, macOS resource fork) — all rejected.
The chain is sound; the findings are about what surrounds it.
