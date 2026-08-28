---
id: RR-DR-HTMLCSP
type: review-response
title: Reusing appContentTypes serves operator .html as a live, unauthenticated, same-origin page with
  no CSP
finding: 'Decision 2 says ''serve everything, use the map only for Content-Type'', reusing appContentTypes
  wholesale. VERIFIED: apps.go:207-208 maps .html/.htm to text/html. VERIFIED: apps_handler.go:125 sets
  a path-scoped Content-Security-Policy on every /_apps/ response; custom_handler.go sets only Content-Type,
  nosniff and Cache-Control - NO CSP. So custom/page.html would be served as HTML, same-origin with the
  API, no CSP, no sandbox, no iframe, and (per decision 3) no authentication. An attacker who gets a victim
  to open https://host/_custom/page.html executes script on rela''s origin with the victim''s session
  cookie against /api/v1/*. That is a stored-XSS-equivalent primitive that the two-file design structurally
  could not have, because its only two outputs were text/css and text/javascript chosen by exact literal.
  The ticket''s defence (''custom.js is fully trusted anyway, so the map protects nothing'') is the WRONG
  COMPARISON and is the load-bearing error in decision 2: custom.js is trusted because the operator deliberately
  WIRED it into their shell; an HTML file merely PARKED in a folder is not an act of wiring, yet becomes
  an executable document on the origin. Different consents.'
severity: critical
status: wont-fix
resolution: 'REJECTED after maintainer challenge. The finding identifies a real MECHANISM (.html is in
  appContentTypes; apps/ sets a CSP at apps_handler.go:125 and /_custom/ does not) but draws the wrong
  CONCLUSION. custom.js is injected into the SPA''s own document, same-origin, no CSP - it can already
  read the session cookie and reach every API endpoint. An HTML page on that same origin reaches exactly
  the same capability, so serving text/html adds NO new privilege. There is no boundary between ''script
  in the shell'' and ''script in a page on the same origin'' because it is the same origin. The delivery-vector
  argument also fails: an attacker who can write custom/page.html can write custom.js instead, which is
  already injected and runs for every user on every page load without needing anyone to visit a URL -
  a strictly better position. So the proposed fix defends against nothing, while ''Content-Security-Policy:
  sandbox'' on the whole route would break an operator legitimately serving an HTML page from their own
  folder. Decision 2 stands UNAMENDED: reuse appContentTypes wholesale, including .html. My error was
  verifying the mechanism (would HTML execute?) and treating that as confirming the threat, without asking
  whether execution added any capability that custom.js did not already provide.'
reason: 'No new capability. custom.js is injected into the SPA''s own document, same-origin with no CSP,
  so it can already read the session cookie and call every /api/v1/* endpoint as the logged-in user. An
  HTML page served from the same origin reaches exactly that and nothing more - there is no privilege
  boundary between ''script in the shell'' and ''script in a page on the same origin'', because it is
  the same origin. The finding''s mechanism is real but its conclusion does not follow.


  The delivery-vector argument also fails. It supposed an attacker luring a victim to /_custom/page.html.
  But writing that file requires write access to the project directory, and anyone with that access writes
  custom.js instead - already injected, runs for every user on every page load, needs no click. The lure
  is strictly worse for the attacker, so closing it removes nothing.


  Cost of the proposed fix is real: ''Content-Security-Policy: sandbox'' on the whole route, or mapping
  .html to text/plain, would break an operator legitimately serving their own HTML page from their own
  folder - in exchange for zero security gain.


  Root cause of my error: I verified the finding''s PREMISE (is .html in appContentTypes? does apps/ set
  a CSP that /_custom/ lacks? both yes) and treated that as verifying its CONCLUSION. The question I failed
  to ask was whether HTML execution granted any capability custom.js did not already grant. It does not.
  Recorded because ''the mechanism is real'' is exactly how a plausible-but-empty security finding survives
  review.'
---

Raised by `/design-review` of TKT-IWMETE before implementation, and REJECTED after the maintainer challenged it.

## Why this was rejected rather than fixed

The reviewer correctly identified that `.html` sits in `appContentTypes` (`apps.go:207-208`) and that `apps/` sets a path-scoped CSP (`apps_handler.go:125`) which `/_custom/` does not. Both facts were verified. What neither fact establishes is a *new* capability.

| Surface | Origin | Can read session cookie | Can call /api/v1/* |
|---|---|---|---|
| `custom.js` (today, shipped) | rela | yes | yes |
| `custom/page.html` (proposed) | rela | yes | yes |

Identical. The `apps/` comparison misleads because an app is *untrusted, installable, third-party* content that must be confined; `custom/` is the operator's own directory, and its `custom.js` is already unconfined by design (see the trust-model godoc in `custom.go`).

Decision 2 therefore stands unamended, `appContentTypes` is reused wholesale including `.html`, and AC9 was dropped from the ticket.
