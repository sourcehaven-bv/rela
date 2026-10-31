---
id: RR-6OGAMA
type: review-response
title: Kind policy on a broken compiled property yields a garbled second problem
finding: 'Compiled Property.Type is empty when the raw type is unknown (validator.property, validate.go:520-529). A raw `list: true` on a non-string type compiles with List false and keeps the raw type. Old literalProps read the raw type and printed it. The new kind rule reads the compiled property, so it emits "... <prop> is a  property" (empty TypeName) for an unknown type, and the integer kind reason instead of the list reason for an integer marked `list: true`. The file is already refused at `...properties.<p>.type` or `.list`, so the extra problem is only noise, but its text is wrong. Skip the literal check for a property whose compiled Type is empty, or skip literalPost when the message type had property problems. Add one test case.'
severity: minor
resolution: "addressed\tliteralPost returns without a problem when a literal names a property whose compiled Type is empty; new test case pins it; the integer-with-list:true case keeps the integer reason, which is true of the compiled property."
status: addressed
---
