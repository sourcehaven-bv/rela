---
id: RR-DQA0YQ6
type: review-response
title: Store faults inside the Tx answer 422 with driver text
finding: An unclassified error carrying a BatchOpError fell through to validation_failed with err.Error().
severity: significant
resolution: CreateErrUnknown now answers writeNoIdentity or a bare 500 batch_failed, whatever the operation.
status: addressed
---
