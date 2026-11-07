---
id: RR-I6TTK56
type: review-response
title: Address format fixes a sort order
finding: The address strings decide range order. Bytewise order differs from tuple order and from the default pg text collation; later SQL ranges could disagree with Go and never converge.
severity: significant
resolution: The package doc states that elements are ordered bytewise by Address within one Kind; a range never spans kinds; SQL must compare addresses with a byte collation (pg COLLATE C and sqlite BINARY). A test pins the order for the prefix-type and face cases.
status: addressed
---
