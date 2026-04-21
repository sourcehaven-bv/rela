---
id: RR-9RLCK
type: review-response
title: memoize+nested-set overwrites silently (round 2)
finding: If fn inside rela.cache.memoize('k', fn) calls rela.cache.set('k', ...), memoize's own post-fn set overwrites that write. Script author sees their side-channel write lost with no diagnostic. Detecting and skipping would require entry versioning; documenting the semantic is cheaper.
severity: significant
resolution: Added FOOTGUN section to luaCacheMemoize doc comment documenting last-write-wins semantic and recommending different keys for side-channel writes (e.g. 'key:aux'). Not worth versioning entries for this pattern.
status: addressed
---
