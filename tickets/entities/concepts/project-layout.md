---
id: project-layout
type: concept
title: Project Layout and Discovery
summary: On-disk project file conventions and the root-discovery walk that locates them
description: The set of operator-authored files that constitute a rela project (schema.yaml, data-entry.yaml, acl.yaml, schedules.yaml, scripts/, actions/, templates/, entities/, relations/, .rela/) and the Discover() walk in internal/project that locates the project root by looking for a marker file. Owns naming conventions for these files, backward-compatible discovery of legacy names, and the Context struct that publishes resolved paths to every consumer.
package: internal/project
layer: core
status: stable
---
