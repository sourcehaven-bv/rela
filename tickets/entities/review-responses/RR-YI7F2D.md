---
id: RR-YI7F2D
type: review-response
title: 'Job-mark writer labels an integration: run_as as principal_type system'
finding: 'internal/realtime/outputs.go:392 stamps every run_as writer with PrincipalType "system", including `integration:<name>`. The writer is on the wire in OutputsView.writer. Clients that key on principal_type (frontend channelView.ts:89 already does for ''agent'') will mislabel an integration run_as. Fix: set PrincipalType from the prefix (system vs integration), or leave it empty for integration names.'
severity: nit
resolution: 'A run_as writer gets PrincipalType integration for integration: run_as, otherwise system; TestJobMark_WriterPrincipalType.'
status: addressed
---
