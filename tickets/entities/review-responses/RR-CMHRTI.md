---
id: RR-CMHRTI
type: review-response
title: Merge keys silently accepted in every realtime.yaml map
finding: 'internal/realtimecfg/keys.go:99 now walks a `<<` value as entries of the same map for every map in the file (channel_types, roles, message_types, properties, triggers). Before this change a merge key was refused, because its value was checked as one entry and failed. The plan said "the key checker needs no change". The change widens the config language beyond sessions, with no test and no docs. It also interacts with mappingKeys (internal/realtimecfg/session.go, called at validate.go:194): that walk ignores merge keys on the path. A channel type that comes in through `channel_types: {<<: *base}` is not found, and session.go:138 silently falls back to sorted state order. The declaration order the plan promises for slice 2 is then lost without an error. A sequence merge (`<<: [*a, *b]`) also gets an odd "must be a mapping". Fix: revert the global change. Skip a merge key only where mappingKeys reports it (session.states), or refuse `<<` in the walker with one clear reason. If merge keys should stay allowed, add tests for non-session maps, make mappingKeys follow merges, and report "could not read state order" instead of falling back to sorting.'
severity: significant
resolution: realtimecfg/keys.go refuses any mapping with << anywhere in realtime.yaml with one clear reason; tested for a map, a struct, the top level and session.states.
status: addressed
---
