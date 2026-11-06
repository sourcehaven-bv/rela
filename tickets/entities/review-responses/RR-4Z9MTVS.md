---
id: RR-4Z9MTVS
type: review-response
title: Committed copy reported as failed when cascade errors
finding: A cascade error after commit made the HTTP request fail although the copy was durable.
severity: significant
resolution: CopyResult.FollowUpErr carries it; the handler logs it and returns 200 with a follow_up_failed warning.
status: addressed
---
