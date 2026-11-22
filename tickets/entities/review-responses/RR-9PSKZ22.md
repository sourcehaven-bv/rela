---
id: RR-9PSKZ22
type: review-response
title: "A server with no identity source said the session ended"
finding: "A standalone server that cannot identify anyone answers 401 with a detail; signing in again cannot help."
severity: significant
resolution: "Addressed: a 401 with a detail and no login_url is no_identity and shows the detail with Try again. Pinned by workspaceAccess.test.ts and WorkspaceAccessScreen.test.ts."
status: addressed
---
