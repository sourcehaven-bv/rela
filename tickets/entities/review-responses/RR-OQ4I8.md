---
id: RR-OQ4I8
type: review-response
title: AppState mutated in place in handleAPISaveSettings/SavePalette
finding: handleAPISaveSettings does `a.State().UserDefaults = &ud` and handleAPISavePalette does the same with UserPalette + Palette. This mutates the immutable AppState snapshot through the shared pointer; concurrent readers that loaded the same pointer observe a racy unsynchronized write. writeMu serializes writers but readers never take it.
severity: critical
resolution: Introduced App.mutateState helper that takes writeMu, builds a shallow copy of the current AppState, runs the caller's mutator on the copy, and publishes the copy via state.Store. Both handleAPISaveSettings and handleAPISavePalette now use mutateState; they no longer scribble through a.State() to assign field values directly.
status: addressed
---
