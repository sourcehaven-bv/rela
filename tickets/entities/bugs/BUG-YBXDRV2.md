---
id: BUG-YBXDRV2
type: bug
title: relation_created and relation_removed triggers never fire
description: The events are documented and pass validation, but entitymanager CreateRelation and DeleteRelation ran no automation.
priority: high
effort: m
why1: Manager.CreateRelation and DeleteRelation wrote the edge without calling the trigger engine.
why2: The trigger engine was wired only into the entity write paths; the relation events were parsed and validated but had no caller.
why3: Validation of flows.yaml checked the event names against a list, not against what the manager actually dispatches.
why4: No test wrote a relation and asserted that a relation trigger fired.
why5: Documented events had no end-to-end test tying the docs to the write path.
prevention: relation_triggers_test.go and relation_triggers_acl_test.go fire each event through the real manager; the guide lists which paths fire and which do not.
started: "2026-11-27"
completed: "2026-11-27"
status: done
---
