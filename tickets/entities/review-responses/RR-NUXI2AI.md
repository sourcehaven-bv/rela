---
id: RR-NUXI2AI
type: review-response
title: kanban_id on the position scope type
finding: kanban_id sat on ListQueryScope, which the position scope extends, but the server scope does not forward it.
severity: minor
resolution: Moved kanban_id to ListParams only.
status: addressed
---
