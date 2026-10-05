---
id: RR-6NZ4YD
type: review-response
title: Address tests lack malformed faces
finding: RunAddressTests covers the zero Ref and an id with @ but not a Face with a path separator or @ or NUL. fsstore maps faces to file stems so an unguarded face must not reach a path.
severity: minor
resolution: 'Amendment A9: add ../x and a@b and NUL faces; each is ErrNotFound on all four backends.'
status: addressed
---

## Finding

RunAddressTests covers the zero Ref and an id with @ but not a Face with a path
separator or @ or NUL. fsstore maps faces to file stems so an unguarded face
must not reach a path.

Design: `.ignored/stage2-design.md` section 11.
