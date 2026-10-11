---
id: RR-9YI7O3
type: review-response
title: pg and sqlite versiontags.go are near-duplicates
finding: TagCurrent, currentVersion, checkExpect, TagVersion and Untag are nearly line-for-line copies.
severity: minor
reason: Behavior-neutral refactor; kept out of this diff to keep review focused. Both copies are held to one contract by storetest.RunVersionTagTests.
status: deferred
---
