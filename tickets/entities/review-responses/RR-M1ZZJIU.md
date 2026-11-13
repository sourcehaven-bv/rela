---
id: RR-M1ZZJIU
type: review-response
title: flows.ts problem JSDoc still promises an automatic retry
finding: 'frontend/src/api/flows.ts: the JSDoc of Completed.problem said a failed output step after the move is retried, the TypeScript twin of the flowview comment.'
severity: significant
resolution: The JSDoc now says the step waits for a manual retry or a cancel.
status: addressed
---
