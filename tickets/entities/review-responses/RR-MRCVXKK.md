---
id: RR-MRCVXKK
type: review-response
title: Rename of a subject with an open run fails open
finding: The rename fix rekeyed runs after the rename committed. A rekey that failed only logged, so the subject kept its new id with no run and every outside_edits rule on it was lifted.
severity: significant
resolution: The manager asks a RenameGuard inside the rename's store Tx (and before a dry run). The flow guard refuses with ErrRenameBlocked while any run on the subject is stored, and fails closed when the run store cannot be read. Run starts take the same store lock on pg and sqlite. The post-commit rekey stays as the fs backstop only. Pinned by TestRenameGuard, TestFlowRenameGuard and TestOutsideEdits_RenameRefusedWhileHeld.
status: addressed
---
