---
id: RR-ERYWEJN
type: review-response
title: A cycle of done branches would spin forever
finding: done branches fire without anyone acting, so a cycle of them moves a run and writes its outputs forever.
severity: significant
resolution: 'doneCycles refuses any cycle made only of done branches. Test: done cycle.'
status: addressed
---
