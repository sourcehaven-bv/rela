---
id: RR-J5ZJYT
type: review-response
title: Property default off the state machine entry fails every create
finding: A status property default that differs from its state machine's entry value makes every create without a status fail EnforceCreate. Nothing rejected that config.
severity: significant
resolution: statemachine.Compile now rejects a property default that is not the machine's entry value. Test TestCompile_RejectsPropertyDefaultOffEntry; doc comment corrected.
status: addressed
---
