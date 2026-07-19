---
id: RR-C3OJ33
type: review-response
title: applyCreateLock re-leaks a hidden machine field on create
finding: 'In handleV1DryRunCreate, forWire runs stripHiddenProperties then applyCreateLock does result.Properties[prop] = entry unconditionally for every machine field. If the machine field is hidden, stripHiddenProperties removed it from result.Properties, then applyCreateLock re-inserts the entry value — re-leaking it on the wire AND (because the SPA derives create-mode visible fields from Object.keys(candidate.properties) in DynamicForm.refreshStagedAffordances) making the hidden field re-appear in the create form. Same root cause as the GET leak: the lock ignores visibility. Fix: skip locking (and pinning) a field that is not visible.'
severity: critical
resolution: applyCreateLock now resolves FieldVerdicts and skips (does not pin/lock/re-insert) any machine field where !IsVisible(prop), so a hidden field stays stripped. Regression test TestTransitionsWire_CreateLockSkipsHiddenField asserts the hidden field's entry value is not re-added to the dry-run response.
status: addressed
---
