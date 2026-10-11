---
id: AM-cli-writes-carry-principal
type: automated-measure
title: Every CLI write command runs with the CLI principal
description: A test drives each CLI write command against a recording audit sink and asserts the record carries the CLI principal, not an empty one.
kind: test
location: internal/cli
status: proposed
---
