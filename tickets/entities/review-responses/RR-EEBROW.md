---
id: RR-EEBROW
type: review-response
title: Webhook copies Mapping fields instead of embedding
finding: Webhook copies Mapping fields instead of embedding
severity: minor
resolution: Embedding breaks the composite literals in existing tests (promoted fields). TestWebhookMapping_CarriesEveryMappingField fails if Webhook lacks a Mapping field or Mapping() does not copy it.
status: addressed
---
