---
id: RR-PD5JFT
type: review-response
title: Section sort refused 'modified' by accident, not by rule
finding: 'Code review S3. validateSectionSort''s doc comment claimed "modified" is deliberately NOT accepted, but the function only special-cased "id". "modified" was refused solely because no entity type declares a property with that name, so declaredBy came back empty and the generic "which no type at this level declares" error fired. Right outcome, wrong mechanism: declare a property called modified and the guard evaporates, with no test to notice. The doc comment asserted an invariant nothing pinned.'
severity: significant
resolution: 'Made explicit: validateSectionSort now rejects "modified" with its own message naming why a section cannot order by it (a traversal result carries no modification time), ahead of the declared-by check. Added a validation test case asserting the refusal. The doc comment was rewritten to say the guard is explicit BECAUSE the accidental one would hold only until someone declares such a property. Note this is the opposite decision from RR-TC3ZLI, deliberately: search holds a materialized result set with ModifiedAt populated, a view section does not.'
status: addressed
---
