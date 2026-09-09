---
id: BUGA-94TEZD
type: bug-analysis-checklist
title: 'Analysis: A denied ?world= short-circuits past worldCapablePath, serving default-world content on refused routes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

`GET /api/v1/_analyze?world=published` against `develop` @ `169de983`, with a
role holding `read: [ticket]` and no `worlds:` grant, returns 200 carrying the
default world's entity id and title. The same request from a principal holding
`worlds: [published]` returns 422 `world_unsupported`.

Conditions: any deployment with a declared world and a route outside
`worldCapablePath`'s allowlist. No postgres or special backend needed —
reproduced on the default in-memory test app.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded in `why1`–`why5` on the bug. In short: the route-capability refusal
lived inside `if !handle.isDefault()`, downstream of `resolveWorld`, and the
`errWorldDenied` arm returns before reaching it. A question about the ROUTE was
made to depend on the resolution OUTCOME.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach.** Extract `refuseWorldIncapablePath` and call it on the REQUESTED
world name, positioned after the duplicate/unknown 400s (config errors, true of
every route) and before every arm that consults the grant. That boundary is
exactly where the response stops depending on config and starts depending on who
is asking.

Keyed on the name, not a resolved handle: a denied handle carries the zero scope
and a zero scope IS the default world, so `handle.isDefault()` cannot tell "no
world asked for" from "a world asked for and refused" — the trap
`freeTextIDsForType` already documents at its own seam.

**Tests.** `TestAttachWorld_DeniedWorldRefusedLikePermitted` asserts the
permitted and denied responses are IDENTICAL on five refused routes, which
catches the disclosure and the grant oracle in one assertion and keeps failing
for any future divergence. Verified to fail on all five subtests against the
unfixed code. Three companions pin the boundaries: the denied handle must still
reach world-capable routes (or the empty-result design that closes the existence
oracle is lost), the default world must pass through, and the config-error
precedence must not regress.

**Related areas checked.**

- `?q=` on a world-capable route with a denied world — clean. `scopedSortedEntities` returns early on `blocksAllReads`, and `freeTextIDsForType` fails closed independently at its own seam. This was the specific concern raised in issue #1436 and it does not reproduce.
- `_views/{type}/{id}` — clean. `viewEntry` handles `w.denied` explicitly, returning `errNoFaceInWorld` byte-identically to a permitted world holding no face. An apparent leak during investigation was the `resolveDefault: true` test stub resolving the world to the entity's default state, i.e. the entity genuinely being in that world; permitted and denied responses are identical.
- `_history`, `_next_action` — the other two world-capable underscore routes. Neither diverges between the grant outcomes.
- The four `blocksAllReads` call sites are the complete set of seams that need it, because after this fix a denied handle can only reach world-capable routes.
