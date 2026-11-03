---
id: AM-served-principals-are-reserved
type: automated-measure
title: Every principal the server stamps lies under a reserved prefix
description: Enumerate the principal names the server stamps for non-human callers (scheduler and automation and webhooks and provisioner) and assert that principal.IsReserved refuses each one when it arrives from a request.
kind: test
location: internal/principal (test over the server's stamped principal names)
status: proposed
---
