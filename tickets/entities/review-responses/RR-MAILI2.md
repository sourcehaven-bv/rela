---
id: RR-MAILI2
type: review-response
title: A separate occurrence claim store adds a pool and DDL without exactly-once delivery
finding: The proposed claim layer opened another PostgreSQL pool and created a rela_job_claims table from the queue constructor. It bypassed normal migration and pool-injection conventions while SMTP could still duplicate after remote acceptance and before local completion.
severity: critical
resolution: Remove the claim store entirely. Reuse the queue's stable pending idempotency keys and document the post-completion expansion retry window as at-least-once instead of purchasing infrastructure for a guarantee the external effect cannot provide.
status: addressed
---
