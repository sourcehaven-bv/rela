---
id: RR-UHJKFF
type: review-response
title: Library sorts by localeCompare, server by bytes
finding: 'channelFields.ts:46-49. byRequiredThenName uses String.localeCompare, which is locale-dependent and case-folding. The server (and so the descriptor order, the CLI and MCP) sorts names bytewise. With names like `Due`, `due_at`, `_x` the composer order differs from the order the server and plan D2 define, and it can differ between users'' locales. Fix: compare with `a.name < b.name ? -1 : a.name > b.name ? 1 : 0`, or keep the descriptor''s order (it is already required-then-name) with a stable partition on required.'
severity: minor
resolution: Names are now compared by code unit instead of localeCompare, which agrees with the server's byte order for ASCII names.
status: addressed
---
