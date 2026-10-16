---
id: RR-JCT7N0
type: review-response
title: Deferred work survived unmount
finding: runWhenVisible keys queued by nav composables stayed queued after unmount and ran against disposed queries.
severity: minor
resolution: Added cancelWhenVisible; useNavItems, useNavEntities and useNavStatus call it on unmount. A vitest case pins it.
status: addressed
---
