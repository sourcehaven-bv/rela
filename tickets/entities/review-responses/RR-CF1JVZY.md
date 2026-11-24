---
id: RR-CF1JVZY
type: review-response
title: 'Code: conflict comment promised a retry that does not exist'
finding: 'decide said a retry sees an unseen clear, but no retry loop wraps the write.'
severity: minor
resolution: 'The comment now says the conflict needs a concurrent write between the pre-pass and the transaction, and that the caller may retry. Without a concurrent write both reads see the same state.'
status: addressed
---
