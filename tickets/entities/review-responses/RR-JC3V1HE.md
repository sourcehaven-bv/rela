---
id: RR-JC3V1HE
type: review-response
title: "The fallback language differed from the docs"
finding: "The code fell back to English whenever English was offered; the config comment and the guides say the first entry of app.languages is the fallback."
severity: significant
resolution: "The fallback is now the first offered language, as documented; a test covers [nl, en]."
status: addressed
---
