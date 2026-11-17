---
id: RR-O1U5C8Z
type: review-response
title: 'Design: three-file call sites in Save and Retry'
finding: Save's no-op check and result version and Retry's Validate and the audit summary named only schema.yaml and data-entry.yaml; a flows-only save would report saved and write nothing.
severity: significant
resolution: 'Save and Retry and the no-op check and result version and audit summary now cover all three files. Test: TestSave_FlowsOnly.'
status: addressed
---
