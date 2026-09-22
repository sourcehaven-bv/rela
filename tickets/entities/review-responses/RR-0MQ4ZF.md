---
id: RR-0MQ4ZF
type: review-response
title: 'Exec.Meta is the wrong seam for a check the data already answers'
finding: 'The plan added a Meta field to Exec so the step could read scope from the live metamodel. Exec''s existing fields (run.go:196-202) are the target of writes or capabilities needed to perform them; handing every one of the thirteen step kinds the live schema lets a future step depend on the current metamodel rather than the two projections the file embeds, which is what makes a migration file a self-contained, reproducible description of its own transformation (file.go:22-28). It also buys nothing here: the question that actually matters is whether any collected relation carries a non-zero FromFace, which is readable directly off the data with no metamodel at all.'
severity: significant
resolution: 'Exec is left unchanged. The plan''s option (b) is withdrawn: the question that matters is answerable from r.FromFace with no metamodel at all, and handing all thirteen step kinds the live schema would let a future step depend on the current metamodel rather than the two projections the file embeds - the property that makes a migration file reproducible. Adding Scope to ShapeProjection stays rejected too, since it would re-baseline every project''s shape hash for a field that does not affect conformance.'
status: addressed
---
