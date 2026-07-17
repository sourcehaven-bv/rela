---
id: RR-CY6WYR
type: review-response
title: everyone / read:["*"] must be reported once globally, not per enumerated principal
finding: The `everyone` role is appended to EVERY principal's attributions unconditionally (resolver.go computeGlobals), and read:["*"] matches all types (roleGrantsRead/grantsVerb treat literal "*" as match-all). If any effective role (especially everyone) grants the verb via "*", readQuery returns AllowAll for every principal including unauthenticated. The tool must NOT fabricate a principal named `everyone` nor enumerate all principals in this case; it must detect policy.Roles["everyone"] granting the verb and report 'everyone (all principals, incl. unauthenticated)' ONCE, globally. Otherwise output is both wrong-shaped and redundant.
severity: significant
resolution: 'Plan specifies: detect policy.Roles[everyone] granting the verb (incl. via "*") and print ''everyone (all principals, incl. unauthenticated)'' once, globally; do not fabricate an everyone principal or enumerate all principals. Acceptance criterion + a test asserting the single global line (not N rows) added.'
status: addressed
---
