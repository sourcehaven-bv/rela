---
id: RR-G2LOOD
type: review-response
title: metamodel.md example comment contradicts content-states.md on whether the adopted face needs a note
finding: 'The new YAML example in docs/metamodel.md comments the adopted face `# nothing declared: the adopted version needs no explanation`, while docs/content-states.md''s new example declares a read_only on exactly that face, and metamodel.md''s own read_only table row describes that case as read_only''s canonical use. Each is defensible alone; read together they contradict. Better to state the mechanism (''this face renders no note'') than an editorial judgement about what operators need.'
severity: nit
resolution: 'Changed the metamodel.md example comment from ''# nothing declared: the adopted version needs no explanation'' to ''# nothing declared: this face renders no note'' — a statement about the mechanism rather than an editorial judgement, so it no longer contradicts content-states.md''s example declaring a read_only on that same face.'
status: addressed
---
