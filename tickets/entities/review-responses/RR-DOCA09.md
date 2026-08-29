---
id: RR-DOCA09
type: review-response
title: 'apiBuildTimeout widened the deadline for every build on the fs backend'
finding: 'NewAPIClient never fails on the default build, so opts.APIClient was always non-nil and every build got the 2-minute ceiling instead of 30s — including Tier-A manuals with no server to stand up. No resource cost (standing up is lazy), but a hung build took four times as long to say so.'
severity: minor
resolution: 'The deadline is gated on the manual actually containing an api{} island.'
status: addressed
---
