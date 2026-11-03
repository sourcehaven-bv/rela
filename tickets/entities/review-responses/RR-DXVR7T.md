---
id: RR-DXVR7T
type: review-response
title: Behaviour of hidden messages inside a mixed run is unspecified
finding: 'AC 4 says a mixed run folds its collapsed messages only. The plan does not say whether a hidden message between two collapsed ones continues the run or splits it, or whether expanding the fold also shows the hidden ones. Specify both in AC 4 and add table cases. The recommendation: a hidden message continues the run, and expanding shows only the collapsed messages.'
severity: minor
resolution: A hidden message now continues a run, and expanding shows only the collapsed messages (AC 4, Terms). Added table cases and an edge case.
status: addressed
---
