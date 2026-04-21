---
id: FEAT-HIWW4
type: feature
title: Readable schema diagrams via filtering and legend bundling
summary: '`rela schema --graphviz` produces readable diagrams for large/polymorphic metamodels by excluding entity types on request and auto-collapsing universal relations into a compact legend table.'
description: 'Enhances `rela schema --graphviz` with: (a) `--exclude <type>` flag to omit entity types and their edges; (b) auto-detection of ''universal'' relations (target count exceeds a ratio of total entity types) which get rendered as a legend table rather than as a fan of edges; (c) smart target-list rendering in the legend (full list for small N, complement phrasing ''any entity except X'' when nearly all, plain ''any entity'' when exactly all).'
priority: medium
status: proposed
---
