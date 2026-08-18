---
id: RR-CUSTY
type: review-response
title: "Custom-type equivalence was a name blacklist, not a lookup"
severity: significant
status: addressed
finding: "widgetAcceptsType implemented the custom-enum rule as 'not string AND not builtin implies enum-like', with isBuiltinPropertyType hardcoding the built-in names. It never consulted meta.Types. VERIFIED: widget: select on the undeclared type 'totally-bogus' was ACCEPTED. The godoc claimed 'the metamodel's own resolveWidget makes the same equivalence' — it does not: helpers.go does a positive membership test on meta.Types, and ResolveWidgetFromType additionally requires len(Values) > 0. So a value-less custom type (declared under types: with no values:) was wrongly accepted as select-able. This is the same class of drift the ticket exists to prevent, reintroduced one function lower."
resolution: "Replaced the negation with a real lookup in widgetPropertyHasValues (meta.Types membership AND len(Values) > 0, mirroring ResolveWidgetFromType). isBuiltinPropertyType deleted — it existed only to approximate the lookup. Pinned by TestWidgetAcceptsProperty_CustomTypesByLookup, including the undeclared-name and value-less-custom-type cases."
---
