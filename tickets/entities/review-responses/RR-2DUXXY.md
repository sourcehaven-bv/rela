---
id: RR-2DUXXY
type: review-response
title: Page could edit another open project's settings
finding: ProjectSettings methods took a project id from the page, so a document's custom.js could rewrite another open trusted project's ai.yaml (sending its AI key elsewhere) or delete its secrets.
severity: critical
resolution: Go pins each settings window to its project when it opens it; methods take the Wails call context and refuse any caller that is not a registered settings window. Tested in TestProjectSettings_RefusesOtherWindows.
status: addressed
---
