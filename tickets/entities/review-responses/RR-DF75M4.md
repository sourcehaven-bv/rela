---
id: RR-DF75M4
type: review-response
title: Deps.validate rejects nil reader for always-create hooks
finding: Deps.validate rejects nil reader for always-create hooks
severity: minor
reason: Production never passes a nil reader (scriptReader returns DenyReader). Failing closed up front is the constructor rule in CLAUDE.md. luaDeps is now built once per delivery instead of per find; that is the same work for any hook that finds.
status: wont-fix
---
