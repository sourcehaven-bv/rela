---
id: RR-YBCEOC
type: review-response
title: Byte-vs-character offset loses every comment on a multi-byte id rename
finding: 'Both database backends computed the moved key as newID plus everything after the old id, slicing at len(oldID)+1 from Go. Go''s len() counts BYTES; SQL''s substring(text from int) and substr() count CHARACTERS. For an id containing a multi-byte rune the offset overshoots, dropping the "@" separator and re-keying every comment to a corrupt target. Verified on the live database: char_length(''TKT-é'') is 5 where Go''s len is 6, so substring(''TKT-é@draft'' from 7) yields ''draft'' rather than ''@draft''. Worse than the collision case because Rename returns nil — nothing anywhere records that the comments are gone. entity.ValidateID admits only ASCII today, so the two units agreed by luck; the code was correct only by the grace of a grammar declared in a package it deliberately does not import, with no test and no comment recording the dependency — while facePrefixPattern twelve lines below escapes ''%'' with the explicit justification that it must not depend on exactly that.'
severity: critical
resolution: 'Both sides of the arithmetic now happen in SQL: char_length($1::text) + 1 on PostgreSQL, length(?) + 1 on SQLite, matching the character-counting substring/substr they feed. No Go-side byte count is involved, so the units agree by construction regardless of what entity.ValidateID later admits. Pinned by a commentstest case renaming a faced thread whose id holds a multi-byte rune, verified to fail against the byte-offset form.'
status: addressed
---
