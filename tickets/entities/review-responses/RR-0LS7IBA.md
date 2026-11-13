---
id: RR-0LS7IBA
type: review-response
title: RelationMetaGatedBy takes any and type-switches
finding: visibility.RelationMetaGatedBy(reader any) uses a type switch as a back-channel.
severity: minor
reason: It fails closed for any unknown reader and ScriptReader is at its plimsoll method cap so a method is not available. The only caller is the scheduler wiring.
status: wont-fix
---
