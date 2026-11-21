---
id: RR-WV9V4UQ
type: review-response
title: Rows suspended by api before api_hold existed had no hold
finding: The upgrade added api_hold with default false; a tenant already suspended by api got no hold and an IdP takeover plus resume could serve it again.
severity: significant
resolution: The upgrade backfills api_hold for rows suspended by api; SuspendForIDP also sets the hold when it takes over an api suspension. Tested in TestProvisioner_APIHoldOnRowsFromBeforeTheColumn; a mutation of either path fails it.
status: addressed
---
