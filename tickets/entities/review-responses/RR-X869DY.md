---
id: RR-X869DY
type: review-response
title: /api/sync CSRF exemption was exploitable under a cookie-mode OAuth proxy
finding: The blanket same-origin exemption for /api/sync/ (middleware_security.go) removed the ONLY CSRF defense in the stack. rela has no app-layer auth — identity is the proxy-set X-Forwarded-User. If the fronting OAuth proxy authenticates the browser by SESSION COOKIE (the common oauth2-proxy/Pomerium/Authelia pattern), a malicious page can cross-origin fetch() PUT /api/sync/ with credentials:include; the browser attaches the proxy cookie + correct Host, the proxy injects X-Forwarded-User=victim, and the write lands as the victim — textbook CSRF. The Host check does NOT stop this (attacker hits the real hostname). The first-create path needs no If-Match, so it's exploitable with no hash knowledge. The exemption made a security guarantee contingent on an unstated, unenforced deployment property.
severity: critical
resolution: 'Replaced the blanket path exemption with an Origin-aware predicate (isCSRFExempt): /api/sync/ skips same-origin ONLY for a request that carries NO Cookie AND NO Origin/Referer — provably a non-browser client (the CLI). A CSRF request always carries a cookie (and Origin), so it is NOT exempt and gets the normal same-origin rejection. The Host check still always runs. Regression TestSync_CSRFExemptionRequiresNoCookie asserts a cookie-bearing and a cross-origin /api/sync write are both 403; TestSync_SameOriginExemption asserts the bare CLI (no cookie/origin) is admitted. (Folds C2: the exemption is no longer a blanket subtree strip.)'
status: addressed
---
