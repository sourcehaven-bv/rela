---
id: RR-D6VS
type: review-response
title: Toast message swallows useful error detail from thrown Error
finding: Call site uses generic `Failed to toggle checkbox` toast instead of `err.message` ('checkbox index 3 out of range (found 2)'). The thrown message is far more useful for the edge cases the comment-#4 widening would catch.
severity: nit
resolution: 'Toast on toggler throw now includes the err.message detail: `uiStore.error(\`Failed to toggle checkbox: ${detail}\`)`. Users now see ''checkbox index 3 out of range (found 2)'' rather than the generic ''Failed to toggle checkbox''. The network-error path keeps the generic message because the underlying error is usually a status code that''s not useful as a toast.'
status: addressed
---
