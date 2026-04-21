---
id: RR-5X07F
type: review-response
title: New() doesn't validate EntitiesKey/RelationsKey are non-empty
finding: 'Empty EntitiesKey/RelationsKey silently changed behavior from old code (old: skip scan on ErrNotExist; new: resolve error).'
severity: minor
resolution: Added explicit 'EntitiesKey must not be empty' + 'RelationsKey must not be empty' checks in New(). AttachmentsKey and CacheKey remain optional (empty disables those features, as before).
status: addressed
---
