---
id: FEAT-drlm
type: feature
title: View-scoped analysis for analyze commands
description: Allow analysis commands to be scoped to entities resolved by a view, enabling validation of specific documents or releases before publishing. Adds --view and --entry flags to all analyze subcommands.
status: proposed
---

Allow analysis commands to be scoped to entities resolved by a view, enabling
validation of specific documents or releases before publishing.

Adds `--view` and `--entry` flags to all analyze subcommands. When specified,
only entities included in the view result are analyzed, while still checking
constraints against the global graph state.
