---
id: RR-6C8L7R
type: review-response
title: Composable tests miss id switch write and storage failure
finding: No test for setCollapsed after a board switch, nor for a throwing setItem.
severity: minor
resolution: Added 'writes under the current board after a switch' and 'still folds the column when storage refuses the write'.
status: addressed
---
