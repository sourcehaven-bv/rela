---
id: RR-D2CP
type: review-response
title: is_one_of and contains cannot be declared with monomorphic FuncSig
finding: predicate.FuncSig.Params is []Type with no union/generic. is_one_of(value, list) requires polymorphism the type system lacks. The escape hatch (AnyType in Params + runtime type-switch) defeats the compile-time type checker for policy authors — typos like `is_one_of(entity, 'x')` (passing Record where scalar meant) become runtime errors instead of startup errors.
severity: critical
resolution: Dropped is_one_of and contains from v1 scope. Shipped typed primitive string_in_list(value string, allowed list_of_string) bool as the only collection helper. Numeric membership uses verbose `x == 1 or x == 2 or x == 3` (type-checked). Polymorphic variants land in follow-up tickets if predicate authors demonstrate the need.
status: addressed
---
