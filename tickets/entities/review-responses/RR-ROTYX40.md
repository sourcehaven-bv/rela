---
id: RR-ROTYX40
type: review-response
title: Guard-denied inference relies on the single-branch invariant
finding: If several branches per move were allowed a guard refusal on one could hide a false precondition on another.
severity: nit
resolution: 'Addressed: Compile refuses a duplicate (from, to) (TestCompile_Errors), TestCompile_OneBranchPerMove pins one branch per compiled move, and decideMove refuses more than one branch (TestDecideMove_SeveralBranchesRefused).'
status: addressed
---
