---
id: RR-R6SLY9
type: review-response
title: Example connector can create duplicate todos
finding: create_remote links the ref only after the completion POST; a raise in between makes the bounded retry POST again.
severity: significant
resolution: create_remote links the ref right after the create POST; stub 503 on completion once; subtest asserts one create POST and one todo.
status: addressed
---
