---
id: RR-1752VXK
type: review-response
title: HiddenFieldFaces test did not reach the face logic
finding: The test never ran guardPage because title was visible on every row dora read.
severity: significant
resolution: dora now holds a block-less role on doc@draft and, through a group, a role with a visible block on every face. The test asserts the draft face is answered and every face is refused. grantCond's face switch is covered.
status: addressed
---
