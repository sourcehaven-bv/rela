---
id: RR-O4CN93R
type: review-response
title: 'Design: absent and empty flows.yaml look the same'
finding: versionOf cannot tell an absent file from an empty one and writer.write returns a plain error instead of ErrConflict when the file appeared.
severity: significant
resolution: 'An absent flows.yaml is kept apart from an empty one; a save creates it and the audit says absent. Prepare failure restores both cases. Tests: TestSave_CreatesFlowsFile and TestSave_PrepareFailureRestoresFlows.'
status: addressed
---
