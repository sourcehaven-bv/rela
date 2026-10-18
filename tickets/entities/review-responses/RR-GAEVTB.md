---
id: RR-GAEVTB
type: review-response
title: CLI -P is a kong []string that splits on commas, breaking message text
finding: create.go and update.go declare `Property []string short:"P"`, and kong splits those on commas (the documented rela -P quirk). Copying that into channel post or edit turns "body=Hello, world" into two properties and rejects it. Section 4 must declare `sep:"none"` on the channel -P flag and add a comma and newline case to TestChannelCLI_*.
severity: significant
resolution: The channel -P flag declares sep:"none". TestChannelCLI_* has comma and newline cases.
status: addressed
---
