---
id: RR-I15PZXM
type: review-response
title: Opened embeds *appbuild.Services
finding: Embedding promotes Services.Close, which closes without releasing the lock; tenantResident kept two handles; Close panicked on nil pin and was not idempotent.
severity: minor
resolution: Opened now has a named Services field, a nil-safe Close guarded by sync.Once; tenantResident keeps only opened.
status: addressed
---
