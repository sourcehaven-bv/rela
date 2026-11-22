---
id: RR-MG0LMAS
type: review-response
title: The title falls back to the shell name during a reload
finding: reload() sets loaded to false, so the name getter returned the boot meta again while the language kept the config value. A failed reload left the two out of step.
severity: minor
resolution: The composable latches configSeen when loaded first becomes true and gates both the language and the name on it. A test unsets loaded after the first load and checks both keep the config.
status: addressed
---
