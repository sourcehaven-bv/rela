---
id: PLAN-PURT04J
type: planning-checklist
title: 'Planning: Headless rela-server build for sqlite and postgres in releases'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `headless` build tag that drops the embedded SPA (`static/`)
and the custom-app editor bundle (`app_editor_dist/`) from `internal/dataentry`;
headless root/404 behaviour; tenant-mode `StaticShell` answers 404; goreleaser
builds `rela-server-headless-sqlite` and `rela-server-headless-postgres` in
archives `rela-headless-sqlite_*` and `rela-headless-postgres_*`; `just`
recipes; CI build/test of the tag; docs in the sqlite and postgres backend
guides. Out: removing SPA-only API routes (collab websocket, custom apps stay),
a headless filesystem-backend release, `rela` CLI changes, Docker images.
Confirmed with the user: keep REST API + SSE + remote MCP; separate archives.

**Acceptance Criteria:**
1. `-tags sqlite,headless` and `-tags postgres,headless` builds of
`./cmd/rela-server` succeed without a frontend build and embed no SPA.
2. Headless router: `/api/v1/_schema` OK; `/` 200 text/plain with the
OpenAPI `Link` header; `/assets/x.js`, `/static/favicon.svg`, `/some/route` 404.
3. Non-headless builds unchanged (existing router tests pass).
4. `goreleaser check` passes; snapshot build yields both headless archives.
5. CI compiles, tests and lints the `headless` tag.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small build-tag change following existing backend-tag pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: Go build tags + embed are the standard mechanism)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: standard build-tag pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Backend selection via build tags (`sqlite`, `postgres`, `memorybackend`) in
`internal/appbuild/appbuild_*.go`; goreleaser already ships tagged variants in
separate archives (`.goreleaser.yaml`).
- `routerConfig.spaFS` (`internal/dataentry/router.go`) already swaps the SPA
FS for tests; `appEditorAsset` already returns nil when the bundle is absent.
- `EmbeddedBuildID` returns "" when no build id is embedded, which the collab
relay treats as "no check".

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. Move the two `//go:embed` declarations into `spa_embed.go`
(`//go:build !headless`). Add `spa_headless.go` (`//go:build headless`)
declaring the same vars as zero `embed.FS` values. Each file sets `const
Headless` (true/false), exported for `cmd/rela-server`.
2. `CheckEmbeddedSPA` returns nil when `Headless`.
3. `NewRouter`: when `Headless` (and no test `spaFS`), skip the `/static/`
mount and replace the SPA catch-all with `headlessRoot`: `/` GET/HEAD answers
200 text/plain naming `/api/v1` and the OpenAPI spec, still wrapped by
`withSpecLink`; every other path 404. CalDAV well-known delegation receives the
same handler. `/_custom/` stays as is.
4. `StaticShell` returns `http.NotFoundHandler()` when `Headless`.
5. `.goreleaser.yaml`: builds `rela-server-headless-sqlite`
(`-tags=sqlite,headless`) and `rela-server-headless-postgres`
(`-tags=postgres,headless`); archives `headless-sqlite` and `headless-postgres`.
Release workflow: assert the headless binaries do NOT contain the SPA entry
assets.
6. `justfile`: `build-server-headless-sqlite`, `build-server-headless-postgres`
(no `build-frontend` dependency).
7. CI: new `headless` job builds both variants, runs `go vet -tags
headless ./...` minus desktop/docs (compiles every test file), and `go test
-tags headless -run Headless` on `internal/dataentry` and `cmd/rela-server`;
lint gets `--build-tags headless` on dataentry.
8. `cmd/rela-desktop` and `cmd/rela-docs` get a `//go:build headless` file
that fails compilation, since both need the SPA.

Alternatives rejected: a runtime flag (`--no-ui`) keeps the bytes in the binary
and is what the user did not want ("not baked in"); a separate
`cmd/rela-server-headless` main duplicates 4k lines of wiring.

**Files to modify:** `internal/dataentry/static.go`, `apps_editor.go`, new
`spa_embed.go`, `spa_headless.go`, `router.go`, `staticshell.go`, new
`headless_test.go`; `cmd/rela-desktop/headless.go`, `cmd/rela-docs/headless.go`
(new guards); `.goreleaser.yaml`; `.github/workflows/release.yml`, `ci.yml`;
`justfile`; `docs-project/entities/guides/GUIDE-sqlite-backend.md`,
`GUIDE-postgres-backend.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** HTTP request paths only. The headless root
handler serves a fixed body and 404s everything else; it reads no store or
project file.

**Security-Sensitive Operations:** None added. All API middleware (ACL,
same-origin, JWT gates) wraps the mux as before; removing UI mounts reduces
surface. The `requireSameOrigin` exemption list for static paths becomes moot,
not wider.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: CI `headless` job builds both; local `go build` plus `strings`/size
check that `index.html` asset names are absent.
- AC2: `headless_test.go` (`//go:build headless`) table test over the router.
- AC3: existing `go test ./internal/dataentry/` untagged.
- AC4: `goreleaser check` and `goreleaser release --snapshot --clean` locally.
- AC5: CI job definition.
- Integration: run the headless sqlite binary on a temp project and curl
`/`, `/api/v1/_schema`, `/assets/x.js`.

**Edge Cases:** HEAD on `/`; `/index.html` (404); CalDAV enabled (well-known
still served, `/` still headless); tenant mode tenant-free request (404).

**Negative Tests:** `/static/favicon.svg` and SPA routes return 404 in headless.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Release-time SPA guard fails for headless binaries: the guard lists binaries
explicitly, so headless ones are not checked for presence; add an inverse check.
- Release runtime grows by 12 binaries: acceptable, builds are parallel.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-sqlite-backend and GUIDE-postgres-backend: a
short "Headless server" note listing the binary and archive names and what is
served.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-S8QAG7O (significant, test strategy: approach
step 7), RR-B95TD1M (minor, desktop/docs guard: approach step 8). Release
downstream jobs checked: SBOM job is generic over checksums.txt; staging globs
`rela-postgres_*`, which the new archive names do not match.
