---
id: RR-TUR3KX
type: review-response
title: Auto rule log name omits the entity type
finding: "trigger_rules.go:108 names every auto rule `<channel type>.<op>.auto`. A channel type with several attach_to types yields several rules with the same name and different ids and entity types.\troom attaches to [ticket, bug]. A failing rule logs rule=room.close.auto, and the reader cannot tell whether the ticket rule or the bug rule failed.\tInclude the entity type, e.g. `room.close.auto[ticket]`."
severity: nit
resolution: Auto rule log name is now <channel type>.<op>.auto[<entity type>].
status: addressed
---
