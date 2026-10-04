---
id: RR-EHD12P
type: review-response
title: Page script could approve keychain secrets
finding: ProjectSettings.TrustSecrets is a bound service any page can call; a document's custom.js could approve itself and its Lua would read the user's secrets.
severity: critical
resolution: TrustSecrets now asks in a native dialog (Desktop.confirmDialog) that page script cannot answer; nil confirm refuses. Tested in settings_sqlite_test.go.
status: addressed
---
