---
id: RR-IJH9FS
type: review-response
title: AC 2 names a test table that does not exist
finding: allowlist_test.go has no 'editable table'. The data-entry edit cases are TestCheckEdit_DataEntry (allowlist_test.go:199). Name it, and add a case that sets display under channels.*.messages.* and expects no violation. The allowlist line itself fits beside channels.*.messages.*.fields.[].** (allowlist.go:184).
severity: nit
resolution: AC 2 now names TestCheckEdit_DataEntry with a display case that expects no violation, and step 2 places the allowlist line beside channels.*.messages.*.fields.[].** (allowlist.go:184).
status: addressed
---
