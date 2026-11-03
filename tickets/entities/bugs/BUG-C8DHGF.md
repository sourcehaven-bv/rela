---
id: BUG-C8DHGF
type: bug
title: Data API request without identity gets 500 instead of 401
description: A data API request with no identity is answered 500 instead of 401.
priority: low
effort: s
why1: attachACLRequest maps every acl.ErrUnstampedPrincipal from access.Opener.Open to 500 acl_unstamped_principal.
why2: ChainResolvers falls back to the principal unknown/data-entry when no resolver names a user, so a request without credentials and a stamper fault reach the edge as the same error.
why3: The RR-875A fail-loud fix treated every unstamped principal as server misconfiguration. It was written for the case where the stamper did not run at all and never separated a missing credential from it.
why4: BUG-O51NS5 moved the no-identity answer below the edge to 401 (writeNoIdentity) but left the edge mapping unchanged, so the two layers disagreed.
why5: No test sent an anonymous request through the production resolver chain and asserted the intended status. The golden corpus recorded the 500 as a known gap, which documented it but did not drive a fix.
prevention: TestACLMiddleware tests pin 401 for an unnamed stamp and 500 for a named principal that fails to resolve. The decl-header/anonymous golden now records 401 through the production resolver chain / so a regression changes a golden file.
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

Under a Declarative ACL with header (or env) identity, a data API request that
carries no identity is answered 500 `acl_unstamped_principal`
(internal/dataentry/router.go:482) instead of 401. `wirePrincipalResolvers`
chains env and header; when both miss, nothing stamps a principal and the gate
reports an internal error. The request is refused (fail-closed), but a missing
credential is a client error, and a 500 hides it among real faults in logs and
monitoring.

Recorded by the TKT-0M4H0S golden parity corpus for all 98 anonymous cases.

## Fix

Return 401 with a problem body naming the missing identity when no resolver
yields a principal; keep 500 for a genuine stamping fault. Pin with a test per
identity mode; update the corpus anonymous recordings.
