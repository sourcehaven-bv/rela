---
id: RR-JOINDUP
type: review-response
title: "joinWidgetTableKeys duplicated an existing generic helper"
severity: nit
status: addressed
finding: "The new helper differed from the existing generic sortedMapKeys[V any] only in that it inlined the join. Its own godoc admitted it was a copy differing by value type — but Go generics already covered that, and sortedMapKeys uses the same natsort ordering."
resolution: "Deleted joinWidgetTableKeys; the call site now uses strings.Join(sortedMapKeys(...), \", \")."
---
