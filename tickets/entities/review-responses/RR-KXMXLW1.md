---
id: RR-KXMXLW1
type: review-response
title: "login_url could send the browser anywhere"
finding: "Any string in login_url was passed to location.assign, including javascript: URLs."
severity: significant
resolution: "Addressed: only https, or http on an http page, is accepted. Pinned by workspaceAccess.test.ts."
status: addressed
---
