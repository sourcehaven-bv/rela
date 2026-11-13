---
id: RR-YI23YFP
type: review-response
title: Golden test misses the risky matcher paths
finding: The recorded schemas have no condition, faces, copy, validate, add_to_pile or notify, no warnings, and no alias, unknown-type or untranspilable clause, so drift there would pass.
severity: significant
resolution: 'Added TestSyntheticAutomations_Golden: a synthetic schema and event matrix (alias, unknown and case-variant types, undeclared and untranspilable clauses, conditions that hold, are false or fail, faces, copies, validate, piles, notify, create, scripts, managed names) over four engines (binder true, false, error, and string-only). It is recorded against the old engine in a commit before the refactor.'
status: addressed
---
