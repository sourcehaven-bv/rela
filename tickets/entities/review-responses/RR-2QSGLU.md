---
id: RR-2QSGLU
type: review-response
title: Document-render singleflight key omits the principal — cross-principal render collapse
finding: 'documentService.Render dedupes concurrent renders on entryID+configID. Harmless while renders were principal-free, but ExecuteDocument now threads the caller''s principal: two concurrent requests from DIFFERENT principals collapse onto one render — the follower gets output produced under the leader''s identity (rela.principal) and inherits the leader''s ctx cancellation. Today content doesn''t vary by principal, so impact is attribution/cancellation coupling; the moment TKT-ZF2DTV makes Lua reads principal-scoped this silently becomes a data leak (A''s request returns content rendered under B''s ACL). Export''s RenderMarkdown doesn''t singleflight and is unaffected.'
severity: significant
resolution: Singleflight key now includes the ctx principal (user+tool) alongside entryID+ConfigID, with a comment explaining why (identity-bearing renders since TKT-L9Q669; prevents the TKT-ZF2DTV-era cross-principal data leak preemptively). Export's RenderMarkdown path was already singleflight-free.
status: addressed
---
