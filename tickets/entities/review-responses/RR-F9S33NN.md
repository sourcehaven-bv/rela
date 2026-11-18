---
id: RR-F9S33NN
type: review-response
title: Chain bound only covers the synchronous settle
finding: The hop count lived on ctx, so a run the flow worker settled later counted from zero again and a chain could run without end.
severity: significant
resolution: 'Each run started on create stores its depth on the header (FlowRunPropDepth), one more than the run named by the create''s audit.FlowStep.Run. A missing producing run counts as the bound. Test: TestFlowCreateStart_ChainIsBounded, including a create under the deepest run.'
status: addressed
---
