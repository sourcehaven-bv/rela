---
id: RR-SMGZU1L
type: review-response
title: Conversion undoes the self-mention rewrite on fs/mem
finding: buildTargets copies source.Content before RenameFamily, and ReplaceEntityFamily writes that content back, so a self-mention of the old id survives conversion.
severity: minor
resolution: Conversion keeps the store's self-mention rewrite (keepSelfMentionRewrite applies mentions.Rewrite to the replacement contents); TestConvert_RewritesMentionsAndAuditsThem.
status: addressed
---
