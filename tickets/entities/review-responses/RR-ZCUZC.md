---
id: RR-ZCUZC
type: review-response
title: as never cast in CommandModal.test.ts is the bluntest possible workaround
finding: Use wrapper.findComponent(CommandModal).vm to grab the instance instead of a typed ref callback, or cast only the function-signature mismatch. as never works but invites future confusion.
severity: minor
resolution: Replaced ref callback + as never cast with wrapper.findComponent(CommandModal).vm + a single 'as unknown as' cast for the runCommand method. Cleaner and idiomatic for Vue Test Utils.
status: addressed
---
