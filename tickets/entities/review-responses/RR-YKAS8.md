---
id: RR-YKAS8
type: review-response
title: Document that hidden-branch pruning is client-only (server enforcement note)
finding: The contract (client conditions UX-only; server re-validates against metamodel required, not required_when/visible_when) is sound but only discoverable across three files. A future edit to visibleWritablePropertiesForCommit could re-introduce the create-path leak. Worth an explicit doc note enumerating what a non-SPA client can submit.
severity: minor
resolution: docs/data-entry.md already states conditions are a UX affordance only and the server re-validates every write. Added the reconciliation note in code (pruneWizardHidden is the single point both prune systems compose at) with a comment flagging the create-path interaction, so the seam is discoverable where the risk lives.
status: addressed
---
