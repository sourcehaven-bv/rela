---
id: RR-FCDLGD
type: review-response
title: MatchingRefs pushes ScriptReader and UnrestrictedReader over the plimsoll exported-method cap
finding: 'ScriptReader and UnrestrictedReader each have exactly 20 exported methods today, with no plimsoll directive. Adding MatchingRefs makes 21, so `just plimsoll` fails. It also widens scriptEntityReaderFamily for every consumer, although only realtime needs the method. Fix: do not add a method to the reader family. Add a small visibility type or function (for example visibility.HolderLookup, built in scriptReads from the same gate, redactor, store and world, with deny and unrestricted variants) and wire it only into appbuild''s holderFinder. This also follows the consumer-side interface rule.'
severity: significant
resolution: No reader-family method. New visibility.HolderLookup (gated, unrestricted, refusing variants) built in scriptReads from the same inputs and wired only into appbuild's holderFinder. Confirmed both readers sit at 20 exported methods.
status: addressed
---
