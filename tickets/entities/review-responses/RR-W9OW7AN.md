---
id: RR-W9OW7AN
type: review-response
title: 'Design: merge write can overwrite a local edit'
finding: The plan read ours, merged and wrote without a precondition, so a user edit in between was lost.
severity: significant
resolution: 'Plan: PatchEntity with ExpectedVersion from the read; on conflict re-read and merge once, then report.'
status: addressed
---
