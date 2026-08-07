---
id: IMPL-30LP5R
type: implementation-checklist
title: 'Implementation: Release workflow ships rela-server with an empty embedded SPA (GoReleaser job never builds the frontend)'
status: done
---

## Development

- [x] Fix implemented — `.github/workflows/release.yml` `release` job gains
Set up Node.js + Build Vue frontend, mirroring the `desktop` job.
- [x] Guard implemented — `scripts/check-embedded-spa.sh` with two subcommands
(`tree`, `binary`), wired in as two workflow steps.
- [x] ~~Unit tests~~ (N/A: the change is CI workflow + a shell guard; the guard
is verified directly against real broken and real good binaries below, which is
stronger evidence than a mock would be)
- [x] Edge cases handled — dir missing, empty dir, index.html without assets,
empty assets dir, missing binary, binary with no assets.

## Manual verification (evidence)

**Guard: tree mode — negative cases all fail (rc=1)**

| Case | Result |
| --- | --- |
| dir does not exist | FAIL, rc=1 |
| empty dir | FAIL, rc=1 |
| `index.html` present, no `assets/` | FAIL, rc=1 |
| `assets/` present but empty | FAIL, rc=1 |
| real `just build-frontend` output | **OK, rc=0** |

**Guard: binary mode — discriminates real artifacts**

- Shipped **v0.14 `rela-server`** (the actual broken release artifact):
`FAIL: no embedded SPA assets found`, rc=1. The guard catches the real bug.
- Locally built `rela-server` after `just build-frontend`:
`OK: embeds SPA assets (4 hashed asset references)`, rc=0.

**End-to-end serving** (`rela-server` built with the frontend, run against the
repo's own `tickets/` project):

- startup log contains no SPA/embed error (v0.14 fails here with
`embedded SPA check failed`)
- `GET /` → `status=200 bytes=3397`, HTML references `/assets/index-BO8XeL4Q.js`
- `GET /assets/index-BO8XeL4Q.js` → `status=200 bytes=31848`

**Acceptance criteria**

- [x] Release job builds the SPA before GoReleaser — step order verified by
parsing the YAML: checkout → Go → Node → Build → Verify tree → GoReleaser →
Verify packaged binary.
- [x] Packaged `rela-server` starts clean and serves non-empty `index.html` —
verified above.
- [x] Release **fails** when the SPA is missing — verified against the real
broken v0.14 binary, not a simulation.

## Quality

- [x] `bash -n` clean; workflow YAML parses (`yaml.safe_load`).
- [x] `go test ./internal/dataentry/...` passes.
- [x] Work tree clean after frontend build (satisfies `ci-clean-worktree-guard`).
- [x] No silent failures — the guard exits non-zero with an actionable message
naming `just build-frontend`.
- [x] No untrusted `${{ }}` interpolation into `run:` blocks (workflow-injection
class does not apply; the only interpolated value is a `find`-derived path).

## Notes

Scope split recorded on `BUG-2YZ575 --depends-on--> TKT-O03TB`: this delivers
the release-job guard; TKT-O03TB retains the broader packaged-binary smoke test
(spawn binary, HTTP GET, wire into `just smoke`).
