---
id: BUG-7928QLO
type: bug
title: Analyze reports hidden fields as missing
description: The Properties check of the data-entry analysis reports a required field the reader cannot see as missing and the message does not name the field.
priority: medium
effort: s
why1: The Properties check validates the redacted row; a hidden field has no value there, so a required hidden field reads as missing.
why2: analyzeProperties passes every finding on and never compares it with the header's Redacted list.
why3: TKT-3FL2S6 moved analyze to a gated reader to close value leaks; the tests it added check that no hidden value leaks, not that a hidden field produces no finding.
why4: The validator returns the property name in a separate field and the message alone was shown, so the finding never named its field.
why5: Checks that run on redacted data have no shared rule for what an absent hidden value means, so each check must handle it on its own.
prevention: TestACLAnalyze_PropertiesSkipsHiddenFields pins that a hidden field produces no finding and that a finding names its field label.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

Seen in the harbourmaster customer portal: a customer opens the analysis and
sees errors on fields that `visible:` hides from them. Each finding only says
"This field is required" without naming the field.
