---
id: RR-TG3I6W
type: review-response
title: A tab lost its default_sort when the order was withheld
finding: The client dropped default_sort on a predicted relation-ordered tab, but the server withholds relation_order from a reader who cannot see _order_out, leaving store order.
severity: minor
resolution: EntityList notes a read that asked for the relation order and did not get it, and rereads with default_sort. EntityList.relationOrder.test.ts.
status: addressed
---
