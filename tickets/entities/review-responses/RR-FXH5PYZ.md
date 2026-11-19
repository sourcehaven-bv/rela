---
id: RR-FXH5PYZ
type: review-response
title: successHold can answer for a tenant removed within the hold
finding: A webhook re-sent within 5 s of a success, for a tenant whose control row was removed, gets the old run and a 200 without a create.
severity: minor
resolution: During the hold the webhook now answers 503 with Retry-After instead of 200, so the IdP retries after the hold and the retry creates the tenant if its control row was removed. Lazy requests answer 202 and recreate after the hold. Pinned by TestProvision_NoRerunAfterSuccess and TestProvision_SuccessHoldEnds.
status: addressed
---
