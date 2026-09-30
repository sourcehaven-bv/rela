---
id: RR-7GQQJF
type: review-response
title: Face order wired at one resolver site and untested
finding: Only the dataentry app resolver passed WithFaceOrder; other resolvers sorted by token and no HTTP test pinned declaration order (cranky review).
severity: significant
resolution: newVisibleReader now requires a face order and rejects nil. The dataentry view reader, export and docs resolvers pass it. TestFaceOrder_WorldAbsentPageUsesDeclarationOrder pins it over HTTP. The script, allow-all, unrestricted and cli resolvers stay on token order because their Family consumers only test containment or length; the Family doc says position-pickers need the option.
status: addressed
---
