---
id: RR-FGR60O
type: review-response
title: Breadth.Calls and Breadth.Totals were unused speculative API
finding: On a brand-new file, Calls() and Totals() were called only by Breadth.String() itself; no test used either. The IDs doc comment even advertised 'Pair it with [Breadth.Calls] when the split matters' - advertising a pairing nobody does. String() additionally took the lock for Totals(), released it, then re-took it per key via Calls(k), so a concurrent record could land between and print an id total disagreeing with its call count.
severity: significant
resolution: Both accessors deleted. String() now reads both maps under ONE lock acquisition, which removes the straddle as well as the dead API, and its doc explains why the single acquisition matters. The IDs comment no longer advertises a deleted method.
status: addressed
---
