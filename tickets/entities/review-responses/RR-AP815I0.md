---
id: RR-AP815I0
type: review-response
title: New tests mutate compiled machines in place
finding: TestDecideMove_SeveralBranchesRefused appended to a compiled machine's branches.
severity: nit
resolution: 'Addressed: the test builds a new Machine with cloned exits. t.Parallel is not added because no test in the package uses it.'
status: addressed
---
