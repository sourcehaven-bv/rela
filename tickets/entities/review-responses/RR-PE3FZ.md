---
id: RR-PE3FZ
type: review-response
title: emitURLFromMap duplicates buildVerifiedURL
finding: internal/lua/urls.go:247-263 emitURLFromMap duplicates the body of buildVerifiedURL almost exactly. Unify by having buildVerifiedURL accept map[string]string and making a small LTable→map adapter at the boundary. One fold step, one verification path.
severity: nit
reason: Duplication between emitURL and emitURLFromMap is noticeable but small (~10 lines). Unifying would require restructuring how the Lua table folding interacts with the URL verification — net churn for nit gain. Leaving for a future cleanup pass if urls.go grows further.
status: deferred
---
