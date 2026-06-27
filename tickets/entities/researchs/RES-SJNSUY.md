---
id: RES-SJNSUY
type: research
title: 'Authentication architecture: identity proxy vs app-side JWT vs full in-app OIDC'
summary: Survey of how rela should authenticate (web + API + CLI). Recommends app-side signed-JWT verification (Sec-Fetch CSRF + Bearer on machine routes) keeping the proxy for login — NOT a full in-app OIDC RP. Forgejo/Gitea show owning auth is a permanent CVE-bearing subsystem justified only when login IS the product.
status: done
---

## Problem

rela has **no application-layer authentication** by design (`docs/security.md`:
"intentionally no login; trust boundary is the proxy"). It runs behind an
identity-aware reverse proxy (oauth2-proxy / Pomerium) that authenticates the
user and injects an `X-Forwarded-User` header the app trusts.

Building the sync HTTP API (TKT-PV0R3V) exposed the friction: because the app
has no notion of its own auth, defending `/api/sync/` from CSRF required a
Sec-Fetch-Site heuristic, and the "trust a header" boundary is a silent,
un-enforced assumption. Question raised: should rela ship its own OAuth/OIDC and
drop the proxy?

## Context

Two facts frame the decision:
1. **The current model and "verify a signed assertion" are nearly the same code.** The same proxies rela supports (Pomerium, Cloudflare Access, Google IAP) inject a *cryptographically signed JWT* the app can verify in ~60-120 LOC; their own docs say you SHOULD ("If an attacker bypasses IAP… signed headers provide secondary security… you must use signed headers" — Google IAP).
2. **CSRF on a Bearer-only API route is eliminated by construction** (OWASP CSRF Prevention): browsers don't auto-attach `Authorization`, so a token-required route isn't CSRF-able — *if* it refuses cookie auth.

## Options

### A — App-side Bearer/JWT validation; keep proxy for browser login
App verifies a Bearer JWT itself (signature vs IdP JWKS, `aud`/`iss`/`exp`),
requires it on machine routes. Proxy still does interactive login.
- **Security:** closes forged-header impersonation on API routes; eliminates API CSRF (Bearer-only). Must get right: alg-allowlist (no `alg:none`/RS256-as-HS256 — RFC 8725), `aud` validation, access-token-not-ID-token. These map to single config flags.
- **Scope:** ~60-120 LOC with `coreos/go-oidc/v3` (cached auto-rotating `RemoteKeySet`; signature+`iss`+`aud`+`exp` verified by default; alg-allowlist baked in). Library owns JWKS rotation.
- **Maintenance:** JWT-library CVE patching + minor IdP quirks. Modest.
- **CLI:** cleanest — device flow (RFC 8628) → Bearer (the `gh auth login` / `kubectl oidc-login` model). No app-side credential store.

### B — Full OIDC relying-party IN the app; drop the proxy
App owns the entire flow: auth-code+PKCE for browsers, device flow for CLI,
session cookie crypto, CSRF tokens, JWKS rotation, refresh, logout,
multi-provider.
- **Security:** largest surface owned. Footgun list (each a silent failure): `state`/CSRF on callback, exact `redirect_uri` match, open-redirect, session fixation, token storage (not localStorage), nonce replay, mix-up attacks, PKCE. Evidence it's hard: CyberArk study of 100 OAuth sites — 21% mis-verify `state`, 28% mis-validate `redirect_uri`, in production.
- **Scope:** open-ended, continuously maintained. Go libs (go-oidc, zitadel/oidc, goth) remove token-verify + code-exchange but explicitly leave sessions/CSRF/logout/refresh/storage to you. (ory/fosite is a *provider*, wrong side.)
- **Maintenance:** highest, forever. Exactly the surface every identity-proxy vendor pitches removing.
- **CLI:** app must build device endpoints OR become a PAT-issuing identity provider — contradicts "don't own a large security surface."

### C — Keep the proxy; harden the trust boundary
Keep authN at the proxy; make "app reachable only via proxy, trusts header"
enforceable not assumed.
- **The fix:** verify a *signed* assertion the proxy injects (Pomerium `X-Pomerium-Jwt-Assertion`, Cloudflare `Cf-Access-Jwt-Assertion`, IAP `x-goog-iap-jwt-assertion`) — same ~60-120 LOC as A. Plus network-boundary + header-strip-with-normalization as the floor.
- **Why it matters:** plaintext-header trust is a live CVE class — Grafana CVE-2022-35957 (SSRF reaches localhost with forged `X-WEBAUTH-USER`), oauth2-proxy CVE-2026-40575 (CVSS 9.1, defaults to trusting 0.0.0.0/0), underscore-smuggling GHSA-vjrc-mh2v-45x6. A signed assertion means one boundary slip ≠ total bypass.
- **CLI:** the weak point of the *plain* model — no clean story; needs `--skip-jwt-bearer-tokens` or a proxy-native machine credential.

## Forgejo / Gitea evidence (the closest real-world analog that DID build its own auth)

Forgejo/Gitea are self-hosted Go apps with web UI + REST API + git/CLI clients
that **own their entire auth stack** — local password, sessions, CSRF, OIDC
relying-party AND provider, LDAP/PAM/SSPI, and a hand-rolled PAT system. What it
costs them:
- **Two forked, self-maintained libraries** (`gitea.com/go-chi/session`, `gitea.com/macaron/csrf`) because the upstreams were abandoned — permanent liability.
- **PAT storage churned through three designs** (plaintext → PBKDF2 → SHA-256) and PBKDF2 needed a perf cache (`SUCCESSFUL_TOKENS_CACHE_SIZE`) under API load — the hidden cost of owning token storage.
- **A continuous auth-CVE stream, mostly authorization/scope-enforcement** (not crypto): 2026 GHSAs for OAuth2 scope bypass via Basic auth, Git Smart-HTTP skipping repo scopes for Bearer tokens, an *incomplete-fix* re-issue (CVE-2025-68941), cross-org authz bypass, unauthenticated container-registry pulls (CVE-2026-27771, Critical); historically CVE-2023-1935 OAuth2 PKCE `state` bypass.
- **Verdict:** they legitimately own auth because for a forge **login IS the product** — git-over-HTTPS uses token-as-password (a proxy can't satisfy a `git clone https://token@host`), and they're a downstream IdP other tools log in through. rela has the OPPOSITE shape: a *consumer* of identity, not a source. Forgejo is the strongest example of the *cost*, not a precedent to copy.

## Recommendation

**Do C+A together — they are the same small piece of code, and that's the sweet
spot. Explicitly avoid B.**

1. **Keep the proxy for browser login** — don't take on B's surface. The CyberArk failure rates + named CVEs + Forgejo's CVE stream are the argument; the entire identity-proxy industry exists to keep this out of app code.
2. **Make the boundary cryptographic (C upgraded):** rela verifies a *signed* assertion the proxy already injects (~60-120 LOC, `go-oidc/v3` RemoteKeySet → check `iss`/`aud`/`exp`). Converts fragile "trust a header" into "verify a signature"; a deployer's network slip is no longer a full bypass.
3. **Solve the CLI with the same verifier (A):** require a Bearer JWT on machine/API routes, obtained via device flow. This also eliminates API CSRF by construction (OWASP) — superseding the Sec-Fetch heuristic shipped in TKT-PV0R3V. Avoid the PAT route (would make rela an identity provider — Forgejo shows that's where the engineering and CVEs live).

Net: one small, well-understood JWT-verification middleware (whether the JWT is
minted by the IdP for the CLI or by the proxy for the browser); no in-app login
flow, no session crypto, no JWKS-rotation maintenance beyond the library; a
clean CLI device-flow story; a signature-enforced trust boundary.
Minimum-surface way to get everything B would give without owning the auth
stack.

**This is the FEAT-ESLP hardening direction.** The Sec-Fetch CSRF fix in
TKT-PV0R3V is correct for the CURRENT trust model and stands until A+C lands.
Suggest a follow-up ticket: "app-side signed-JWT verification on /api/sync/ (+
require Bearer on machine routes)".

## Sources
OWASP CSRF/Session/JWT cheat sheets; RFCs 9700 (OAuth Security BCP), 7636
(PKCE), 8628 (device flow), 8725 (JWT BCP); coreos/go-oidc/v3, zitadel/oidc,
markbates/goth, ory/fosite; Pomerium/Cloudflare Access/Google IAP signed-header
verification docs; oauth2-proxy request-signatures + CVE-2026-40575 +
GHSA-vjrc-mh2v-45x6; Grafana CVE-2022-35957; CyberArk 100-site OAuth study;
Gitea/Forgejo `services/auth` + security advisories (2024-2026).
