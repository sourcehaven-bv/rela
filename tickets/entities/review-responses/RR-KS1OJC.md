---
id: RR-KS1OJC
type: review-response
title: printable only replaces Cc, so the planned bidi-override test fails or the helper is wrong
finding: db_importfs.go printable maps unicode.IsControl, which excludes U+202E and other Cf format runes. Yet section 7 lists a bidi override in TestChannelRender_ControlCharsAndIndent. Specify that the shared printable also replaces bidi controls (U+202A-202E, U+2066-2069) and other Cf runes except ZWJ, or drop the claim.
severity: minor
resolution: The shared printable replaces Cc and Cf runes (bidi controls included) except ZWJ.
status: addressed
---
