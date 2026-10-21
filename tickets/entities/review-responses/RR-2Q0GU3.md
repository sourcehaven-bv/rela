---
id: RR-2Q0GU3
type: review-response
title: Name-length edge case only considers entity ids
finding: The plan says the CreateTemp suffix (about 15 bytes with the dots) is safe because entity file names are short ids. SafeFS also writes relation files named FROM[@face]--TYPE--TO[@face].md, comment thread keys and channel names. Those are longer and partly user-chosen. Check the longest valid relation and channel file names against NAME_MAX (255) minus the suffix. If any can come close, add a test that writes one.
severity: nit
resolution: 'Plan updated: the edge case covers relation file names, comment keys and channel names, and truncates the base part of the temp name if needed.'
status: addressed
---
