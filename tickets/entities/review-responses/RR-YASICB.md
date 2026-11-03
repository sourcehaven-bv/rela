---
id: RR-YASICB
type: review-response
title: Text value can forge a setext heading
finding: === or --- alone on an appended line turned the previous line into a heading and ended the section.
severity: significant
resolution: EscapeMarkdown also escapes - = +; TestRun_TextCannotForgeASetextHeading fails without it.
status: addressed
---
