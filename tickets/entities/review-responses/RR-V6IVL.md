---
id: RR-V6IVL
type: review-response
title: TestValidateCreateIDOpts table missed key combinations
finding: 'The validator''s table-driven tests covered the basic matrix but missed: short type with both id and prefix set (precedence), manualPrefixed with matching id AND prefix also set, bare prefix as id (TAG-), whitespace-only id, whitespace-only prefix.'
severity: significant
resolution: 'Added 5 new test rows: ''short, both id and prefix set — id rejection wins'', ''manual prefixed, id matches AND prefix also set'' (→ prefix not applicable), ''manual prefixed, bare prefix as id'' (→ must start with), ''manual prefixed, whitespace-only id treated as empty'', ''short, whitespace prefix treated as empty''. All 17 rows pass.'
status: addressed
---
