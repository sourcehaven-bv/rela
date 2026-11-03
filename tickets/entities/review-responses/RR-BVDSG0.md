---
id: RR-BVDSG0
type: review-response
title: Auto rules log and report as if they were trigger posts
finding: "trigger.go:1222 and trigger.go:1229 (and the retry log below them) say \"trigger skipped\" and \"trigger post cannot succeed / keeps failing\" for auto rules too. system.go:104 isPermanent is documented as classifying \"an append error ... about this one post\" but now also classifies List and SetLifecycle errors. The ErrNotOpen branch is unreachable for an auto rule.\tAn operator grepping logs for a failed auto close sees \"trigger post keeps failing\" with rule=room.close.auto and looks for a trigger named room.close.auto in realtime.yaml.\tWord the messages neutrally (\"realtime rule action ...\") or branch on ru.auto. Generalize the isPermanent doc comment to \"a channel write error\"."
severity: nit
resolution: Shared failure logs now say a rule's write cannot succeed / keeps failing; the skipped log stays trigger-only (unreachable for auto rules). isPermanent doc generalized to a channel write error.
status: addressed
---
