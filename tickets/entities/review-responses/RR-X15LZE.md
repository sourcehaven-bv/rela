---
id: RR-X15LZE
type: review-response
title: Pushdown declines every virtual sort key
finding: listpushdown.go declined all virtual sort keys, including id, though only some need it.
severity: minor
resolution: Only modified declines explicitly, because a type may declare a property named modified and the Go path orders by modification time. id is a reserved property name (metamodel.ReservedPropertyNames), so no type declares it and the undeclared-key check already declines it; the comment says so.
status: addressed
---
