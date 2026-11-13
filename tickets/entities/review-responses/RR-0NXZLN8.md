---
id: RR-0NXZLN8
type: review-response
title: Engine compiled twice at construction
finding: NewEngineFromMetamodel called NewEngine, which compiled once without the metamodel before the real compile.
severity: nit
resolution: A private newEngine builds the struct; each constructor compiles once.
status: addressed
---
