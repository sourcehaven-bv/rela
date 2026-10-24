---
id: RR-HY1NRF
type: review-response
title: appendTarget's doc comment now sits above appendSession
finding: 'internal/channels/sqlchannels/sql.go:463-464: the two lines ''// appendTarget returns the channel an append goes to...'' were left above the new appendSession comment, so godoc for appendSession starts with the wrong sentence and appendTarget (line 490) has no doc. Move the two lines down to appendTarget.'
severity: nit
resolution: The appendTarget doc comment is back on appendTarget.
status: addressed
---
