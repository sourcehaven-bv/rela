---
id: RR-AJOVY9
type: review-response
title: Matrix dropped rows with undeclared enum values
finding: A row whose axis value was outside the enum counted towards Total but appeared in no cell and not in unplaced.
severity: significant
resolution: 'foldMatrix treats a value an enum axis does not declare as unplaced. Test: TestFoldMatrix_UndeclaredEnumValueIsUnplaced. Docs updated.'
status: addressed
---
