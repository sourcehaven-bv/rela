---
id: RR-CR-TOCTOU
type: review-response
title: 'TOCTOU between shell variant selection and the /_custom/ fetch is real and undocumented'
finding: 'selectShell stats the files, then a separate request fetches them. Between those the operator can delete custom.css (shell references a URL that 404s) or an editor can leave a half-written custom.js (served, throwing a syntax error into the SPA document). Not a security issue — the allowlist holds regardless — but the precomputed-variants design removes only the CACHE-POPULATION race, and the next reader would reasonably assume it removed this one too.'
severity: minor
status: addressed
resolution: "Accepted as consistent with the feature's stated 'if it breaks, you keep the pieces' contract, and documented explicitly in the customAssetExists godoc so nobody mistakes it for an oversight."
---

Raised by `/code-review` of the TKT-3DBK6I implementation.
