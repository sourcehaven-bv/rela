---
id: RR-TLTTMLZ
type: review-response
title: on_gone delete can delete a locally created pushed row
finding: 'A pushed create got Adopted false, so on_gone: delete deleted the local original when the remote stopped listing it.'
severity: significant
resolution: Pushed creates are recorded with Adopted true, so they are kept and detached. TestPush_PushedRowIsNotDeletedWhenGone; docs updated.
status: addressed
---
