---
id: RR-R9FUCS
type: review-response
title: 'AC5 is wrong: a declared type with an empty Plural is lost on reopen too'
finding: buildPluralToTypeMap (layout.go) adds only schemas with Plural != "". entityFileKey writes such a type to '<type>s', and the scan cannot map that directory back, so the entity vanishes exactly like an undeclared type. app.buildSchemas always sets Plural, but Config accepts an empty one, and the new check would let it through. Pick one fix. (a) fsstore.New rejects a schema with an empty Plural. (b) One plural function, which defaults to type+'s', serves both entityFileKey and buildPluralToTypeMap. Rewrite AC5 to test Plural:"" plus reopen.
severity: significant
resolution: 'Plan updated: New and SetSchemas normalize an empty Plural to <type>s, so writing and scanning use one plural rule; AC5 tests Plural "" plus reopen.'
status: addressed
---
