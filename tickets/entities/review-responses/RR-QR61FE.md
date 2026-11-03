---
id: RR-QR61FE
type: review-response
title: Cap is an id-order window before sort
finding: 'MentionedBy serves sources in id order, so a 200 cap keeps the lowest ids and sort: orders only that window; the Sort contract says sort precedes caps.'
severity: significant
resolution: The cap is an id-order window of 200 readable matching sources. The section carries incomplete when more exist; the guide states that sort orders the collected set and does not pick it.
status: addressed
---
