---
id: RR-JNZEET
type: review-response
title: 'computed: properties accept related() and fail every write'
finding: internal/computed compiles value programs whose profile does not refuse traversals; a boolean computed related() loads and then every write fails.
severity: significant
resolution: 'Plan adds a load-time refusal of related() in computed: with a test.'
status: addressed
---
