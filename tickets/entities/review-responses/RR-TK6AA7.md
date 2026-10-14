---
id: RR-TK6AA7
type: review-response
title: 'Reserved module names: builtins vs rela globals'
finding: _LOADED holds the stdlib names but not rela's own globals (rela, http, ai, crypto, mail), so require('http') looks in lib/http.lua; a future RegisterModule would silently shadow a user module of the same name.
severity: minor
resolution: User modules are cached in a Go-side map, stdlib names are served from the registry, and reserved first segments (rela, http, ai, crypto, mail, stdlib names, os, io, debug, package, channel, utf8, bit32) are rejected. Covered in TestRequire_InvalidNames and TestRequire_StandardLibraryIgnoresReassignedGlobal.
status: addressed
---
