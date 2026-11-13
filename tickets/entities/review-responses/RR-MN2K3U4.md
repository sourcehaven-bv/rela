---
id: RR-MN2K3U4
type: review-response
title: Batch run skips rows addressed at a non-served face
finding: batchRows keeps only the face the default world serves, so a list under ?world= that sends ID@face addresses would see every row skipped; the single-row path accepts any readable face.
severity: significant
reason: 'Not reachable: validateLLMAction refuses llm actions on types with faces, so no faced address can name a real row. Commented in batchRows; revisit when llm actions support faces.'
status: deferred
---
