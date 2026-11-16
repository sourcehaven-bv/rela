---
id: RR-EV38III
type: review-response
title: 'Nits: wiring asymmetry, duplicated comments, nil contract, compileListFilter signature'
finding: Duplicated inline comments at the two wiring sites, the RelatedReads.Conditions nil contract wording, the allowlist reason copying the fallback list, and compileListFilter taking all of Deps.
severity: nit
resolution: Removed the wiring comments, fixed the nil contract, the allowlist reason points at the filterInGo doc, and compileListFilter takes the metamodel and RelatedReads.
status: addressed
---
