---
id: RR-SNN2K8Z
type: review-response
title: Remote audit is fail-open and untested through the wiring
finding: 'Cranky 7: recordRemote records nothing when the workspace has no audit sink so a wiring change could silently drop the remote-resolve/dismiss/link audit.'
severity: minor
resolution: dataentrywire now takes an auditedWorkspace (WorkspaceProvider plus Audit) so a wiring without an audit sink does not compile; the scheduler test checks the three ops and their summaries.
status: addressed
---
