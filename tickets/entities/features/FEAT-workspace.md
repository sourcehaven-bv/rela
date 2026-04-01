---
id: FEAT-workspace
type: feature
title: Workspace service layer abstraction
description: Unified service layer for all entity/relation operations
summary: Single entry point eliminating dual-write patterns
status: implemented
priority: high
---

# Workspace Service Layer

Introduce a unified workspace service layer that:
- Provides single entry point for all entity/relation operations
- Eliminates dual-write patterns (direct file + graph)
- Unifies file watching across consumers
- Exposes project-level accessors consistently
