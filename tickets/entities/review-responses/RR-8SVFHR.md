---
id: RR-8SVFHR
type: review-response
title: Plimsoll directives on both channel backends need bumping and are near the method cap
finding: filechannels.Store and sqlchannels.Store carry max-exported-methods=21 and already have 33 methods each. Adding Tombstones, Purge and DropTypeRecord takes exported methods to 24 and pushes the total toward 40. The plan's file table does not mention the directive bump or keeping helpers as functions.
severity: minor
resolution: The directives are bumped with a "required interface" note and new helpers stay free functions.
status: addressed
---
