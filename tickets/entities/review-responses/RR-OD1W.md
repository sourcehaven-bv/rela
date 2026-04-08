---
id: RR-OD1W
type: review-response
title: 'F18: parseChatRequest double-reads opts.RawGetString(model)'
finding: Code path called RawGetString('model') twice and used a different style from the temperature/max_tokens optional parsers directly below it.
severity: nit
resolution: 'Refactored to match the temperature/max_tokens style: read the value once into a local, check against lua.LNil, then type-assert. Single read, consistent with the rest of the function.'
status: addressed
---
