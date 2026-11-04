---
id: RR-XKBJKNJ
type: review-response
title: Arrow keys go to the wrong option when nothing is checked
finding: 'RlSegmentedControl.vue move(): with no option checked current was -1, so Left went to index n-2.'
severity: minor
resolution: move() starts from the tab stop, which is the focused option when nothing is checked. Story NoneCheckedArrows (three options, Left from the first goes to the last).
status: addressed
---
