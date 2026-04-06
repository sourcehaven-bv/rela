---
id: RR-UWZW
type: review-response
title: RunFile error messages will lose filename context
finding: The plan proposes modifying RunFile() to read the file and call RunString(). However, L.DoFile() provides filename context in error messages. After the change, errors will show '<string>' instead of the filename. Consider using L.LoadString() with a chunk name parameter to preserve filename in errors, or accept this as a minor trade-off.
severity: minor
resolution: 'Updated plan: RunFile() will use L.LoadString() with chunk name set to filename to preserve error context.'
status: addressed
---
