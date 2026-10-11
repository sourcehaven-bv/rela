---
id: RR-6ZMAPQ
type: review-response
title: A cancelled request aborts a save midway
finding: Prepare and Migrate ran on the request context, so a closed browser tab could cancel a migration after files were written.
severity: significant
resolution: Prepare, Migrate and Retry run on context.WithoutCancel.
status: addressed
---
