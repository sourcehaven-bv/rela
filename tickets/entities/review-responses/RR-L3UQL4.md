---
id: RR-L3UQL4
type: review-response
title: No explicit coverage floor for a security-critical package (default 50 vs measured 93.4)
finding: '.testcoverage.yml pins high floors (85–95) on load-bearing packages; a read-side ACL seam qualifies. With only the default 50 floor, the security control''s tests could erode silently by 40+ points before CI trips. Add ^internal/visibility$: 85.'
severity: minor
resolution: 'Added .testcoverage.yml override ^internal/visibility$: 85 with a security-critical rationale comment. Measured 94.2% after review fixes.'
status: addressed
---
