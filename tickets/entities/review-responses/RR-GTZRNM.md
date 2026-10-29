---
id: RR-GTZRNM
type: review-response
title: Use Output.key for the denial title rather than rebuilding it
finding: Outputs already compile under key+".outputs" with an indexed key (o.key is what answerGrants/handoffs report, waiting.go:284, 366). Expose it (o.Key()) and use it in the precheckOutputs denial title (outputs.go:328) instead of t.Key()+fmt.Sprintf(".outputs[%d]"). That gives one source for the key path and lets outputKey (outputs.go:265) be deleted.
severity: nit
resolution: The denial title now uses a new `Output.Key()` and `outputKey` is deleted.
status: addressed
---
