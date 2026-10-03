---
id: RR-FE1EGP
type: review-response
title: Error contract still specifies B1 against ruling 2
finding: Section 2 returns non-NotFound load errors as errors (500s) while ruling 2 keeps the uniform miss plus slog.Warn
severity: significant
resolution: 'Design section 8.3 (PR 2): error contract rewritten to ruling 2. Load failures other than ErrNotFound log slog.Warn and return a miss; only gate failures return err. B1 withdrawn; parity test pins it.'
status: addressed
---

**Where:** design section 2, "Error contract", and risk 2.

The design still specifies behaviour change B1: a load error other than
`store.ErrNotFound` is returned as an error, so a route turns an outage into a
500. Ruling 2 (section 7) decided the opposite: keep the uniform miss and log a
`slog.Warn`. The body of the design and the ruling disagree, so an implementer
reading section 2 builds the rejected behaviour.
