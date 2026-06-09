---
id: RR-FA1E
type: review-response
title: Disabled-channel schedule* error type and message
finding: |
  Plan says "developer error" but doesn't name the error. A generic throw new Error('disabled') is unsearchable; an unnamed throw in scheduleFieldSave is hostile to grep.
severity: minor
status: addressed
resolution: |
  Throw `new Error(\`useAutoSave: \${channelName} channel is disabled; remove the disable\${ChannelName}Channel flag or stop calling \${methodName}\`)`. Test asserts the channel name appears in the message. No custom error class -- this is dev-time, not catch-able runtime control flow.
---
