---
id: RR-G4VKS89
type: review-response
title: rela flow show needs the related() binder
finding: 'CompileTransitions refuses a related() when: without a binder, and the compile with the raw-store binder is already repeated at three sites.'
severity: significant
resolution: One exported appbuild helper (CompileHeadlessFlows) compiles with the raw-store binder. The boot path, schemacheck, CompileTransitions and the CLI all call it.
status: addressed
---
