---
id: RR-S76E6N
type: review-response
title: Dead status field in useAutoSave.parseError
finding: parseError still returned { status, message } but no caller read .status after the migration — a one-line wrapper around getErrorMessage.
severity: minor
resolution: Deleted parseError; the four call sites call getErrorMessage(err, 'Save failed') directly.
status: addressed
---
