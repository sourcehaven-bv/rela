---
id: RR-C0UN1K
type: review-response
title: Context.Exists is dead code and its new doc describes a caller that does not call it
finding: Context.Exists has zero non-test call sites. The doc comment added in this change says 'Checking only the new name would let a caller conclude a legacy project is uninitialized and overwrite it' — true in the abstract, but the caller it describes (InitializeWithFS) calls project.SchemaFileAt directly, not Exists. So the method is dead and TestExistsAcceptsLegacyName tests dead code while claiming to be 'the guard behind the init refusal'. It isn't; that guard is TestInitializeRefusesExistingSchema. Either delete Exists and its test, or route InitializeWithFS through it so the comment becomes true.
severity: significant
resolution: 'Kept Exists but rewrote the doc so it describes what the method actually guarantees rather than naming a caller that does not call it: it now states the rule for callers about to CREATE a project, and points at SchemaFileAt for those needing to know WHICH name was found (which is why InitializeWithFS uses that instead — it needs the name to choose between the two error messages). The method is a reasonable public-API convenience on Context, so deleting it in a rename ticket would be unrelated churn; the misleading comment was the actual defect and it is gone.'
status: addressed
---
