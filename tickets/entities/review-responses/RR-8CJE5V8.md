---
id: RR-8CJE5V8
type: review-response
title: Column Add on a state-machine column_property puts the card in the entry state
finding: On a board whose column_property is a state-machine field the create dry-run locks the field to the entry value so Add in a later column creates the card in the initial column.
severity: significant
reason: The create form shows the locked Status before save so the card lands where the form says. A proper fix needs the server to expose which fields are state machines to the SPA. Tracked as a follow-up. Atlas boards have no state machine.
status: deferred
---
