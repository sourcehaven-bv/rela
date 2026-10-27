---
id: RR-DIYY1P
type: review-response
title: fs Reads cost grows with segment size, not with the cap, and no budget test can see it
finding: 'filechannels.Reads plans to "read the log from the marker''s segment to the end". A segment holds up to 1000 messages plus up to 2000 patch records (megabytes), so each key costs a whole segment read even when one message is unread. Every live head and list replay now runs this, for every subscriber. A key whose LastSeq equals the marker (most keys on a list) is not skipped either. The dataentry budget tests count channels.Store calls, so they cannot catch per-key file reads inside one Reads call. Fix: per key, read lastSeq through the existing tail reader first and skip the key when lastSeq is at or below the marker. Otherwise scan backward from the end with the doubling tail (as lastMessage does) and stop at a seq at or below the marker or at UnreadCap. Pin it with a filechannels test that counts bytes or segments read (a 2000-message channel with one unread reads one tail chunk).'
severity: significant
resolution: fs Reads skips keys whose tail last seq is at or below the marker, else scans backward with the doubling tail and stops at the marker or the cap. TestReads_TailCost counts bytes read.
status: addressed
---
