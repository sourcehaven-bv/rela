---
id: RR-E6GGM4G
type: review-response
title: 'Design: proxy bypass rule could open header spoofing'
finding: In header mode operators will add a proxy bypass for peers. A rule keyed on any Authorization header lets a caller set the principal header itself.
severity: significant
resolution: 'GUIDE-server-security gives the proxy rule: bypass only on Bearer relapeer_ and always strip or overwrite the principal header, with an nginx example. rela also ignores the principal header on a peer request.'
status: addressed
---
