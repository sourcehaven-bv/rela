---
id: RR-NHQH
type: review-response
title: Allowlist test cases mislabeled and one is functionally redundant
finding: |-
    Two issues in `insertEntityRef.test.ts` allowlist cases:

    1. **Wrong label**: `['null byte', 'TKT- -1']` — the value `'TKT- -1'` is 'TKT' + dash + space + dash + '1'. That's a SPACE, not a null byte. A real null byte is `'\0'`. The test name is wrong, and crucially the regex (which doesn't include `\0` in its character class) would also reject a real null-byte case — but a reviewer reading 'null byte' thinks they have null-byte coverage when they have space-coverage instead (duplicating the 'whitespace' case above). Rename to 'space within ID' or add a real `['null byte', 'TKT-\0-1']` case.

    2. **Underscore inconsistency between description and allowlist**: the regex is `/^[A-Za-z][A-Za-z0-9_-]{0,255}$/` — it ALLOWS underscores in the body. But the file-top comment says 'manual IDs ("data-entry-ui")' and the 'leading underscore' test rejects `_TKT`. That's fine. The 'accepts manual IDs containing hyphens and underscores' test uses `'data-entry_ui'` — fine. But there's no manual-id entity in the repo today that uses underscores; the regex allows them, the test asserts they pass, but the assumption that underscores are valid in real manual IDs is unverified against the metamodel. Worth a sanity-check: does the metamodel emit any underscore-containing IDs? If no, the allowed-character set is bigger than the actual ID space — not a correctness issue, just a 'why is this here' question.

    3. **Test count vs assurance ratio**: 28 cases for ~30 lines of helper code is heavy. After de-duplicating the 'replaces selection' identical-to-happy-path case (see RR-EBGN), the null-byte fix, and consolidating the 14 'rejected' cases via the existing `for...of` loop already does, the suite is tight. The current 28 number is a vanity metric, not a coverage win.
severity: nit
resolution: The 'NUL byte' / 'DEL character' test cases use real NUL (0x00) and DEL (0x7f) bytes in the source -- verified via `od -c`. The labels are correct. Removed the redundant 'replaces the current selection' duplicate case (RR-EBGN), narrowing the test count to 31 focused assertions. Underscore in the regex is now moot since RR-D54M switched to a denylist.
status: addressed
---
