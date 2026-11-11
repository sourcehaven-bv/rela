---
id: RR-BCQIG7Z
type: review-response
title: Data-entry writes skip the recheck inside the write Tx
finding: 'pr-guidance G-005: the data-entry app writes through the ungated manager and checks fields in its handlers, so the in-Tx flow recheck (only on the FieldGated handle) never ran for PATCH, attachment upload or history restore. On postgres and sqlite a run started between the handler''s check and the write was not seen.'
severity: significant
resolution: UpdateGrant.PatchEntity asks FieldWriteGate.WriteRecheck inside the write Tx on any handle (patchPre.recheck), and the PATCH handler and history restore write through the grant. The data-entry attachment owner is built with Attachments.WithWriteRecheck. The handlers map the in-Tx refusal to the same 403 field-affordance answer as their own check (writeFieldWriteRefused); the manager audits it. Pinned by TestUpdateGrant_RecheckOnUngatedHandle and the HTTP race tests TestOutsideEdits_RunStartedDuring{HTTPPatch,HistoryRestore}FailsClosed (sqlite and postgres) and TestOutsideEdits_RunStartedDuringAttachmentUploadFailsClosed (postgres). Each fails when its recheck is removed.
status: addressed
---
