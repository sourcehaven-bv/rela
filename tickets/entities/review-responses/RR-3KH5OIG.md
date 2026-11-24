---
id: RR-3KH5OIG
type: review-response
title: 'Design: path choice before the Tx is a TOCTOU'
finding: 'Choosing the clearing path before the Tx from a pre-read lets a concurrent write make a clear necessary after the decision.'
severity: significant
resolution: 'The choice is static: any write touching a scoped, scope or scope-owning relation type runs the scope check inside its Tx and applies the clears there.'
status: addressed
---
