---
id: RR-Z4921BG
type: review-response
title: "The pile page still offers rename, delete and remove in a read-only workspace"
finding: "The entity page and list hide Add to pile in a read-only workspace, but the pile page keeps its rename, delete and remove controls, which the server refuses."
severity: significant
resolution: "The pile page uses the same writable flag; a vitest pins it."
status: addressed
---
