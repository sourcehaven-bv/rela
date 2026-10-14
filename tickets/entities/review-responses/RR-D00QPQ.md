---
id: RR-D00QPQ
type: review-response
title: ReadScript reuse contradicts AC7 messages
finding: ReadScript's errors say 'script not found ... (must be in lib/ directory)', not the 'module not found' message AC7 requires; the code calls files.Load directly.
severity: nit
resolution: 'Plan text corrected: the loader calls ProjectFiles.Load directly with module-specific messages.'
status: addressed
---
