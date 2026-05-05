---
id: RR-QU6BK
type: review-response
title: Empty-string-as-no-op is a UX trap for update_entity callers
finding: 'Documented, but unintuitive: a user who wants to clear a string property will reasonably try "" and get nothing. Consider erroring on "" for update_entity specifically (''ambiguous: use null to delete or a non-empty value to set''). Behavior change vs. current silent-no-op, but better UX. Or accept the silent-no-op and rely on documentation — plan picked this; reaffirm.'
severity: minor
reason: 'Deliberate decision affirmed: empty-string-as-no-op preserves create-path symmetry (form callers depend on `""` not overwriting). Erroring on `""` for update_entity specifically would create a confusing asymmetry between create_entity and update_entity. The tool description now carries an explicit warning (''Empty string is treated as no value (silently ignored — use null to delete)'') so the AI client sees the rule before it tries `""`. If clients still confuse the two in practice, revisit then.'
status: wont-fix
---
