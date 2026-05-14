---
id: RR-RF8J
type: review-response
title: 'Architect #7 + cranky #4: test-only Workspace methods in _test.go'
finding: createEntity / updateEntity / deleteEntity etc. are defined as methods on *Workspace inside workspace_test.go. Unusual pattern; hides from grep on workspace.go.
severity: significant
reason: 'Intentional transitional cost: 44 test call sites kept working without rewrite. TKT-64R3 will classify each test (delete / relocate / rewrite) when workspace itself goes. Reviewers agreed this is acceptable as a transitional tool; migrating now would expand TKT-IU2S scope substantially with work thrown away at TKT-64R3.'
status: wont-fix
---
