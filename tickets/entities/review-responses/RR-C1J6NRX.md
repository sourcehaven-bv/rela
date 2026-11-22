---
id: RR-C1J6NRX
type: review-response
title: "Sign in again did nothing on a URL with a fragment"
finding: "location.assign(location.href) with a #fragment only jumps within the page and never reaches the edge."
severity: significant
resolution: "Addressed: with no login_url the button reloads. Pinned by WorkspaceAccessScreen.test.ts."
status: addressed
---
