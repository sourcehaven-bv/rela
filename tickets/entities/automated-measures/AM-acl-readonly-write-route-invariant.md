---
id: AM-acl-readonly-write-route-invariant
type: automated-measure
title: 'Integration test: every /api write route is ACL-gated (no write lands under ReadOnlyACL)'
description: Enumerates every registered /api write route and asserts each produces a denied-write / no store mutation under ReadOnlyACL, including the relations-only PATCH dangling-peer case. Catches any future un-gated write path (the class that produced BUG-K6FEVB and prior BUG-JME1DI). This is the P4 invariant test — integration level, not per-handler.
kind: test
location: internal/dataentry (integration test, to be added in the BUG-K6FEVB fix PR)
status: proposed
---
