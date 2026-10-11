---
id: RR-8CD5SB
type: review-response
title: Spec cache key ignores the generator config
finding: A Generate that starts before SetAuthHeader and finishes after it could cache a spec without a security scheme until the next schema change.
severity: minor
resolution: Generator keeps a generation counter bumped by SetAuthHeader and UpdateMetamodel; a spec built from an older generation is returned but not cached. TestSetAuthHeader_InvalidatesCachedSpec.
status: addressed
---
