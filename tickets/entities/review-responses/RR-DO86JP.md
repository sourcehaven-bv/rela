---
id: RR-DO86JP
type: review-response
title: Attachment upload/download broke on a faced address once the SPA sent one
finding: 'SectionEditForm now receives the served address, so the file widget issued PUT/GET on /policys/POL-1@published/_attachments/..., which the attachment handlers looked up as a bare id: a uniform 404 on pgstore, and on fsstore a phantom attachment key nobody can list or download. computeAttachments also built hrefs from the faced `_self`.'
severity: critical
resolution: Attachments are per entity, so the dispatcher parses the address and hands the attachment handlers the bare id (bareEntityID), and computeAttachments builds hrefs from the bare `_self` (bareSelfHref). The existing face gate on downloads is unchanged.
status: addressed
---
