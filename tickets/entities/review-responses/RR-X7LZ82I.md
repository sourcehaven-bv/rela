---
id: RR-X7LZ82I
type: review-response
title: link_text did not reach the browser
finding: v1.PropertyDisplay had no LinkText field and toV1PropertyDef copied only currency and link, so the SPA never received link_text.
severity: critical
resolution: PropertyDisplay gains LinkText and toV1PropertyDef copies it; TestToV1PropertyDef_Display and the e2e url-link test cover it.
status: addressed
---
