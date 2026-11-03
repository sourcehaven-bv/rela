---
id: RR-Q4FRPD
type: review-response
title: Text fields reach the body without markdown escaping
finding: item.title and other text refs are flattened but not escaped, so a feed title like ![](tracker) renders as an image when placed in create_if_missing.content.
severity: minor
reason: Shared with webhooks, which behave the same; impact is a tracking image only (DOMPurify blocks script). Escaping belongs in ingest for both sources; documented in the Trust section for now.
status: deferred
---
