---
id: RR-H1PU3U
type: review-response
title: 'Foreign property types picked by alphabetical order'
finding: 'For foreign properties the first type in sorted order decided the declared type, so renaming a type could change whether a condition compiles; a mentioned_by step accepts any property some type declares.'
severity: significant
resolution: 'Foreign properties are all NilType now, so no order dependence remains. A typo that matches a property of another collected type still compiles and reads nil; that is the price of mixed collections and is documented.'
status: addressed
---
