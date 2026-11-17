---
id: RR-823CLRS
type: review-response
title: 'Trigger ACL check: a literal relate.to target is not judged'
finding: A fixed relate.to id was only checked for the relation type and update on the subject, so an editor could link records to an id they cannot read and probe for it through cascade errors.
severity: significant
resolution: 'A relate output with a fixed target is refused by the checker ("links to the fixed record ..."), and relate.** is locked in the flows.yaml allowlist; create.relate, which links the subject to the record it creates, stays allowed. Test: "a fixed relation target"\; TestCheckEdit_Flows "add a relate output".'
status: addressed
---
