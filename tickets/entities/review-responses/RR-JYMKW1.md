---
id: RR-JYMKW1
type: review-response
title: '--run-as accepts look-alike system: names that pass for rela''s own identities'
finding: "validateRunAs (identity.go:87-96) checks only the prefix, a denylist of exact names (ownJobIdentity, identity.go:99-101), and principal.ValidateRunAs, which rejects only Cc characters (principal.go:647). Format characters (Cf) and homoglyphs therefore pass: `system:realtime` followed by U+200B, U+2060 or U+202E, or `system:rеaltime` with a Cyrillic е.\t`RELA_RUN_AS=$'system:realtime​' rela channel post ...` is accepted. The SPA and MCP show the author as system:realtime, so the message passes for a trigger post, which the refusal list exists to prevent (docs/acl-security.md:1074-1077). The CLI table shows U+FFFD, but other readers do not.\tValidate the part after `system:` with a character set, as integration names already are (ValidIntegrationName: lowercase ASCII letters, digits, `-` and `_`, and `:` if needed). Then check the denylist against that normalized name. Add test cases for a zero-width character, a bidi control and a Cyrillic homoglyph."
severity: significant
resolution: 'A system: name must match principal.ValidSystemName (segments of lowercase ASCII letters, digits, - and _, separated by :) before the registry check. Tests cover U+200B, U+2060, U+202E, a Cyrillic homoglyph, upper case and an empty segment.'
reason: 'A system: name must match principal.ValidSystemName (segments of lowercase ASCII letters, digits, - and _, separated by :) before the registry check. Tests cover U+200B, U+2060, U+202E, a Cyrillic homoglyph, upper case and an empty segment.'
status: addressed
---
