---
finding: V1 has validation.go but v2 has none. Root node must have entry_type but this is not validated. This is a critical gap - invalid view definitions will cause runtime errors rather than clear error messages.
id: RR-f6nc
resolution: Added validation_v2.go with Validate methods for FileV2, ViewDefV2, and QueryNode. Root nodes must have entry_type and param, child nodes must have via or via_incoming, and all entity/relation types are validated against the metamodel.
severity: critical
status: addressed
title: No validation for v2 types
type: review-response
---
