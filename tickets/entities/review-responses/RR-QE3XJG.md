---
id: RR-QE3XJG
type: review-response
title: 'Test gaps: valid-JWT-beats-spoofed-header unpinned; gate tests bypass the real NewRouter composition'
finding: Three gaps. (1) TestRequireVerifiedJWT_SpoofedHeaderDoesNotAuthenticate only covered the no-valid-token case; the operator-intuitive case — a VALID assertion arriving alongside a spoofed header must resolve to the JWT subject — was unpinned. (2) Every gate test hand-composed stampAuditPrincipal(requireVerifiedJWT(...)), so reordering the wraps in NewRouter would leave them all passing while breaking production. (3) No validateIdentityFlags case for an empty jwtHeader (because the check did not exist — see RR-39YRW3).
severity: minor
resolution: Added TestRequireVerifiedJWT_ValidAssertionBeatsSpoofedHeader. Added TestJWTGate_RouterChainOrder, which drives the REAL app.NewRouter() with a declarative ACL and asserts the verified subject is the principal ACL authorizes against, that unverified requests 401 before reaching ACL, and that the SPA shell stays reachable — following the existing TestACLMiddleware_RouterChainOrder precedent, which exists precisely because this bug class only appears at the composition site. Added the empty-header case to TestValidateIdentityFlags. Also added TestLogSampler for the new sampling logic.
status: addressed
---

Reported by cranky-code-reviewer. The reviewer credited
`TestRequireVerifiedJWT_OrderingRelativeToStamper` for asserting the *inverted*
wrap actually loses the subject, but correctly observed that a hand-composed
chain cannot catch a regression in `NewRouter` itself.

`TestJWTGate_RouterChainOrder` closes that: it is the composition-site test,
mirroring the existing ACL one.
