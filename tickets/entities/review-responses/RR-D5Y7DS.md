---
id: RR-D5Y7DS
type: review-response
title: storetest.Counting always claims the mention capability
finding: Counting.MentionSources exists whatever store it wraps, so the resolver's type assertion succeeds even when the wrapped store has no index.
severity: minor
reason: Counting is a test-only wrapper. When the wrapped store lacks the index it returns a loud error, so a test fails rather than reading an empty answer. Every store backend implements the capability.
status: wont-fix
---
