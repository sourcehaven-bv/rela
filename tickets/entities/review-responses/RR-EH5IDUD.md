---
id: RR-EH5IDUD
type: review-response
title: Evaluator should be a constructor argument
finding: StartBackground builds and starts in one call so a setter races the workers.
severity: significant
resolution: Passed as a functional option (WithCards) to New/NewWithQueue/StartBackground; nil means capture unsupported. Server passes the generation's own App.
status: addressed
---
