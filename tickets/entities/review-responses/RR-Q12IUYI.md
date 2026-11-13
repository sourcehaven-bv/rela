---
id: RR-Q12IUYI
type: review-response
title: Unknown option keys were ignored
finding: A misspelled option (expect_mail, fake.date, t.principal, actor options) was ignored, so the assertion it meant to make never ran.
severity: significant
resolution: optionTable raises unknown option X and lists the allowed options. TestTyposFail covers it.
status: addressed
---
