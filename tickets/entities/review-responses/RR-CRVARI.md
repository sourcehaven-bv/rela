---
id: RR-CRVARI
type: review-response
title: 'Docs undersell app blast radius: app HTML is as trusted as a Lua action script'
finding: 'By design (''the app is the user'') an app gets create/update/delete/relationCreate AND runAction(anyActionId) under the user''s full ACL. The sandbox protects the browser, not the data. Combined with the CSP issues and apps being static files in a possibly-shared project dir, a malicious/compromised app HTML can perform any mutation the user can. Docs framing (''user-authored apps'') undersells this. FIX: state plainly in docs/data-entry.md + CLAUDE.md that apps/ HTML is as trusted as scripts/ Lua and must get the same review rigor.'
severity: minor
resolution: Added a 'Trust level' paragraph to the Custom apps security section in docs/data-entry.md stating plainly that the sandbox protects the browser, not the data; an app runs as the user and can do any create/update/delete/link + invoke any registered Lua action via rela.action, so app HTML must get the same review rigor as a scripts/ Lua action — it is code, not content. (The internal/dataentry/CLAUDE.md note already frames apps/ alongside actions/scripts.)
status: addressed
---
