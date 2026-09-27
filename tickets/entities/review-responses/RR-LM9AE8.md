---
id: RR-LM9AE8
type: review-response
title: versionschema.go comment says swept versions always carry version-sweep
finding: internal/sqlitedb/versionschema.go still said sweep-captured rows carry the system principal and the editor is only in the audit log.
severity: minor
resolution: Comment rewritten to describe copying last_edited_by_* with the version-sweep fallback.
status: addressed
---
