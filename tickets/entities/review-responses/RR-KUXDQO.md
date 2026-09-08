---
id: RR-KUXDQO
type: review-response
title: nextActionPrefilters re-parsed the query per request and coupled type order to the matcher
finding: dataentry re-ran searchparser.ParseQuery on every request only to recover EntityTypes that conditionlint had already parsed at compile time, and appbuild's Prefilters relied on types[0] matching the matcher's program map by unsorted parse order.
severity: nit
resolution: conditionlint.NextActionMatcher exposes Types() (sorted, the compiled types); ConditionPrefilterer.Prefilters(ctx, meta) takes no types and the matcher supplies its own. dataentry no longer imports searchparser in nextaction.go. TestNextActionMatcher_TypesAreTheCompiledTypes.
status: addressed
---
