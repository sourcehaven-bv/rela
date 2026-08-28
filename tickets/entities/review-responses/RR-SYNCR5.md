---
id: RR-SYNCR5
type: review-response
title: 'CRITICAL: sync /api/v1 requests are 403d by same-origin middleware — CSRF exemption does not cover the v1 data routes sync now uses'
finding: 'The fancy-browser rewrite moves all sync reads/writes onto /api/v1/{plural}/... plus /api/v1/_schema. The sync client sends only Authorization Bearer (no Origin, no Cookie, no Sec-Fetch-Site). But requireSameOrigin (middleware_security.go) gates /api/ as sensitive, and nonBrowserExemptPrefixes lists only /api/sync/, /api/v1/_feeds/, /api/v1/_caldav/ — NOT /api/v1/{plural}/... or _schema. So isCSRFExempt returns false for the sync client paths and requireSameOrigin rejects origin_missing (403). rela-server always mounts this middleware, so against a real proxy-fronted server every sync request 403s starting at the first /api/v1/_schema fetch — the feature does not work against its documented deployment target. TestSync_SameOriginExemption even asserts GET /api/v1/tickets/TKT-001 with no Origin returns 403 as its control, certifying the broken behavior; the CLI fakeServer has no security middleware so it never exercised this.'
severity: critical
status: addressed
resolution: 'The isCSRFExempt provably-non-browser test (no Sec-Fetch-Site, no Cookie, no Origin/Referer) is the actual CSRF safety property; the path list only scopes WHERE that relaxation applies. A bare bearer-only request to a v1 CRUD route is exactly as CSRF-safe as one to /api/sync/ — a browser can never produce that shape. Fix: added a dedicated helper isSyncExemptV1Path covering the specific v1 routes the sync client uses (the plural/{id} data routes, their /relations subpaths, and /api/v1/_schema) to nonBrowserExemptPrefixes semantics, so the exemption still requires the provably-non-browser signal (unchanged) and does NOT blanket-exempt anything a browser reaches with a cookie. TestSync_SameOriginExemption control flipped: a no-Origin no-Cookie no-Sec-Fetch GET on a v1 sync path is now reachable (not 403); a Cookie/Origin/Sec-Fetch-bearing request to the same path is still same-origin gated (new assertions). End-to-end test drives the real router-with-security for a v1 GET and a v1 PATCH.'
---

## Finding (code-review, fancy-browser final)

See frontmatter. The blocker is that moving sync onto `/api/v1` lost the
`/api/sync/`-scoped CSRF exemption without re-establishing it for the v1 routes
sync now speaks. The exemption is not a weakening — `isCSRFExempt` still requires
the provably-non-browser shape (no `Sec-Fetch-Site`, no `Cookie`, no
`Origin`/`Referer`), which a browser cannot forge — it only needs the *path* to
be in scope.

## Recommended resolution

Extend the non-browser-exempt path scope to the exact `/api/v1` routes the sync
client uses, keeping the `isCSRFExempt` conditioning intact. Do NOT blanket-exempt
all of `/api/v1` — that is the SPA's cookie-authenticated surface. Flip the
`TestSync_SameOriginExemption` control and add an end-to-end reachability test
through the real security middleware for a v1 GET and a v1 PATCH.

## Other findings from the same /code-review (all addressed in the same pass)

- **#2 (significant):** `CheckSchemaCompatible` only checked property NAME
  presence while the doc promised shape-drift protection. FIXED: the handshake
  now compares property **shape** (value type + list-ness), decoded from
  `/api/v1/_schema`'s `PropertyDef`. Pinned by `TestCheckSchemaCompatible`
  (type-drift, list-drift, missing-prop, remote-only-extras-OK).
- **#3 (minor):** `recordCreate` swallowed a post-rename read error, leaving
  `Local=""` and forcing a spurious re-push. FIXED: the error propagates; a
  re-run resumes under the minted id.
- **#4 (minor):** `ForcePush` of an unindexed-but-remotely-present record took
  the CREATE branch (duplicate mint). FIXED: create only when `!indexed AND
  Base==""`; a non-empty base (record exists remotely) PATCHes under If-Match.
- **#5 (nit):** vacuous splice non-mutation assertion → now snapshots `prior`
  and deep-compares; stale `/api/sync/entities` example in `client.go` → updated
  to `/api/v1`.
- **Also caught in CI pass:** the fakeServer's minted-id prefix used
  `ToUpper(x[:1])`, tripping the label-derivation lint (DEC-6C1NAA) — rebuilt
  from the plural instead. And the `/api/v1` CSRF fix's first cut would have let
  a sandboxed-iframe `Origin: null` cross-origin POST ride the exemption; the
  Origin check was tightened to reject a *present* Origin header (a bare CLI
  sends none), closing that vector.
