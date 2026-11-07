---
id: RR-VW6OBC5
type: review-response
title: 'Design: nil Fields means the opposite of search'
finding: search.TypeScope.Fields nil means nothing hidden; here nil means all pending, so a naive converter silently degrades.
severity: minor
resolution: 'Plan changed: the field is named Visibility with a Nil: contract (nil = unknown = pending, fail closed); step 5 adds a test asserting Pending == 0 for a type without grants.'
status: addressed
---
