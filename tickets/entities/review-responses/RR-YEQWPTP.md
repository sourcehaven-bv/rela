---
id: RR-YEQWPTP
type: review-response
title: SharedBase.QueryScopes has no production caller
finding: dataentrywire passes the package-level QueryScopes, so the WithClock claim about query scopes holds only for hosts that opt in.
severity: minor
resolution: WithClock godoc now says the clock reaches query scopes resolved through SharedBase.QueryScopes and that the package-level function stays on the wall clock.
status: addressed
---
