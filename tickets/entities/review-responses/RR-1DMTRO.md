---
id: RR-1DMTRO
type: review-response
title: Concurrent identical creates on a bounded relation fail with replace_failed
finding: Two identical concurrent creates on a bounded relation ended in replace_failed instead of being retried as an update.
severity: minor
resolution: Fixed in 2869cf91e. The handler re-plans once on ErrRelationAlreadyExists.
status: addressed
---

Review finding R2-5.
