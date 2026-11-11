---
id: RR-BN23OMF
type: review-response
title: Elevated update writes on a read-only instance
finding: 'CISO review on PR #2114: under --read-only the elevated handle skipped AuthorizeWrite, so an allow_acl_bypass: write action changed status while rela.update_entity in the same setup was refused.'
severity: significant
resolution: ScopedUpdates asks access.ModeOf whether the instance refuses writes and then runs the normal AuthorizeWrite and audit on the non-elevated handle. The update fails as an ordinary one does with a denied-write row. TestScopedUpdates_ReadOnly and TestAction_ElevatedUpdatesReadOnly pin it. The holdsActionPermission comment is corrected.
status: addressed
---
