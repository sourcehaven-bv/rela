---
id: RR-44NG9VZ
type: review-response
title: Stale comments and an unused fixture
finding: 'engine.go:311 says the schema refuses an automation; entitymanager/audit_test.go:895 says no loader rejects a nameless automation; testutil WorkspaceMetamodelYAML still declares automations: and has no callers.'
severity: nit
resolution: Fixed the engine.go admits comment and the audit_test.go comment; deleted the unused testutil.WorkspaceMetamodelYAML.
status: addressed
---
