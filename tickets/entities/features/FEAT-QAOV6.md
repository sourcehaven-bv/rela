---
id: FEAT-QAOV6
type: feature
title: Built-in scheduled task runner for Lua scripts
status: proposed
description: Built-in scheduler that runs Lua scripts on cron-like schedules without external task scheduling infrastructure
---

## Description

A built-in scheduled task runner that executes Lua scripts on configurable cron-like schedules, eliminating the need for external cron or systemd timers. This enables rela projects to define recurring automation (analysis, validation, reports) as part of the project itself.
