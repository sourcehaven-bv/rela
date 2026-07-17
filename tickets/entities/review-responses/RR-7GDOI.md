---
id: RR-7GDOI
type: review-response
title: Paren nesting cap is ~31 not the documented 64 (double enter() per group); inconsistent with not-nesting
finding: 'Each ( expr ) consumes two depth units: parsePrimary enter() then parseOr enter() inside it, neither leaving until the group closes. Verified: parens rejected at depth 32, accepted at 31, while bare not-nesting gets the full 64. Not a security hole (both reject safely) but the MAX_DEPTH=64 comment overstates the real limit 2x for the common form and the two nesting kinds are inconsistent. Count a paren group as one unit (don''t double-enter), or document the real effective limits.'
severity: minor
resolution: Parenthesized groups no longer consume the complexity budget twice — with the switch to a total-node budget, a paren group adds no node of its own (it returns the inner expression), so nesting is bounded uniformly by MAX_NODES with no double-count. The stale MAX_DEPTH=64 comment is gone (replaced by the MAX_NODES doc). Test confirms 100-deep parens around a single value stays within budget.
status: addressed
---
