---
id: REV-017K5H
type: review-checklist
title: 'Review: CalDAV prep: VTODO renderer + completion fields in internal/calfeed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/calfeed/` PASS at **95.4%** (up from 94.8%; floor 50%).
`golangci-lint run ./internal/calfeed/` — 0 issues. `go vet` clean, `gofmt`
clean, `just arch-lint` OK, `go build ./...` OK. Downstream `internal/dataentry`
+ `internal/dataentryconfig` pass unchanged.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — none were raised
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-4RWHHZ, RR-PO49W3, RR-1W6PSF, RR-XVVFZ3 (significant);
RR-1DM1I7, RR-V4RGAF, RR-D2KBPQ, RR-187PW6, RR-PWPKM0 (minor). All nine
`addressed`.

The review used **mutation testing**, which found two holes that inspection had
missed: a duplicated `PERCENT-COMPLETE` and a `Priority`-blind `TodoETag` both
passed the entire suite. Both were reproduced independently before fixing, and
both now fail loudly — re-verified after the fix.

Three items are documented as deliberately NOT done, each with reasoning:

- **ctag domain separator** (RR-V4RGAF) — mixing `Component` into the hash would
change every existing event-feed ctag, forcing a needless full re-sync on every
subscribed client, to fix a collision that is inert (ctags are only compared
within one collection).
- **Component/slice mismatch as an error** (RR-D2KBPQ) — would change
`RenderCollection`'s signature for every VEVENT caller to guard a state no
caller can reach. Belongs in TKT-UGYSC8's config-load validation instead.
- **`fixtureProp` returning `(string, bool)`** (RR-187PW6) — the inverted
comparison no longer depends on `""` meaning absent.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all seven PASS — see the per-AC table in IMPL-3SIYV4. AC6
was implemented as a semantic-match rather than a parse-round-trip (calfeed has
no parser by design); the reviewer mutation-tested the substitute and confirmed
it has real teeth. It has since been strengthened from a whitelist to a
denylist. The true round trip is tracked on TKT-MF1CWZ.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing
surface — this is an internal serializer with no config, CLI or API change. The
user-facing CalDAV documentation lands with TKT-N8RESF.)
- [x] ~~User-facing documentation updated~~ (N/A, as above)
- [x] ~~Docs-checklist marked as done~~ (N/A, as above)

Package-level godoc was updated: the `Todo`/`Component` types, the
`Todo.normalized` contract, and the RFC citations behind each normalization are
all documented at the declaration site.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Three commits: `d57d186d` (renderer), `5a0cac4e` (normalize + validate at the
chokepoint), `d57e2eee` (invert the fixture comparison). Deferred work is
recorded on TKT-MF1CWZ rather than left as code comments.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1308

## Evidence (TKT-SNBQX0 — VTODO renderer in internal/calfeed)

Reviewed as part of the CalDAV code review; see **REV-E7QYNN** for the full
finding table and automated-check figures.

**No findings against the renderer.** The reviewer stated plainly that
`internal/calfeed`'s "normalization, clamping, and validTrigger validation are
correct and well-argued".

It was also load-bearing for a fix elsewhere: `Todo.normalized()`'s rule that "a
timestamp is the stronger signal" is what RR-R4SCVX applies on the inbound path,
where a COMPLETED with no STATUS was previously dropped and reverted the user's
checkbox. Only the promotion arm transfers — normalized() also demotes a
STATUS:COMPLETED carrying no timestamp, which is right outbound and wrong
inbound.

One change here since the original review: `Todo.Origin` was added for the
client-marker design and then **removed** when that design was reverted in
favour of the alias-table inference.

Coverage 95.4%. `just lint` 0 issues.

**Not done:** PR (`/pr`), shared across the CalDAV tickets.
