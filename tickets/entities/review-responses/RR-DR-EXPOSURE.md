---
id: RR-DR-EXPOSURE
type: review-response
title: The plan under-rates the exposure asymmetry it creates with apps/, its own template
finding: 'The plan''s routing claim is independently VERIFIED correct: /_custom/ is registered on the
  outer mux (router.go:128), isAPIPath (router.go:216-218) matches only /api/ and /api, so requireVerifiedJWT
  and attachACLRequest both no-op, and sensitivePathPrefixes is /api/ only so it does not even get the
  same-origin check. Fully unauthenticated. The gap is the ACCOUNTING: the template being copied, /api/v1/_apps/,
  is behind the JWT gate and ACL precisely because it is under /api/. So this copies apps/''s path-validation
  while DROPPING apps/''s authentication, and the risk section rates the result MEDIUM with ''residual
  risk accepted''. For apps/ the equivalent exposure is authenticated-only; here it is the open internet.'
severity: significant
status: addressed
resolution: 'Folded into TKT-IWMETE and PLAN-6VVJJZ before implementation. Keep it unauthenticated - fonts
  and logos referenced from custom.css must load before login or the login page renders unstyled, so gating
  breaks the feature''s point. Fix the RECORD instead: (a) docs must say plainly this folder is MORE exposed
  than apps/, which operators will otherwise assume is equivalent since the ticket sells apps/ as the
  template; (b) restate the residual risk as ''readable by anyone on the internet who guesses the filename'',
  not the current softer phrasing; (c) put AC6''s ''do not put anything here you would not publish'' ABOVE
  the folder-layout example in docs/customisation.md, not in the bottom Notes block where the now-false
  ''only these two exact filenames'' claim currently lives.'
---

Raised by `/design-review` of TKT-IWMETE before implementation.
