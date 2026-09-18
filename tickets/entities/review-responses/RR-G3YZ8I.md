---
id: RR-G3YZ8I
type: review-response
title: A partly-tokenizable query silently drops rows the server matched
finding: 'The guard was `uf.split(query).length === 0`, which only catches a PURELY untokenizable query. uFuzzy''s Latin-oriented splitter silently discards what it does not match, so a partly-Latin query was ranked on the fragment alone: `日本語-jp` splits to [''jp''], dropping a row the server matched on the CJK substring. This is the same ''No matches over a good response'' failure the empty-split guard exists to prevent, reached through a different door. My own probe found the reviewer''s case understated it: accented Latin is affected too (`café` -> [''caf''], `naïve` -> [''na'',''ve'']), so this hits ordinary European titles, not only CJK or Cyrillic.'
severity: significant
resolution: 'Added `tokenizerSawWholeNeedle(query, terms)`: the terms'' combined length must cover the query''s letter/number characters (separators excluded, so `fancy-rank` stays fully covered). Both rankers use it in place of the bare empty-split check. Note the first version of this fix REGRESSED the all-separator case (''---'' has zero meaningful characters, so 0 >= 0 read as fully covered and fell through to a filter that matched nothing) - caught immediately by the existing tests, and now handled by an explicit `terms.length === 0` branch. Pinned by a mixed-script matrix over six query shapes plus a case asserting the guard does not swallow a normal hyphenated query.'
status: addressed
---
