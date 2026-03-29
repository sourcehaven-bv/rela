---
finding: Child nodes should NOT have entry_type/param (root-only fields), and non-root nodes must have via or via_incoming. These constraints are documented but not enforced.
id: RR-zjlv
resolution: 'Added validateAsChild method that validates: child nodes cannot have entry_type or param (root-only fields), must have exactly one of via or via_incoming, and all references are validated against metamodel.'
severity: significant
status: addressed
title: Child nodes validation missing
type: review-response
---
