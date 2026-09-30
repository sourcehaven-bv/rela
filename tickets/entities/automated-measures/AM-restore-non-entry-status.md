---
id: AM-restore-non-entry-status
type: automated-measure
title: History restore of a non-entry status is pinned
description: Tests pin that a history restore skips the state machine's entry rule but still requires a declared edge into the value whose guard the principal holds; that an ordinary create at a non-entry state is still refused; and an archguard pins every entry point and call site of RecreateEntity to history restore.
kind: test
location: internal/statemachine/restore_test.go, internal/entitymanager/transition_test.go, internal/archguard/recreate_test.go
status: active
---
