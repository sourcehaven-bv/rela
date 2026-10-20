---
id: RR-JVI6H8
type: review-response
title: MessagePropInfo carries both Values and Enum
finding: §1 says the descriptor "carries the enum type name, not a copy", but the struct has `Values []string` and adds `Enum string`. Pick one. Values is simpler, the library cannot read the schema store anyway, and config is not secret. Send `values` plus optional labels and colours resolved server-side, or only `enum` and resolve app-side.
severity: nit
resolution: The descriptor sends values for structure and the enum type name for presentation; the app resolves labels and colours from the schema store and passes them as enumOptions (R28).
status: addressed
---
