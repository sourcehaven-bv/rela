---
id: TKT-UG3C
type: ticket
title: Add appbuild.NewForTest fixture and migrate CLI tests off workspace.NewForTest
kind: refactor
priority: high
effort: m
status: wont-fix
---

**Folded into TKT-DS43.** The production swap to `*appbuild.Services` can't
compile while CLI tests still use `workspace.NewForTest` +
`newCLIServicesFromWorkspace` — both production and test code share the
`cliServices.svc` field type. Doing them in one PR produces a coherent single
diff. `appbuild.NewForTest`, the test fixture migration, and all four CLI test
file migrations now live in TKT-DS43.
