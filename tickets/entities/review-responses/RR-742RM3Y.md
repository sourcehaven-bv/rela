---
id: RR-742RM3Y
type: review-response
title: SPA autosave may stall on an old disallowed edge
finding: useAutoSave bundles property edits with a dirty relations body; a 422 for an old disallowed edge would fail the bundled save.
severity: minor
reason: A relations body is only dirty after the user edits that relation; the form shows the 422 like any other. The project data was checked clean. Follow-up only if it shows up in practice.
status: deferred
---
