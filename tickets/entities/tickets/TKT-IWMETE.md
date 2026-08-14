---
id: TKT-IWMETE
type: ticket
title: Move operator customisation into a single custom/ folder and serve arbitrary assets from it
kind: enhancement
priority: high
effort: m
status: done
---

## Problem

TKT-3DBK6I shipped operator customisation as **exactly two allowlisted
filenames** at the project root (`custom.css`, `custom.js`), served at
`/_custom/`. That leaves the feature half-useful: an operator can restyle the
sidebar but cannot put their own logo in it. There is no way to serve a font,
image, or any other asset that operator CSS/JS needs.

`background-image: url(...)` has nothing to point at today.

Two secondary problems with the current shape:

- **The mount is dishonest.** `/_custom/` is a URL prefix backed by two
hardcoded root-level filenames; the URL implies a directory that does not exist.
Every other operator-authored surface (`apps/<id>/`, `actions/`, `scripts/`,
`templates/`) is a real directory.
- **Two homes for one feature.** An assets-only subfolder (the first idea) would
have put entry files at the root and assets under `custom/`, producing the
doubled path `/_custom/custom/logo.svg`.

## Proposal

One folder, mapped to the existing URL prefix:

```text
your-project/
└── custom/
    ├── custom.css      ← entry stylesheet (injected when present)
    ├── custom.js       ← entry module (injected when present)
    ├── logo.svg
    └── fonts/brand.woff2
```

Served at `/_custom/<path>` — so `custom.css` writes `url(/_custom/logo.svg)`.

This also collapses a question the assets-only design raised ("should .js/.css
in the asset folder be served?"): with one folder, `custom.js` simply *is* in
the folder.

## Why now — this is the cheap moment

**VERIFIED: the current layout is unreleased and unused.**

- `git tag --contains 35374c58` → empty. No release tag contains the TKT-3DBK6I
merge; latest tags are v26.8.0 / v26.7.1 / v26.7.0, all predating it.
- No `custom.css` / `custom.js` exists anywhere in the repo (excluding
`node_modules`).

So the migration cost is **zero operators**. One release from now this becomes a
breaking change with real users to carry. **This must land before the next
release tag.**

Consequently: **no back-compat, no root-level fallback, no deprecation path.** A
fallback would permanently double the surface area to serve nobody.

## Scope

IN:
- `custom/` directory replaces the two root-level files.
- Serve arbitrary files from it (fonts, images, nested paths), reusing the
`apps/` idiom: `os.OpenRoot` → `root.OpenRoot("custom")` → `Open(rel)`, with
`path.Clean`/`fs.ValidPath` pre-checks, size cap, uniform 404.
- Extension→MIME allow-map (reuse `appContentTypes`), unknown →
`application/octet-stream` under `nosniff`.
- Update docs, both CLAUDE.md files, and the e2e suite.

OUT:
- Back-compat with the root-level layout (see above).
- Any change to the `@layer` cascade work, the injection mechanism, or
`disable_custom_injection` — those are settled and stay as-is.
- Upload/management UI. Files are placed on disk, like `apps/`.

## Security — the real change, stated plainly

This **weakens the strongest guarantee in the current implementation**. Today
`openCustomAsset` compares against two exact literals, so traversal is
structurally impossible *before the filesystem is touched* — that property is
cited in the code comments, in `internal/dataentry/CLAUDE.md`, and in the
TKT-3DBK6I review.

A directory drops back to path-validation + `os.OpenRoot` containment. That is
the `apps/` model and it is well-tested (`TestOpenAppEntry_Traversal`), but it
is strictly weaker than "two exact strings" and must not be waved through on the
grounds that `apps/` does it.

Also note: **the whole folder becomes web-readable.** `apps/` has the same
property, but an `apps/<id>/` folder is purpose-built whereas `custom/` will
accumulate whatever an operator drops in it — a stray `notes.md` or
`.env.backup` would be served. This needs documenting prominently, and is a
genuine argument for the extension allow-map over serve-anything.

Unchanged from TKT-3DBK6I: `custom.js` remains fully trusted, same-origin, no
CSP — the opposite posture from sandboxed `apps/`.

## Decisions (settled with maintainer before planning)

**1. Entry filenames: keep `custom/custom.css` + `custom/custom.js`.** The path
stutters slightly, but `index.*` conventionally means "served at the directory
root", which is not what these are — they are injected into rela's shell, not
served as a page. Zero doc churn.

**2. Serve EVERYTHING in the folder**, with the known-extension map
(`appContentTypes`) used only to pick a correct `Content-Type` and
`application/octet-stream` as the fallback. Reused wholesale, INCLUDING `.html`.
Rejected: a strict allow-map that 404s unknown extensions.

Why the strict map was rejected — it would make rela gatekeep the operator's own
files. Every new image format (AVIF, JXL, whatever follows) would need a rela
release before an operator could use their own logo, in a feature whose premise
is that the operator already controls the metamodel, Lua and ACL.

This also removes the need for an operator-extensible MIME map in config
(considered, and now unnecessary): with an `octet-stream` fallback there is
nothing left to extend. The only residual gap is an operator wanting an exotic
type served *rendered* rather than as a download — add it to the built-in map
when it actually comes up.

**On `.html` specifically (design review raised this as critical; REJECTED — see
RR-DR-HTMLCSP).** Serving `text/html` from this route adds no new capability.
`custom.js` is injected into the SPA's own document, same-origin, with no CSP —
it can already read the session cookie and reach every API endpoint. An HTML
page on the same origin reaches exactly that, nothing more. And an attacker who
can write `custom/page.html` can write `custom.js` instead, which is already
injected and runs for every user on every page load without anyone visiting a
URL. No `sandbox` CSP, no `.html` type override: both would cost a legitimate
use case (an operator serving their own HTML page) for zero security gain.

**4. Exclude any path with a dot-prefixed SEGMENT.** `custom/.env`,
`custom/.git/config`, `custom/.DS_Store`, editor swap files → 404.

This is the mitigation the extension allow-map was reaching for, without its
cost. Dotfiles are categorically not web assets — no stylesheet references
`url(.env)` — so the rule has **no false positives we expect in practice**,
unlike an extension map that would block real files (AVIF) to catch hypothetical
ones.

**CORRECTED after design review (RR-DR-DOTCLAIMS).** Two claims in an earlier
draft were too strong:
- "editor swap files → 404" is only half true. Vim *swap* files for `custom.css`
are `.custom.css.swp` (dot-prefixed, caught), but vim/Emacs *backup* files are
`custom.css~` and `#custom.css#` — **not** dot-prefixed, so they are served (as
octet-stream), leaking a prior revision of operator code.
- "no legitimate false positives" is falsified by `.well-known/` (ACME
challenge, `security.txt`). Obscure enough to accept, but absolute claims in
security comments are how the next person justifies deleting the check.

The rule is a filename-shape heuristic standing in for a sensitivity classifier;
the two overlap by convention only. It catches none of `notes.md`, `backup.sql`,
`id_rsa`, `credentials.json`. It stays — still strictly better than the
extension map at `.env.backup` — but its documented scope must match reality.

It is also STRICTLY BETTER at the case that motivated the concern: `.env.backup`
has extension `.backup`, which is *unknown* to the map and would therefore have
been SERVED as octet-stream under decision 2. The dotfile rule catches it; the
extension map would not have.

Must match on **every path segment**, not just the final filename — otherwise
`custom/.git/config` passes on the segment `config`.

**CORRECTION (verified by probe):** an earlier draft of this ticket claimed the
dot-segment rule "incidentally also rejects `..`". It does NOT. `path.Clean`
resolves `..` away *before* the check runs, so `../secret` cleans to `secret`,
which has no dot segment. Traversal defence rests entirely on `fs.ValidPath` +
`os.OpenRoot` containment — the dot rule is orthogonal and must not be credited
with it.

Probe of the intended chain (clean → ValidPath → dot-segment → OpenRoot →
sub-OpenRoot("custom") → Open) confirmed: `custom.css`, `logo.svg`,
`fonts/b.woff2` served; `.env`, `.git/config`, `.DS_Store`, `../secret.txt`,
`../../etc/passwd`, `/etc/passwd`, `a/../../secret.txt`, `sub/../../secret.txt`,
`""`, `.`, `..` all rejected.

Note `apps/` has no equivalent exclusion. That is arguably a latent gap there,
but it is NOT in scope here — flag it separately rather than widening this
ticket.

**3. SECURITY — the `custom/` folder is served PUBLICLY and UNAUTHENTICATED.**

Verified in `internal/dataentry/router.go`: `/_custom/` is not an `isAPIPath`,
so it sits outside BOTH the JWT gate (`requireVerifiedJWT`) and the ACL
(`attachACLRequest`) — deliberately, so the SPA shell and its assets load
without authentication. This was ASSUMED to be authenticated during the initial
discussion and is not.

Consequence: a stray `notes.md` or `.env.backup` in `custom/` is not merely
"visible to a logged-in operator" — it is readable by anyone who can reach the
server. The mitigation is DOCUMENTATION, not a gate: a gate that 404s unknown
extensions breaks legitimate files while giving only partial protection (a
`.txt` or `.json` secret would still be served, since both are in the map).

**This needs explicit design-review attention** — it is the load-bearing
security property of the change and rests on a routing read, not on a test.
Design review must confirm it and decide whether the docs caveat is sufficient
or whether the folder warrants its own gate.

## Acceptance criteria

- AC1 `custom/custom.css` and `custom/custom.js` are served at
`/_custom/custom.css` / `/_custom/custom.js` and injected into the shell when
present, exactly as before.
- AC2 An arbitrary asset (`custom/logo.svg`, `custom/fonts/x.woff2`) is served
at its path with the correct Content-Type and `nosniff`.
- AC3 Traversal is rejected for every spelling (reuse the TKT-3DBK6I vector
list plus nested-path variants); no file outside `custom/` is ever served.
- AC4 **(REWRITTEN per RR-DR-AC4 — the original was vacuous.)** Discrimination,
not absence: with BOTH `<root>/custom.css` (`ROOT-VERSION`) and
`<root>/custom/custom.css` (`FOLDER-VERSION`) present, the served body is
`FOLDER-VERSION` and never contains `ROOT-VERSION`. Plus: with ONLY the
root-level file present, `selectShell` returns `variants.plain` (shell
byte-identical to no-customisation). The original test — root file 404s — passes
even if a fallback still exists but is reached by another spelling, and asserts
the same thing as AC3.
- AC5 A stock deployment (no `custom/` dir) serves a byte-identical shell and
404s every `/_custom/` path.
- AC6 Docs, both CLAUDE.md files, and e2e reflect the folder layout, and state
prominently that everything in `custom/` is served PUBLICLY and UNAUTHENTICATED
— "do not put anything here you would not publish".
- AC7 An unknown extension (e.g. `custom/data.avif`) is still SERVED, as
`application/octet-stream` under `nosniff` — not 404'd. Pins decision 2 so a
future "hardening" cannot silently reintroduce the gatekeeping.
- AC8 Any path with a dot-prefixed SEGMENT 404s: `custom/.env`,
`custom/.env.backup`, `custom/.git/config`, `custom/.DS_Store`,
`custom/sub/.hidden`, `custom/sub/.git/config`. Pins decision 4, including the
segment-not-just-filename rule.
- AC10 A symlink INSIDE `custom/` pointing at a project file OUTSIDE `custom/`
(e.g. `../metamodel.yaml`) is rejected. Pins the nested-`OpenRoot` property
(RR-DR-SYMLINK): a single root would follow it, since the target never leaves
the project root.
- AC11 A directory request 404s by both spellings (`/_custom/fonts` via the
`IsDir` check, `/_custom/fonts/` earlier via `fs.ValidPath`), and there is NO
index resolution — `/_custom/fonts/` must not serve `fonts/index.html`.

## Related

- Supersedes the root-level layout from TKT-3DBK6I.
- `RR-CR-ETAG` — **the earlier claim that it "is unaffected and stays deferred"
is WRONG** (RR-DR-ETAG). It was deferred on the grounds that "the shell is
~3.4KB and uncacheable-by-design"; that rationale does not transfer to a 200KB
webfont, which is static and exactly what browsers cache hardest. Under the
current handler every such asset re-transfers in full on every navigation. Must
be re-opened for the asset path or explicitly re-justified — not inherited.
