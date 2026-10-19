---
id: RR-AN3EUH
type: review-response
title: printable is too strict for some Unicode and lets line separators through
finding: "printable (internal/cli/printable.go:17-23) replaces every Cf character except ZWJ, including the soft hyphen U+00AD, the word joiner and the Arabic letter mark, which appear in real text. U+2028 and U+2029 (categories Zl and Zp) pass through unchanged.\tA Dutch or Arabic message shows U+FFFD where the text was valid. A terminal that treats U+2028 as a line break lets content start a line without the `  | ` prefix.\tReplace the bidi controls (U+202A-202E, U+2066-2069, U+200E, U+200F, U+061C) and zero-width characters explicitly, keep the harmless Cf characters, and add U+2028 and U+2029 to the replaced set."
severity: nit
resolution: printable now replaces Cc, U+2028/2029, the bidi controls (U+061C, U+200E/F, U+202A-202E, U+2066-2069), U+200B and U+FEFF, and keeps other format characters (soft hyphen, ZWJ, ZWNJ, word joiner). Tests added.
reason: printable now replaces Cc, U+2028/2029, the bidi controls (U+061C, U+200E/F, U+202A-202E, U+2066-2069), U+200B and U+FEFF, and keeps other format characters (soft hyphen, ZWJ, ZWNJ, word joiner). Tests added.
status: addressed
---
