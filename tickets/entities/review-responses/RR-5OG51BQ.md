---
id: RR-5OG51BQ
type: review-response
title: Merge-function mock tests the least risky code
finding: Base movement is the dangerous code. Run one-off mutations of useAutoSave.ts (moveBase applied guard, mergedContent guard, holdsProp ignoring queuedProps, own-write 412) and confirm the property fails.
severity: significant
resolution: 'Ran one-off mutations: moveBase always moving, holdsProp ignoring queuedProps, unsaved fields not held (props and body), no rebase, merged save leaving the old base, per-save base not following a merge. Each fails the property; the rare one also fails a unit test. The own-write 412 rule no longer exists; the mergeProperty write-ours fault covers that behaviour.'
status: addressed
---
