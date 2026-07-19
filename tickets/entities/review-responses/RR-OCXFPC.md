---
id: RR-OCXFPC
type: review-response
title: Badge entityType prop typed object-only, blocking string callers
finding: 'stylesForProperty accepts string | EntityType, but Badge''s prop was entityType?: EntityType (object only). The widget/detail call sites hold the entity type as a string, so wiring them through would have required casts (the smell already visible as `as never` in tests).'
severity: significant
resolution: 'Badge''s prop widened to entityType?: string | EntityType with a comment explaining which call sites hold which shape; both stylesForProperty and getEnumLabel already accept the union. The test''s `as never` cast is gone.'
status: addressed
---
