---
id: RR-TG8KVC
type: review-response
title: Guard the compiled message type in messageStates
finding: ct.MessageTypes holds only the types that compiled (validate.go:180 adds a type only when messageType returns non-nil). messageStates iterates raw types and sets ct.MessageTypes[mt].In. Guard against a nil entry so a type with another problem does not cause a nil dereference during validation.
severity: nit
resolution: '`messageStates` skips a type whose compiled entry is nil, and a negative test covers a type that fails to compile and also has `in:`.'
status: addressed
---
