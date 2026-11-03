---
id: RR-G9YD0I
type: review-response
title: Budget and acceptance tests are weaker than the plan's AC6, AC7 and section 13
finding: 'internal/appbuild/realtime_triggers_internal_test.go:227-258 checks one changed id and counts Rows only. AC6 asks for 10 vs 50 changed ids with identical Rows counts and at most SeenShards KV reads per rule. Section 13 asks for a bytes-written-per-round budget. AC7 (no Lua/automation engine call) has no test. There is no test for a cooldown across two engines, a takeover with a dirty shard, ctx cancellation mid-round, or corrupt-shard recovery. Fix: add a counting KV (Gets/Puts/bytes) to the 10-vs-50 budget test and add the missing cases.'
severity: minor
resolution: Added the 10 vs 50 changed id budget test with a counting KV (gets, puts, bytes written) and identical Rows counts, plus tests for takeover with unsaved shards, cancel mid-round, corrupt index and shard, and two workers on one lock. AC7 is enforced by arch-lint, which keeps lua, automation and entitymanager out of the realtime package; the rule now says so.
status: addressed
---
