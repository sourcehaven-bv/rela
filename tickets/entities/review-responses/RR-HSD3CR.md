---
id: RR-HSD3CR
type: review-response
title: Method values bypass a call-only check
finding: Counting only call expressions misses a method value such as fn := st.GetEntity passed to a helper; that reads the zero face without a visible call.
severity: significant
resolution: The checker counts every selector reference to GetEntity/getEntity/bareEntityID except as the callee of a call with a different arity; a method value counts as one read.
status: addressed
---
