---
id: RR-WBQC
type: review-response
title: ChainResolvers ignores Tool, only checks User
finding: The chain advances on p.User == "" regardless of p.Tool. Today this is fine — every resolver hard-codes Tool=ToolDataEntry — but the contract is invisible. A future resolver returning {User:"", Tool:"something"} would have its Tool silently dropped.
severity: significant
resolution: 'Documented the chain contract on ChainResolvers: zero User signals fall-through; Tool is ignored. Future resolvers needing a different Tool must also provide a non-empty User. No code change — the chain already does the right thing; the comment makes the contract explicit. File: internal/dataentry/router.go:177-194.'
status: addressed
---
