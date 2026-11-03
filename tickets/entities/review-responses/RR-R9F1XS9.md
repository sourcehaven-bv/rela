---
id: RR-R9F1XS9
type: review-response
title: Sidebar badge would pay for message reads
finding: Setting Messages on /_inbox/summary makes every summary-only live round (every logged-in tab, sidebar badge) run GetMessages, a ref resolve and message rendering, while it uses only counts.
severity: significant
resolution: 'Split the work: Service.Inbox gates and records waiting refs unexported; a separate package function WaitingMessages renders the messages and only the inbox page route runs it (memoized per principal and round). The summary, MCP and CLI make zero GetMessages and ref resolves (AC 6).'
status: addressed
---
