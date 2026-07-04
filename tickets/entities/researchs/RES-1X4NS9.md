---
id: RES-1X4NS9
type: research
title: How should rela calendar feeds be authenticated — ICS-subscribe vs read-only CalDAV, and does OAuth2 fit?
summary: 'OAuth2-for-CalDAV is provider-specific, not a self-hosted client capability: Apple authenticates custom CalDAV with Basic/Digest and Google won''t consume third-party CalDAV, while ICS-subscribe Basic-in-URL is broken on iOS. Recommend loopback-only feeds for Phase 1 and a capability-token-in-URL follow-up for remote/proxy access; drop the redundant fail-loud guard; keep CalDAV Phase 2 (and not for auth reasons).'
status: done
---

## Problem

How is a rela calendar feed **authenticated** for the two real deployments — (1)
localhost-bound single-user PIM, and (2) behind an authenticating reverse proxy
that runs OAuth2/OIDC and injects `--principal-header` — and does that change
the **transport** (one-way ICS subscription vs. a read-only CalDAV endpoint)?

This supplements RES-AHY3VS, which assumed one-way ICS + "proxy authenticates
the poller." That assumption was never verified. The hard fact that broke it: a
**subscribed ICS feed is fetched by a headerless background poller** (Apple
`dataaccessd`, Google's fetcher) that cannot run an interactive OAuth2 flow and
generally cannot set custom headers — so an OAuth2 proxy in front of rela
**blocks** it. The user's question — "does CalDAV allow OAuth2 flows?" — is the
right one to chase, because if CalDAV unlocks OAuth2 it could reframe the whole
transport choice.

## Context

Verified findings (web sources + codebase). The decisive results **invert** the
earlier assumption that CalDAV buys us an OAuth2 story.

### Finding 1 — What each transport can actually authenticate with

| Transport | Apple Calendar (macOS/iOS) | Google Calendar |
|---|---|---|
| **ICS URL subscription** | Basic-in-URL (`user:pass@host`) is **broken on iOS** — the `Authorization` header is not sent (persistent since iOS 11). Unreliable. | Cannot subscribe to an arbitrary authenticated ICS URL as a user account; public/unauthenticated ICS only. |
| **CalDAV account (custom server)** | **Basic or Digest only.** Apple's CalDAV client supports Basic/Digest against arbitrary servers; it does **not** do OAuth2 bearer against a self-hosted server. OAuth2 is used only for *provider* account types (iCloud). | Google's own CalDAV **requires OAuth2** and rejects Basic/HTTP — but that is for connecting *to Google's* CalDAV, **not** for Google Calendar acting as a client of *our* server. Google Calendar won't add an arbitrary third-party CalDAV account. |

The load-bearing correction: **OAuth2-for-CalDAV is provider-specific, not a
generic client capability.** Apple/Google speak OAuth2 to *their own* CalDAV
services via built-in account types. A **self-hosted** CalDAV server (which is
what rela would be) is authenticated by Apple's client with **Basic/Digest**,
and Google Calendar won't consume it as a user subscription at all. So standing
up a read-only CalDAV server does **not** deliver the clean OAuth2 flow — it
delivers Basic/Digest, the same credential problem ICS-subscribe has, plus a
much larger protocol surface (PROPFIND / REPORT / ETags / sync-collection /
WebDAV).

### Finding 2 — Read-only CalDAV is still a real protocol, and doesn't solve auth

RFC 4791: a CalDAV server must implement WebDAV `OPTIONS`, `PROPFIND`, `REPORT`
(`calendar-query`, `calendar-multiget`), `GET` with strong `ETag`s, and
advertise `supported-calendar-component-set`, `getctag`, `displayname`,
`resourcetype`. Even a **read-only** subset (no PUT/DELETE, no VTODO) is
materially more code than serving a static `.ics` byte stream — and per Finding
1 its clients still authenticate with Basic/Digest, so it does **not** buy the
OAuth2 story that motivated considering it. It remains the right Phase-2 home
for two-way sync, but it is **not** an auth upgrade over ICS.

### Finding 3 — The proxy deployment: no portable "let the poller through"

- `oauth2-proxy` has `--skip-auth-route`, but it opportunistically validates
cookies and, in ≤ 7.10.0, matched against the full URI including query params —
**CVE-2025-54576**, an auth-bypass fixed in 7.11.0. Using it to expose a feed
path is fragile and version-sensitive.
- There is no portable "accept a background client" answer across oauth2-proxy /
Authelia / Pomerium / Traefik ForwardAuth. Each needs its own carve-out (a
skip-auth route + a *separate* Basic/bearer check, or a signed-URL scheme). So
"the proxy authenticates the poller" is **operator-specific config rela cannot
promise**, not a clean default.

### Finding 4 — codebase (question 5, confirmed)

`cmd/rela-server/main.go` already warns on unsafe binds: `main.go:178` (bound
beyond loopback), `main.go:186` (`--principal-header` on non-loopback bind —
explicit hazard), and `main.go:192` + `shouldWarnNoACL` (`main.go:271`)
(non-loopback bind **without `acl.yaml`**). So the fail-loud guard proposed in
RR-7C151B (warn/refuse on non-loopback + no header + no `acl.yaml`) is
**redundant** — the third warning already covers exactly that condition. **Drop
it.** The feed inherits the existing warnings for free.

### Constraint recap

rela has **no login**; identity is the proxy-injected `--principal-header` + the
ACL read gate, and the feed path is CSRF-exempt via the existing
`nonBrowserExemptPrefixes` / `isCSRFExempt` mechanism (a background poller sends
no Sec-Fetch-Site / Cookie / Origin, so it is provably non-browser).

## Options

### A. ICS-subscribe + rela-issued capability token in the URL (revive C1)

rela issues an opaque per-feed token; the URL is `…/_feeds/tasks.ics?token=…`;
rela validates it → principal for the ACL read gate.
- **Clients?** Works uniformly — every client takes a URL, the token rides in the
query string (no header, no Basic, no `Authorization` bug). The **only**
mechanism that works on iOS/macOS/Google feed readers alike.
- **rela code:** moderate — hashed/revocable token store in `.rela/` + issue/list/
revoke CLI + token→principal lookup. (The store dropped earlier; the proxy
analysis brings it back as the only portable answer.)
- **Proxy fit:** good — self-contained token; proxy only skip-auths the path.
- **Throwaway?** No — the token→principal seam is what a Phase-2 CalDAV
`Authorization` path reuses.

### B. ICS-subscribe + proxy-side Basic / signed URL (operator config)

rela auth-free; operator configures the proxy.
- **Clients?** Partial — Basic-in-URL is **broken on iOS**; signed-URL is
proxy-specific.
- **rela code:** ~none. **Proxy fit:** poor/fragile (per-proxy, CVE-prone
skip-auth, iOS Basic bug). **Throwaway?** Pushes the problem to operators.

### C. Read-only CalDAV + OAuth2 bearer (bring Phase 2 forward)

- **Clients?** **No, not as hoped** — self-hosted CalDAV is Basic/Digest for
Apple, and Google won't consume a third-party CalDAV account. OAuth2 premise
fails.
- **rela code:** large (real WebDAV/CalDAV surface even read-only). **Proxy fit:**
same Basic/Digest problem behind a bigger protocol. **Throwaway?** Doing it *for
auth* is a false premise; keep it deferred.

### D. Loopback-only for Phase 1; defer network/proxy feed access

- **Clients?** Yes for local (calendar app on the same machine); nothing remote.
- **rela code:** minimal (existing loopback trust). **Proxy fit:** N/A by design.
- **Throwaway?** No — ships the real PIM need now, keeps the auth decision honest.

## Recommendation

**Ship D now, design for A next — and drop C as an auth motivation.**

- **Phase 1 (this ticket): loopback-only feeds (D).** The stated need is a local
PIM nudge; on `127.0.0.1` the calendar client and rela share the machine and the
existing loopback-trust model suffices — no token, no proxy, no new auth code.
Renderer + Lua + config + endpoint (behind the existing `isCSRFExempt` + ACL
read gate) are all built and useful immediately.
- **Remote/proxy access = capability token in the URL (A), as a distinct
follow-up ticket.** A is the *only* portable mechanism that works across
iOS/macOS/Google for a self-hosted feed (token in the query sidesteps the iOS
Basic bug and needs no per-proxy choreography). Reviving the token store is
justified now precisely because the proxy can't cleanly authenticate a
headerless poller. Design the Phase-1 handler with a **principal-resolution
seam** so token→principal is additive later, not a rewrite.
- **Drop the fail-loud guard (RR-7C151B)** — redundant with the existing
`shouldWarnNoACL` / bind-beyond-loopback warnings.
- **CalDAV stays Phase 2, and NOT for auth reasons** — read-only CalDAV doesn't
deliver OAuth2 for a self-hosted server. Build it later for *two-way sync*,
authenticated by Basic/Digest (or the same capability token).

**Tradeoffs accepted:** Phase 1 does not serve remote/networked subscriptions —
a deliberate scope cut matching the real use case, not a capability gap we
pretended to fill. Remote access becomes a small, portable, client-verified
capability-token addition on the seam left in place. We explicitly reject
chasing OAuth2 via CalDAV: for self-hosted servers that path is a mirage.

**Net effect on TKT-RDM9M5:** remove the proxy-trust auth section and the
fail-loud guard from the plan; scope the endpoint to loopback-trust for Phase 1;
add a principal-resolution seam; file the capability-token remote-access work as
a follow-up ticket.

## Sources

- Google CalDAV requires OAuth2, rejects Basic/HTTP:
https://developers.google.com/calendar/caldav/v2/auth ,
https://developers.google.com/workspace/calendar/caldav/v2/guide
- Apple iOS/macOS CalDAV custom-server = Basic/Digest:
https://www.webdavsystem.com/server/access/caldav/ipad_iphone_calendar/ ,
https://developer.apple.com/forums/thread/53284
- ICS-subscribe Basic-in-URL broken on iOS (Authorization header not sent):
https://github.com/nextcloud/calendar/issues/70 ,
https://developer.apple.com/forums/thread/82847
- CalDAV protocol surface (RFC 4791): https://www.rfc-editor.org/rfc/rfc4791.html ,
https://sabre.io/dav/building-a-caldav-client/
- Self-hosted CalDAV servers use htpasswd/Basic (Radicale/Baikal):
https://sabre.io/baikal/ ,
https://ossalt.com/guides/how-to-self-host-radicale-caldav-carddav-server-2026
- oauth2-proxy skip-auth-route + CVE-2025-54576:
https://oauth2-proxy.github.io/oauth2-proxy/behaviour/ ,
https://github.com/oauth2-proxy/oauth2-proxy/security/advisories/GHSA-7rh7-c77v-6434
