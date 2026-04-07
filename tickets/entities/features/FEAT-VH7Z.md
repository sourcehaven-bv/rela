---
id: FEAT-VH7Z
type: feature
title: Structured leveled logging via log/slog
summary: Use log/slog throughout internal library code for structured, leveled, parallel-test-safe logging.
description: 'Replace stdlib log usage in library packages with log/slog. Benefits: (1) parallel-test-safety — slog handlers are safe to capture per-test, unlike the stdlib log package which uses global mutable state via log.SetOutput and races with t.Parallel; (2) leveling — support for --verbose / --quiet to bump log level at entry points; (3) structured attributes — logs become machine-readable with named fields rather than sprintf strings. Enforcement via a depguard lint rule that forbids stdlib log imports with narrow exemptions (internal/mcp/server.go must bridge to mcp-go''s WithErrorLogger which takes *log.Logger).'
priority: medium
status: proposed
---
