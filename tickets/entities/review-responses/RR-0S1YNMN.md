---
id: RR-0S1YNMN
type: review-response
title: Multi-key do items and the skip-on-failed-create rule
finding: compileOutputs renders a do item's keys in a fixed order (set, relate, pile, notify, create, lua, lua_file) and a failed create skips the rest of that item.
severity: significant
resolution: 'Plan: the migration splits multi-key items in that fixed order; an item combining create_entity with lua or lua_file is refused with a message to split it (the only case where the skip rule matters). A golden covers a multi-key item.'
status: addressed
---
