---
id: RR-AMO8Q3
type: review-response
title: store.ErrConflict reused as the retry sentinel
finding: 'Users see "retry: store: already exists", and the MCP/CLI mapping matches every ErrConflict including unique and version conflicts.'
severity: nit
resolution: New sentinel entitymanager.ErrConcurrentChange; only it maps to 409 conflict with a retry hint in dataentry, MCP and CLI; tests check a plain store.ErrConflict gets no retry hint.
status: addressed
---
