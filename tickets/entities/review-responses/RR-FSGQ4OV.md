---
id: RR-FSGQ4OV
type: review-response
title: TOCTOU between bound entity read and elevated write
finding: 'A concurrent change to the bound trigger entity after when: was judged would not fail the elevated write.'
severity: minor
resolution: The action pins the raw bound row (ScopedUpdates.Pin). An elevated write to it fails with a value-free store.ErrConflict when the row's version moved; the store write is pinned to the read version. ExpectedVersion on the patch was not used because the script sees a redacted row and a conflict error would carry the raw token. The pin moves to each elevated write so a script may update twice. TestScopedUpdates_Pin and TestAction_ElevatedUpdatesBound.
status: addressed
---
