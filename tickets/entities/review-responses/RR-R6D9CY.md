---
id: RR-R6D9CY
type: review-response
title: A trigger-only (entity-driven) transition can be forged by any poster
finding: 'Decision d1 moves sessions on entity changes by having a trigger post a typed message into a transition without roles. realtimecfg requires a non-empty post: on every message type (validate.go: "name the roles that may post this message type"), and a transition without roles fires for every principal who may post that type. So any member with the post right can post the "entity changed" message and move the session without the entity changing. That is an integrity hole in exactly the composition the plan relies on, and the Security section does not mention it. Either allow a trigger-only marker (e.g. empty post: for message types only triggers write, or a transition `by: trigger`), or document the workaround (a post role no principal holds) and add a load warning when a transition without roles is keyed on a type that users may post and a trigger also posts. Add a test for the chosen rule.'
severity: significant
resolution: 'Plan updated: a new transition key by: trigger makes a transition fire only for trigger posts (not combinable with roles); with neither key any allowed poster fires it, documented and tested.'
status: addressed
---
