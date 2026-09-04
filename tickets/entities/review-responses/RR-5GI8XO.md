---
id: RR-5GI8XO
type: review-response
title: 'PR-A: headless rename handed observers a state face - search index overwrite'
finding: 'Both backends'' renameEntity fell back to renamedStates[0] when the family had no default face (headless, which load tolerance permits), handing a NON-DEFAULT state to EntityRenamed — whose consumers key search documents by bare id. Exactly the leak the notifyPut skip exists to prevent; the back door was the rename path. Memstore was worse: it initialized to renamedStates[0] unconditionally.'
severity: significant
resolution: Both backends now skip notifyRenamed entirely for a headless family (nil renamedDefault), with the rationale in a comment. Pinned by fsstore's new TestObservers_HeadlessRenameSkips (hand-written headless state file, rename succeeds, zero observer calls) and TestObservers_SkipNonDefaultStates (put/update/rename paths).
status: addressed
---
