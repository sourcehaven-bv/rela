---
id: RR-7IIXBS5
type: review-response
title: Proxy bypass guidance does not limit paths
finding: The guide told operators to skip the login for Bearer relapeer_ on any path, but rela checks peer keys on /api/ only. Security review.
severity: minor
resolution: The guide now limits the bypass to /api/v1/ and explains why.
status: addressed
---
