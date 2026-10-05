---
id: RR-V6WB01
type: review-response
title: Control characters in values reach the terminal
finding: oneLine replaced only CR and LF, so an escape sequence in an error value would act on a terminal printing the tree.
severity: minor
resolution: oneLine replaces every control character; TestTextStripsControlCharacters.
status: addressed
---
