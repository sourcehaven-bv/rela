---
id: RR-3OY8XW
type: review-response
title: No referrer policy on diagram <img> leaks page URL to render server
finding: 'The diagram <img> sets no referrerPolicy, so the default strict-origin-when-cross-origin sends the rela origin to the cross-origin render server on every fetch. A stateless render request has no need for the referrer. Fix: img.referrerPolicy = ''no-referrer''.'
severity: significant
resolution: 'img.referrerPolicy = ''no-referrer'' on the diagram image. Test: ''sets no-referrer on the diagram image''.'
status: addressed
---
