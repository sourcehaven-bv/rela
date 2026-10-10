---
id: RR-ZA4KZJ
type: review-response
title: CalDAV CardinalityError mapped to 409 causes infinite retry
finding: A CardinalityError on a CalDAV write returned 409. Clients treat 409 as retryable and retried forever.
severity: minor
resolution: Fixed in 2869cf91e. A CardinalityError goes through refusedWriteResponse, which returns 403 on the create flow.
status: addressed
---

Review finding R2-4.
