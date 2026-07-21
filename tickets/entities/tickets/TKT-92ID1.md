---
id: TKT-92ID1
type: ticket
title: Introduce RootedFS and enforce path-validation boundary via arch lint
kind: enhancement
priority: medium
effort: m
status: wont-fix
---

> **Closed by backlog sweep (2026-07-20):** substantively done — RootedFS exists (internal/storage/rooted.go) and the path-validation boundary is enforced at compile time by the type seam (RootedFS deliberately does not implement storage.FS), which is stronger than the proposed arch-lint rule. The residual lint-rule idea (ban raw os/io in high-level components) is tracked by TKT-K3YYE.

