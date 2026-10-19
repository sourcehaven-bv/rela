---
id: RR-6XIF2F
type: review-response
title: A look-alike $USER passes the new reserved-name refusal
finding: 'newLocalIdentity refuses $USER only when principal.IsReserved matches. That check is an exact ASCII prefix test, after trimming leading ASCII space and C0 controls only. These values therefore pass: `USER=$''⁠system:realtime''` or `$''​system:realtime''` (an invisible prefix), `USER=ѕystem:realtime` with a Cyrillic ѕ in the prefix, and `USER=System:realtime`. A homoglyph after the prefix is refused, because the prefix still matches. Such a principal posts channel messages and writes the graph with no asserted_by and no notice. The SPA shows the author as system:realtime. The CLI table shows it the same way for U+2060, because this diff changed printable to keep the word joiner, which it used to replace. ACL grants match the exact name, so this only spoofs what readers see. That is the threat the refusal list exists for, and the --run-as path now closes it with ValidSystemName. Fix: on the no-run-as path, also refuse a $USER that contains a Cf, Zl or Zp rune, and compare the reserved prefixes case-insensitively. Accept or document the residual homoglyph risk for a free-form $USER. Consider whether printable should keep replacing U+2060 and the tag characters U+E0000-E007F.'
severity: minor
resolution: 'On the no-run-as path, a $USER with any control, format, line or paragraph separator rune is refused, and the system:/integration: prefix check is case-insensitive. printable again replaces U+2060, and now also the tag characters U+E0000-E007F. The remaining risk from look-alike letters in a free-form $USER is documented in acl-security.md and cli-reference.md (display only; ACL grants match the exact name). Tests added.'
status: addressed
---
