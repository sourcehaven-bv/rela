---
id: RR-KXMXLW1
type: review-response
title: "login_url could send the browser anywhere"
finding: "Any string in login_url was passed to location.assign, including javascript: URLs."
severity: significant
resolution: "Addressed: only a local path on the same site is accepted, which is what the edge sends; another host, //host, /\\host and a path that normalises to //host (such as /.//host) are refused, and the absolute same-origin URL is what is followed. Pinned by workspaceAccess.test.ts."
status: addressed
---
